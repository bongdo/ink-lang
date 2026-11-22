---
name: Ink
description: Ink language reference, canonical syntax, diagnostics and fixes for .ink code
---
# Ink agent guide

Printed by `ink guide`; loaded on demand as the `ink` OpenCode skill. One canonical way per intent: do not invent syntax.

## Source of truth

- `ink guide` — this text, printed by the binary; it wins on disagreement.
- `go.mod` — project root, module path and Go version.
- In the compiler repository only: `docs/product/SPEC.md` (wins over everything else),
  `docs/product/EDGES.md`, `examples/tour` (valid), `examples/problems` (invalid on
  purpose). A project created by `ink new` has no `docs/` or `examples/`.

## Loop

From the project root (the directory with `go.mod`):

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
  go.mod           module path and Go version
  main.ink         package main; must contain fn main
  lib/             optional local package
```

One directory is one package; no package clause. Local: `use ./lib as lib;`.
Go: `use fmt;`; versioned paths need an alias: `use charm.land/bubbletea/v2 as tea;`.
Generated, never edit: `zz_ink.go`, `zz_ink_test.go`, `.ink/`.

## Canonical syntax

```hare
use fmt;

type User struct {
    Name: str,
    fn Greet() str { return "hi, " + Name; };
};

type Shape enum { Circle { radius: f64 }, Empty, };
type Writer interface { Write(p: u8[]) int!; };
type ID = u64;
const Max = 100;

fn load(name: str) User! { return User { Name: name }; };
fn split(n: i32) (i32, bool)! { return n, true; };
fn pair() i32, bool { return 1, true; };
```

- `type X struct/enum/interface` has no `=`; aliases keep `=`. Methods live in
  the type body; `self` is implicit; struct literals use `field: value`.
- `fn f() R { ... };` — no `=` before the body, trailing `;` after it.
- `var x = value;`; `var p: *T = nil;`; no `:=`.
- `const` is package-level and untyped; a local `const` is an error, use `var`.
- Import paths are bare: `use fmt;`, `use encoding/json;`, `use ./lib as lib;`.
  A versioned path (`/v2`) requires an alias. No quotes.
- `T!` marks a fallible result. `void!` may fail with no value. Multi-result:
  `(A, B)!`.
- Handlers: `or fail;`, `or fail("context");`, `or err { ... };`,
  `or panic("msg");`. `or` always starts a new line, indented once.
- The handler name is always `err`. Inside it, `fail(err, "context");` wraps and
  returns. Manual `if err != nil { return err; }` propagation is rejected.
- A fresh error is `fail(error("msg"));`. `fail("msg")` is only the
  shorthand after `or`; it wraps the call error, it does not create one.
- Multiple results: `fn f() A, B`, `var a, b = f();`, `return a, b;`; no tuples.
- `match` is exhaustive on enums; use `case T as x` and `else` for open types;
  there is no `switch`.
- `&x` takes an address (`&n`, `&s.X`, `&User { ... }`); `*p` reads and writes
  through a pointer. Go APIs that output through a pointer need `&`:
  `json.Unmarshal(data, &u)`, `rows.Scan(&name)`.
- `xs[i]` indexes; `xs[a:b]`, `xs[:b]`, `xs[a:]`, `xs[:]` slice and share
  storage. On a string `s[i]` is a `byte`; use `runes(s)` for code points.
- Assignment and arguments copy the value: structs and arrays copy, while a
  slice shares its backing array until `append` grows past `cap`; a map,
  pointer and interface share. Map iteration order is unspecified, so collect
  and sort keys when order matters.
- `for (var i, x in xs)`, `for (var k, v in m)`, `for (var x in xs)` and
  `for ()`; the index is `int`. `var v, ok = m[k]` reads presence without
  `has`.
- `make[i32](n)` or `make[i32](n, cap)` makes a slice; `error("msg")` makes a
  fresh error.
- `arena { ... };` allocates from a Go arena and frees it at block exit.
  Inside it `new[Point]()` and `make[i32](n)` use the arena; outside they are
  ordinary Go allocations. Arena values must not escape: no assignment to a
  captured variable, no return, no `&T{}` inside. A call argument is a borrow
  for that call; the callee must not keep it. Copy a scalar field out instead.
  `par` is allowed; `new`/`make` must run before `par`, and a worker may open its
  own `arena { ... };`. The compiler sets `GOEXPERIMENT=arenas` automatically.
- `//` comments only; `void` is a result only.

## Concurrency

```hare
par {
    a = loadA()
        or fail;
};
par for (var i = 0; i < len(xs); i += 1) limit 8 { ... };
within 30s { ... };
```

- `par` runs top-level statements as workers; nested blocks are sequential.
- Maps: no range in `par for`, no writes in `par`. Collect keys, write locally.
- Hidden context stops at package and function-value boundaries.
- Checks are static (aliases, escapes, literals); `ink run -race` and
  `ink test -race` are the dynamic oracle.

## Common diagnostics

| code | canonical fix |
|---|---|
| `const-local` | move `const` to package level, or use `var` |
| `decl-equals` | remove `=` before a struct/enum/interface/function body |
| `iface-fn` | remove `fn` from an interface method signature |
| `import-simple` | write `use fmt;` without quotes |
| `import-alias` | add `as name` to a versioned import (`/v2`, `/v3`) |
| `err-manual` | replace manual propagation with `or fail` |
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
