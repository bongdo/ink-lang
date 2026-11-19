# Ink agent guide

Printed by `ink guide`; loaded on demand as the `ink` OpenCode skill. One canonical way per intent: do not invent syntax.

## Source of truth

- `ink guide` — this text, printed by the binary; it wins on disagreement.
- `ink.toml` — project name and Go module path.
- In the compiler repository only: `design/SPEC.md` (wins over everything else),
  `design/GAPS.md`, `examples/tour` (valid), `examples/problems` (invalid on
  purpose). A project created by `ink new` has no `design/` or `examples/`.

## Loop

From the project root (the directory with `ink.toml`):

1. `ink check --json --fast` — parse, resolve, sema; fastest feedback.
2. Fix diagnostics to `[]`; run `ink fmt -w .`.
3. `ink check --json` — full generated-Go typecheck; completion gate.
4. `ink test` (`-race` for concurrency); `ink run [-- args]` (`-race` for races).

No prompts. `ink check --json` exits 1 on errors, 0 when clean.

## JSON contract

`ink check --json` prints one array: `file`, `line`, `col`, `endLine`,
`endCol`, `severity`, `code`, `message`, `hint`. Positions are 1-based;
`severity` is `error` or `warning`. Clean output is `[]`.
`ink fmt -l --json` prints a JSON array of unformatted paths.

## Layout

```text
hello/
  ink.toml         name and Go module path
  main.ink         package main; must contain fn main
  lib/             optional local package
```

One directory is one package; no package clause. Local: `use "./lib" as lib;`.
Go: `use fmt;`; versioned paths need an alias: `use "charm.land/bubbletea/v2" as tea;`.
Generated, never edit: `zz_ink.go`, `zz_ink_test.go`, `.ink/`.

## Canonical syntax

```hare
use fmt;

type User struct {
    Name: str,
    fn Greet() str { return "hi, " + Name; };
};

type Shape enum { Circle { radius: f64 }, Empty, };
type Writer interface { Write(p: u8[]) int or error; };
type ID = u64;
const Max = 100;

fn load(name: str) User or error { return User { Name = name }; };
fn pair() i32, bool { return 1, true; };
```

- `type X struct/enum/interface` has no `=`; aliases keep `=`. Methods live in
  the type body; `self` is implicit; struct literals use `field = value`.
- `fn f() R { ... };` — no `=` before the body, trailing `;` after it.
- `var x = value;`; `var p: *T = nil;`; no `:=`.
- `const` is package-level and untyped; a local `const` is an error, use `var`.
- `use fmt;` for simple Go imports; quoted paths for Go imports with `/` and
  local `./` packages; a versioned path (`/v2`) requires an alias.
- `T or error` plus `or return;`, `or err { ... };`, `or panic("msg");`.
  Manual `if err != nil { return err; }` propagation is rejected.
- Multiple results: `fn f() A, B`, `var a, b = f();`, `return a, b;`; no tuples.
- `match` is exhaustive on enums; use `case T as x` and `else` for open types;
  there is no `switch`.
- `&x` takes an address (`&n`, `&s.X`, `&User { ... }`); `*p` reads and writes
  through a pointer. Go APIs that output through a pointer need `&`:
  `json.Unmarshal(data, &u)`, `rows.Scan(&name)`.
- `xs[i]` indexes; `xs[a:b]`, `xs[:b]`, `xs[a:]`, `xs[:]` slice and share
  storage. On a string `s[i]` is a `byte`; use `runes(s)` for code points.
- `//` comments only; `void` is a result only.

## Concurrency

```hare
par { a = loadA() or return; };
par for (var i = 0; i < len(xs); i += 1) limit 8 { ... };
within 30s { ... };
```

- `par` runs top-level statements as workers; nested blocks are sequential.
- Maps: no range in `par for`, no writes in `par`. Collect keys, write locally.
- G10: hidden context stops at package and function-value boundaries.
- G11: checks are static (aliases, escapes, literals); `ink run -race` and
  `ink test -race` are the dynamic oracle.

## Common diagnostics

| code | canonical fix |
|---|---|
| `const-local` | move `const` to package level, or use `var` |
| `decl-equals` | remove `=` before a struct/enum/interface/function body |
| `iface-fn` | remove `fn` from an interface method signature |
| `import-simple` | write `use fmt;` without quotes |
| `import-alias` | add `as name` to a versioned import (`/v2`, `/v3`) |
| `err-manual` | replace manual propagation with `or return` |
| `nil-untyped` | give `nil` a type: `var p: *T = nil;` or `nil[T]` |
| `par-map-write` | write to a local slice, merge after the `par` |
| `par-map-range` | collect keys into a slice, then `par for` the slice |
| `par-capture-write` | copy the value locally and collect the result |
| `par-break` | return a value or set a local flag |
| `par-loop` | replace `for ()` with a bounded loop |
| `comment-block` | use `//`; block comments are rejected |
| `void-value` | return or store a value; `void` is a result only |
| `missing-return` | end every path with `return;` |
| `unused-import` | remove the import or use it |
| `unused-var` | use the variable or replace it with `_` |
| `syntax` / `go-type` | read `message` and `hint` |

## Editor

`ink lsp` speaks LSP on stdio: diagnostics, hover, definition, completion,
document/workspace symbols, signature help, formatting, references, rename and
quick fixes. `ink check` stays the source of truth.

## Examples

`examples/tour` builds and runs; start there. `examples/problems` is invalid on purpose: uncomment one block to see its diagnostic.
