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
- Contract blocks live in a struct body. `wire { Field: "name"; };` is the
  default JSON view; any other lowercase block name is a named view:
  `admin { Field: "name"; };`. A block is an allowlist; unlisted fields are
  omitted. Named views are package-local and must be used (`u.admin()`) or the
  compiler reports `contract`. Named views return `map[str, any]`; only the
  default `wire` view decodes. `check`, `store`, `doc` and `policy` are ordinary
  view names in this version, not validators or database schemas.
- `fn f() R { ... };` — no `=` before the body, trailing `;` after it.
- `var x = value;`; `var p: *T = nil;`; no `:=`.
- `const` is package-level and untyped; a local `const` is an error, use `var`.
- Import paths are bare: `use fmt;`, `use encoding/json;`, `use ./lib as lib;`.
  A versioned path (`/v2`) requires an alias. No quotes.
- `T!` marks a fallible result. `void!` may fail with no value. Multi-result:
  `(A, B)!`.
- Handlers: `or fail;`, `or fail("context");`, `expr on err { ... };`.
  `or` always starts a new line, indented once.
- `on err { ... };` as the first statement of a function or block is a scope
  listener for unhandled `go` errors. It must be non-empty and cannot use
  `return`; `fail` sends the error to the enclosing function.
- The handler error is always `err`. Inside it, `fail(err, "context");` wraps
  and returns. Manual `if err != nil { return err; }` propagation is rejected.
- A fresh error is `errors.New("msg")` (`use errors;`); `fail(errors.New("msg"));`
  inside a handler constructs it and returns. `fail("msg")` is only the
  shorthand after `or`; it wraps the call error, it does not create one.
- A fallible call as a bare statement (`f();`) is a compile error. Use
  `f() on err { ... };` or `f() or fail;`; `_ = f();` discards deliberately.
  The rule also applies inside `go` bodies.
- `panic(...)` is Go's panic: not caught by `on err`, not recovered by `go`.
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
- `make[i32](n)` or `make[i32](n, cap)` makes a slice; `errors.New("msg")`
  makes a fresh error and `error` is only a type.
- `arena { ... };` takes memory from a reusable scratch buffer and returns it at
  block exit. Inside it `new[Point]()` and `make[i32](n)` use the buffer;
  outside they are ordinary Go allocations. The buffer is reused by the next
  execution, so a hot loop stops allocating after its high-water mark; the
  buffer is released at block exit outside a loop, and after the enclosing loop
  when it is inside one. Every execution starts zeroed. Arena memory
  never leaves the block: no assignment to a captured variable, no return, no
  `&T{}`, no call argument or receiver. Read fields and indices, copy structs
  and arrays that hold no references, and `copy` bytes out. `go` is allowed;
  `new`/`make` must run before `go`, a worker may write `rows[y]` and open its
  own `arena { ... };`.
- `//` comments only; `void` is a result only.

## Concurrency

```hare
go {
    a = loadA()
        or fail;
};
go for (var i = 0; i < len(xs); i += 1) limit 8 { ... };
go save(1, "hello");            // detached, program-owned
```

- `go { }` runs top-level statements as workers and joins at the block.
  Nested blocks are sequential inside one worker.
- `go for` runs iterations as workers; `limit N` is optional (default
  `GOMAXPROCS`); the body is sequential inside one worker.
- `go f(args);` starts a detached task; `f` returns `void` or `void!`.
  Arguments and the receiver are evaluated first and there is no closure; the
  task is owned by the program and may outlive the caller. A method call on a
  value takes its address, so the task shares that variable.
- A fallible task needs its own handler: `go f(a) on err { log(err); };`.
  `void!` without one is a compile error. The handler runs in the task, sees
  only `err` and package names, and cannot `fail` or `return`; the scope
  listener never receives a detached task error.
- `on err { ... };` first in a function or block listens for unhandled errors
  of `go` constructs. A scope listener cannot be empty or use `return`; `fail`
  escalates the error to the enclosing function. An inline
  `expr on err { ... };` handles one expression, may fall through, and leaves
  the value at its zero value; it may `fail`, `return` or `panic`.
- Maps: no range in `go for`, no writes in `go`. Collect keys, write locally.
- No hidden context: `ctx` is an ordinary value, passed explicitly to the Go
  APIs that need it. `go` never creates or cancels a context.
- Panics are Go panics: `on err` does not catch them and `go` does not recover
  them; the process exits with status 2.
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
| `error-ctor` | write `errors.New("msg")`; `error` is a type |
| `nil-untyped` | give `nil` a type: `var p: *T = nil;` or `nil[T]` |
| `contract` | keep the block inside a struct; list existing exported fields; use each named view |
| `par-removed` | write `go { ... };` |
| `within-removed` | use `context.WithTimeout` and pass `ctx` explicitly |
| `or-panic` | use `on err { panic(...) }` or `or fail` |
| `go-map-write` | write to a local slice, merge after the `go` |
| `go-map-range` | collect keys into a slice, then `go for` the slice |
| `go-capture-write` | copy the value locally and collect the result |
| `go-break` | return a value or set a local flag |
| `go-loop` | replace `for ()` with a bounded loop |
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
