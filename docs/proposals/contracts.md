# Ink — contract blocks in structs

Status: implemented in this change. Source of truth for the design.

## Goal

One type declaration can carry named external views without Go struct tags in
the source. `wire {}` is the default view; any other bare block name inside the
type is a named view of the same contract type. There are no separate
`validate`/`db`/`doc` buyers in this change: every block is a view of the same
kind, per the owner's decision ("это один и тот же тип — wire просто с разными
именами").

## Surface

```ink
struct User {
    ID: i64,
    Email: str,
    Role: str,
    Pass: str,

    fn Label() str {
        return Email;
    };

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
        Email: "email",
        Role: "role",
    };
};
```

Rules:

- Blocks are only allowed inside a `type ... struct` body.
- `wire` is the default view and may appear once.
- Any other identifier followed by `{` is a named view. Names are unique.
- A block is an allowlist: only listed fields are in the view. Unlisted
  fields are not serialized.
- Each entry is `Field: "external-name"`. External names are arbitrary
  strings. Entries are separated by `,` or `;`; a trailing separator is
  allowed. The block must be non-empty.
- A field appears at most once in a block; external names are unique per block.
- Block names are not keywords; `wire` is contextual in a type body. A field
  named `wire` is written `wire: T,` and is unambiguous.
- Named view names start with a lowercase letter and are package-local. The
  generated method keeps that exact name, so another package cannot call it.
  An unused named view is an error; `wire` is exempt. (The review found that an
  exported view cannot be checked for use soundly, so views are unexported.)
- Fields listed in any view must be exported (an uppercase ASCII letter); Go's
  `encoding/json` ignores unexported fields, and a named view would silently
  disagree with the default view.
- Contracts on generic structs are rejected in this change.
- `wire` is the only block with special behavior. `check`, `store`, `doc` and
  `policy` are ordinary view names in this version; they are not validators or
  database schemas.
- Contracts on enums, interfaces and aliases are rejected by the parser
  because only the struct body accepts them.

## Codegen

Default `wire`:

- Emit `json:"<external>"` on listed fields.
- Emit `json:"-"` on unlisted exported fields.
- If there is no `wire` block, emit no tags and keep Go's default behavior.
- Reject external names that `encoding/json`'s `isValidTag` rejects, plus the
  empty string and exactly `-`. Allowed: Unicode letters and digits, and the
  punctuation `!#$%&()*+-./:;<=>?@[]^_{|}~ `. This excludes quote, backslash,
  backtick, comma and control characters. The check runs in sema, before codegen;
  a bad tag silently falls back to the Go field name.
- Unlisted unexported fields are already ignored by `encoding/json`; emitting a
  tag on them is harmless.

Named views:

- Generate one method on the type:
  `func (u *User) admin() map[string]any`
- The method keeps the view name exactly; view names must start lowercase, so
  the method is package-local. This makes the unused-view error sound.
- The method returns a map with external names and field values:
  `map[string]any{"id": u.ID, "email": u.Email, "role": u.Role}`.
- Usage: `json.Marshal(u.admin())`.
- A generated method must not collide with an existing field or method; that is
  a compile error.
- Named views are encode-only in this change. Decoding a named view is not
  supported.
- Map keys are emitted in alphabetical order by `encoding/json`; object order
  is not significant, but tests must compare maps or sorted JSON.

## Diagnostics

All semantic contract errors use one diagnostic code `contract` and a message
starting with `contract:`. Parser errors stay `syntax`. Cases:

- empty block;
- field does not exist in the type;
- field is unexported;
- field repeated in one block;
- external name repeated in one block;
- two blocks with the same name;
- `wire` block on a generic struct (named views are also rejected there in
  this change);
- generated view method name collides with a field or method;
- named view never referenced by a selector call;
- external name is empty, `-`, or rejected by `isValidTag`;
- named view does not start lowercase.

## Tests

- Parser: default and named blocks, commas/semicolons, trailing separator,
  string literals, block after methods, field named `wire`.
- Sema: missing field, duplicate field/name/block, empty block, generic struct,
  method collision, unused view.
- Gen: `json:"-"` on unlisted fields, external names on listed fields, named
  view map method, JSON output of default and named views, nested struct with
  its own default view.
- Formatter: round-trip of blocks.
- LSP: no new keyword; completion inside a block should offer field names
  (optional for the first pass).
- Existing behavior: a struct without contracts generates no tags and no
  methods.

## Open questions for review

1. Named views are package-local (lowercase method). This is a v1 limitation:
   an exported type's admin view cannot be used from another package. A future
   exported view API can be added without changing the block syntax.
2. Named view method signature: `map[string]any` (no DTO, no field-order
   preservation). Chosen for this change.
3. Contracts on generic structs are rejected in this change. Go permits generic
   receiver methods; this is a scope cut, not a language limitation.
4. Nested named views: a named view value that is itself a struct with views
   marshals through its default `wire` tags, not another named view.
5. Decoding: only the default view decodes (through `encoding/json` tags).
   Named views are encode-only.
6. Formatter ordering: fields, methods, contracts. Source order is not
   preserved across categories; this matches the existing type-body printer.
7. `check --fast` does not resolve generated view methods; a typo in a view
   call is reported by the full typecheck, and an unused view by sema.
