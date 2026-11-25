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
   The program starts in the project root (the directory with `go.mod`);
   `ink -C dir <command>` selects the project from anywhere.

No prompts. `ink check --json` exits 1 on errors, 0 when clean.

`ink run` builds `.ink/bin/<name>`, runs it as a child, forwards SIGINT/SIGTERM
and exits with the child's code. `ink test` passes its flags to `go test`.
`ink <cmd> --help` prints the usage.

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

## Dependencies

`ink check` tidies `go.mod` for you: importing a module adds its requirement.
Pin a version with `go get pkg@v1.2.3`. `GOFLAGS`, `CGO_ENABLED` and `-tags`
pass through to the Go toolchain, so native libraries follow their own build
rules; a cgo package without system headers needs the package's own tag, for
example `GOFLAGS=-tags=nocgo ink check` for raylib-go on a box without X11.

## Tests

```hare
test "counter adds" {
    var c = Counter { n: 1 };
    c.Add(2);
    assert_eq(c.Value(), 3);
};

test "load fails on bad json" {
    var s = Load("bad.json")?;
    assert_eq(len(s.Items), 0);
};
```

- `test "name" { ... };` at package level in any `.ink` file. The file name has
  no special meaning, and a `Test*` function is an ordinary function, not a
  test. Blocks are generated into `zz_ink_test.go` and run by `ink test`.
- A test lives in the package of its own directory: a test in `store/` uses
  `store`'s names directly and must not `use ./store;`.
- `assert(cond)` and `assert_eq(got, want)` fail with `file:line`. A failed
  assertion or panic fails only that test; the remaining tests still run.
- `check(cond, "msg")` creates a fresh error when `cond` is false; `?`
  propagates an error and `on err { ... }` handles it.
- `ink test [go test flags]`: `-run`, `-v`, `-count=1`. When the package uses
  `par` or `spawn`, `ink test` adds `-race` itself; `--no-race` opts out. The
  race detector needs cgo; without it tests run without `-race` and say so.
- With no `test` block, `ink test` prints `[no test files]` plus a hint on how
  to declare one; that is not a passing suite.

## Canonical syntax

```hare
use fmt;

struct User {
    Name: str,
    fn Greet() str { return "hi, " + Name; };
};

enum Shape { Circle { radius: f64 }, Empty, };
interface Writer { Write(p: u8[]) int!; };
alias ID = u64;
const Max = 100;

fn load(name: str) User! { return User { Name: name }; };
fn split(n: i32) (i32, bool)! { return n, true; };
fn pair() i32, bool { return 1, true; };
```

- Declarations start with `struct`, `enum`, `interface` or `alias`; aliases keep
  `=`. Methods live in the type body; `self` is implicit; struct literals use
  `field: value`.
- Contract blocks live in a struct body. `wire { Field: "name"; };` is the
  default JSON view; any other lowercase block name is a named view:
  `admin { Field: "name"; };`. A block is an allowlist; unlisted fields are
  omitted. Named views are package-local and must be used (`u.admin()`) or the
  compiler reports `contract`. Named views return `map[str, any]`; only the
  default `wire` view decodes. `check`, `store`, `doc` and `policy` are ordinary
  view names in this version, not validators or database schemas.
- `fn f() R { ... };` — no `=` before the body, trailing `;` after it.
- A method is declared inside the type body, after its fields; there is no
  file-scope receiver clause, and methods are never split across files. A big
  type is split by composition: `struct SessionAPI { App: *App };` with
  its methods in the body, and `App` holds `Sessions: *SessionAPI`. `self` is a pointer: mutating `Items`, `Orders`
  or any field inside a method changes the value the caller holds, and a map or
  slice field is shared even when the struct is copied. Return `void!` when the
  method can fail; a plain `void` cannot use `?`. Inside a method a
  sibling is called on `self`: `self.Add(1)`, not `Add(1)`. A call on a value
  takes its address, so `c.Add(1)` mutates `c`; passing `c` to a Go function
  still needs `&c`.
- `var x = value;`; `var p: *T = nil;`; no `:=`. Every `var` has a value —
  there is no uninitialized declaration, so the zero value is written out:
  `var n = 0;`, `var u = User {};`, `var m = map[str, i32](nil);`.
- `const` is package-level and untyped; a local `const` is an error, use `var`.
  `ink fmt` keeps consecutive `const` declarations and consecutive `alias`
  declarations together, with one blank line around each group.
- Import paths are bare: `use fmt;`, `use encoding/json;`, `use ./lib as lib;`.
  A versioned path (`/v2`) requires an alias. No quotes. The local name is the
  last path element unless aliased, so a Go package whose name differs from its
  directory needs one: `use github.com/gen2brain/raylib-go/raylib as rl;`.
  `use pkg as _;` is a blank import for a driver or an `init` side effect.
- `T!` marks a fallible result. `void!` may fail with no value. Multi-result:
  `(A, B)!`.
- `call()?` propagates the error to the caller. `?` may sit anywhere in an
  expression; it needs a fallible function (`!`) and is a compile error in a
  `void` function.
- `expr on err { ... };` handles that call here. The handler error is always
  `err`; the expression keeps its zero value when the call fails.
- `on err { ... };` as the first statement of a function or block is a scope
  listener for unhandled `par` errors. It must be non-empty and cannot use
  `return`.
- A fallible call must be handled: `call()?`, `call() on err { ... };` or
  `_ = call();`. A bare `f();` is a compile error; the rule also applies inside
  `par` bodies.
- `check(cond, "msg");` returns a fresh error when `cond` is false. It is the
  only way to create an error in app code and needs a fallible function or a
  `par` listener. `print(...)` writes a line to stdout like `fmt.Println`, has
  no error, and needs no `_ =`.
- Manual `if err != nil { return err; }` propagation is rejected; use `?`.
  `errors.New`, `errors.Is` and `errors.As` remain Go interop; app code
  classifies errors in an adapter.
- `panic(...)` is Go's panic: not caught by `on err`, not recovered by `par`.
- Multiple results: `fn f() A, B`, `var a, b = f();`, `return a, b;`; no tuples.
- `match` is exhaustive on enums; use `case T as x` and `else` for open types;
  there is no `switch`.
- `&x` takes an address (`&n`, `&s.X`, `&User { ... }`); `*p` reads and writes
  through a pointer. Go APIs that output through a pointer need `&`:
  `json.Unmarshal(data, &u)`, `rows.Scan(&name)`.
- `xs[i]` indexes; `xs[a:b]`, `xs[:b]`, `xs[a:]`, `xs[:]` slice and share
  storage. On a string `s[i]` is a `byte`; use `runes(s)` for code points.
- A package used only for its side effects is a blank import:
  `use modernc.org/sqlite as _;` before `sql.Open`, otherwise the binary fails
  at run time with `unknown driver`. SQLite is `sql.Open("sqlite", path)`;
  transactions are `var tx = db.Begin()?;`, then `tx.Commit()?;`
  and `defer tx.Rollback();` for the error path. `errors.Is(err, sql.ErrNoRows)`
  detects an empty result.
- `defer f();` accepts a fallible call and drops its error; use it for cleanup
  only, never for work whose error matters.
- Backtick strings are raw: no escapes, newlines kept.
- `_ = x` discards a value you do not need. A `void` call is already a
  statement (`f();`), so `_ = f()` is an error.
- JSON key order follows the struct field order, not the order inside the
  `wire` block.
- `q.Get("k")` returns `""` when the key is absent and when it is present but
  empty (`?k=`); `var v, ok = q["k"]` gives `[""]` and `true` for `?k=`. Absent
  and empty are different — decide which one your route accepts.
- A Go named type whose underlying type is a map, slice or array is used like
  that type and keeps its methods: `r.URL.Query().Get("done")` is a `str`,
  `q["done"]` is a `str[]`.
- Assignment and arguments copy the value: structs and arrays copy, while a
  slice shares its backing array until `append` grows past `cap`; a map,
  pointer and interface share. Map iteration order is unspecified, so collect
  and sort keys when order matters.
- `for (var i, x in xs)`, `for (var k, v in m)`, `for (var x in xs)` and
  `for ()`; the index is `int`. `var v, ok = m[k]` reads presence without
  `has`.
- A slice type is its constructor: `i32[](n)` is n zeros, `i32[](cap: n)` is
  empty with capacity, `i32[](n, cap: m)` sizes both. `size:` and `cap:` are
  the only labels; `i32[](size: n)` equals `i32[](n)`. A map type is its
  constructor too: `map[K, V] {}` is empty, `map[K, V](nil)` is nil, and
  nothing allocates implicitly. `errors.New("msg")` makes a fresh error and
  `error` is only a type.
- Go functions keep their Go signatures: `fmt.Println` returns `(int, error)`,
  so a bare call is an error; write `print(...)` for a line of output,
  `_ = fmt.Println(...)` to discard the error deliberately, or `...?` to
  propagate it.
- `asset("web/index.html")` returns `(u8[], bool)`: file bytes and found.
  Files come from `ink build -embed web`; without the flag it reports false.
- `arena { ... };` takes memory from a reusable scratch buffer and returns it at
  block exit. Inside it `new[Point]()` and `i32[](n)` use the buffer;
  outside they are ordinary Go allocations. The buffer is reused by the next
  execution, so a hot loop stops allocating after its high-water mark; the
  buffer is released at block exit outside a loop, and after the enclosing loop
  when it is inside one. Every execution starts zeroed. Arena memory
  never leaves the block: no assignment to a captured variable, no return, no
  `&T{}`, no call argument or receiver. Read fields and indices, copy structs
  and arrays that hold no references, and `copy` bytes out. `par` is allowed;
  `new`/`i32[](n)` must run before `par`, a worker may write `rows[y]` and open
  its own `arena { ... };`.
- `//` comments only; `void` is a result only.

## Statements and values

```hare
if (x > 0) { ... } else if (x == 0) { ... } else { ... };
if (i < k && i < len(xs)) { ... };   // conditions use && || !
for (var i = 0; i < n; i += 1) { ... };
for (; cond; ) { ... };          // no while and no for (cond)
for (var i, x in xs) { ... };
{
    defer closeIt();
};
```

- Every block statement ends with `};` and keeps parentheses around its
  condition; a missing `;` is reported at the next token.
- `defer f();` runs at block exit, LIFO, including through `return`, `break`,
  `continue`, `?` and panic. It takes only a call, and the call's result
  is dropped, so keep deferred cleanup non-fallible or handle the error
  explicitly before returning.
- A function literal may be an expression `fn(a: i32) i32 => a + 1;` or a
  block; it captures locals. A block literal with results must end every path
  in `return;`, same as a function. A method may share its type's name; method
  names live in the type's method set, not in the package scope. `func` is Go,
  not Ink.
- Types: slice `T[]`, array `T[N]`, map `map[K, V]`, pointer `*T`. Literals:
  `i32[] { 1, 2 }`, `T[N] {}`, `map[str, i32] { "a": 1 }`,
  `User { Name: "a" }`. There is no `[1, 2]` form.
- `int` is Go's `int`: `len`, indexes and loop counters are `int`. `i32`,
  `i64`, `u64`, `f64` are distinct types, so convert at the boundary:
  `i32(n)`, `f64(dt)`. An untyped `const` defaults to `int`.
- A conversion is the type as a function: `i32(x)`, `f64(n)`, `str(b)` for
  bytes, `bytes(s)`, `runes(s)`. `str(65)` follows Go's `string(rune)` and is
  `"A"`, not `"65"`; format numbers with `fmt.Sprint(n)`.
- Identifiers are ASCII: `a-z`, `A-Z`, `0-9`, `_`. An identifier does not
  start with a digit. Builtin type and function names (`i32`, `len`, `min`, …)
  are reserved and cannot be shadowed anywhere.
- Reserved words cannot be identifiers: `use`, `fail`, `as`, `fn`, `type`,
  `struct`, `enum`, `interface`, `var`, `const`, `return`, `if`, `else`,
  `for`, `in`, `match`, `case`, `par`, `spawn`, `on`, `arena`, `defer`, `break`,
  `continue`, `or`, `test`, `true`, `false`, `nil`, `map`. `go` and `within`
  are removed keywords with their own diagnostics.

## Concurrency

```hare
par {
    a = loadA()?;
};
par for (var i = 0; i < len(xs); i += 1) limit 8 { ... };
spawn save(1, "hello");         // detached, program-owned
```

- `par { }` runs top-level statements as workers and joins at the block.
  Nested blocks are sequential inside one worker.
- `par for` runs iterations as workers; `limit N` is optional (default
  `GOMAXPROCS`); the body is sequential inside one worker.
- `spawn f(args);` starts a detached task; `f` returns `void` or `void!`.
  Arguments and the receiver are evaluated first and there is no closure; the
  task is owned by the program and may outlive the caller. A method call on a
  value takes its address, so the task shares that variable.
- A fallible task needs its own handler: `spawn f(a) on err { print(err); };`.
  `void!` without one is a compile error. The handler runs in the task, sees
  only `err` and package names, and cannot `?` or `return`; the scope listener
  never receives a detached task error.
- `on err { ... };` first in a function or block listens for unhandled errors
  of `par` constructs. A scope listener cannot be empty or use `return`; it
  handles the error and the scope continues. An inline
  `expr on err { ... };` handles one expression, may fall through, and leaves
  the value at its zero value; it may `return` or `panic`.
- Maps: no range in `par for`, no writes in `par`. Collect keys, write locally.
- No hidden context: `ctx` is an ordinary value, passed explicitly to the Go
  APIs that need it. `par` never creates or cancels a context.
- Panics are Go panics: `on err` does not catch them and `par` does not recover
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
| `err-manual` | replace manual propagation with `?` |
| `error-ctor` | write `check(false, "msg")`; `error` is a type |
| `nil-untyped` | give `nil` a type: `var p: *T = nil;` or `nil[T]` |
| `contract` | keep the block inside a struct; list existing exported fields; use each named view |
| `go-removed` | concurrency is `par` (waits) or `spawn` (does not) |
| `within-removed` | use `context.WithTimeout` and pass `ctx` explicitly |
| `or-panic` | use `on err { panic(...) }` |
| `go-map-write` | write to a local slice, merge after the `par` |
| `go-map-range` | collect keys into a slice, then `par for` the slice |
| `go-capture-write` | copy the value locally and collect the result |
| `go-break` | return a value or set a local flag |
| `go-loop` | replace `for ()` with a bounded loop |
| `comment-block` | use `//`; block comments are rejected |
| `void-value` | return or store a value; `void` is a result only |
| `or-removed` | replace `or fail` with `?` after the call |
| `fail-removed` | replace `fail(...)` with `check(false, "msg")` |
| `missing-return` | end every path with `return;` |
| `init-cycle` | break the cycle: initialize in `main` or in a function |
| `map-addressable` | copy out, mutate, write back: `var p = m[k]; p.F = v; m[k] = p;` |
| `conformance-unchecked` | warning: the method set is not nameable; `go build` is the backstop |
| `str-number` | `str(n)` is a rune; format numbers with `strconv.Itoa`, `strconv.FormatFloat` or `fmt.Sprint` |
| `go-vet` | `go vet` finding in generated code; fix the Ink line it points at |
| `nil-map-write` | warning: the map is nil; initialize it with `map[K,V] {}` |
| `unused-import` | remove the import or use it |
| `unused-var` | use the variable or replace it with `_` |
| `syntax` / `go-type` | read `message` and `hint` |

## Editor and agents

`ink mcp` is an MCP server on stdio for harnesses: `diagnose`, `hover`,
`definition`, `references`, `complete`, `symbols`, `rename`, `quickfix`.
`ink new` writes it into `opencode.json`, so an agent calls the compiler
directly. `ink new` also installs an OpenCode plugin under
`.opencode/plugins/ink/`: after every `.ink` edit it runs `ink check` and
injects the diagnostics into the session, so an agent sees errors without
calling a tool.

`ink lsp` speaks LSP on stdio: diagnostics, hover, definition, completion,
document/workspace symbols, signature help, formatting, references, rename and
quick fixes. `ink check` stays the source of truth.

## Deliberately absent

No `chan`, `select`, `async`/`await`, `switch`, `goto`, labels, `:=`, `while`,
user-declared variadics, local named functions (use a function literal), or
`unsafe` in app code. Keep these in a Go adapter; pass `ctx` explicitly.

## Examples

`examples/tour` builds and runs; start there. `examples/problems` is invalid on purpose: uncomment one block to see its diagnostic.
