# Ink — language specification (0.2 preview)

This document freezes every question left open in
[`archive/SYNTAX.md`](archive/SYNTAX.md). Where this preview and the older v0
notes disagree, this document wins. This is not a 1.0 promise: feature status is
in [`FEATURES.md`](FEATURES.md).

## 1. Source files and packages

- Source files end in `.ink`, UTF-8.
- A package is one directory. Every `.ink` file in it belongs to the same
  package. There is no package clause; the directory name is the package name.
- The project root is the directory holding `ink.conf`:

```text
name = "hello"
module = "example.com/hello"
```

  `ink.conf` is a flat `key = "value"` file. `ink.toml` is still read when
  present, so old projects keep working.

- `module` is the Go module path used for generated code. If absent,
  `example.com/<name>` is used.
- The root package must be `package main` and contain `fn main`.
- Import paths are bare, never quoted: `use fmt;`, `use encoding/json;`,
  `use ./internal/ed as ed;`. The path is read until whitespace or `;`.
- A non-local Go import path whose last segment is a version (`/v2`, `/v3`,
  ...) requires an alias: `use charm.land/bubbletea/v2 as tea;`. The default
  alias would be `v2`, which names the version, not the package.
- `as _` is a blank import: the package is linked for its side effects and
  binds no name, so it is never reported as unused. It is allowed only for Go
  import paths; a local `use ./lib as _;` is an error.
- Generated Go lives next to the sources as `zz_ink.go` and
  `zz_ink_test.go`. `.ink/` holds generated support files.

## 2. Lexical rules

- `;` ends every statement and every block used as a statement.
- Comments: `// ...` only; block comments are rejected with a diagnostic.
- Identifiers: Unicode letters, digits, `_`; the first character is a letter or
  `_`. Names beginning with `_s_` are reserved for codegen.
- Visibility is by the first letter, exactly as in Go: `User` is public,
  `user` is private.
- `fail` is a statement keyword and cannot be an identifier.
- Go keywords (`func`, `range`, `package`, `import`, `go`, `chan`, `select`,
  `switch`, `goto`, `fallthrough`) are rejected as Ink identifiers.
- String escapes follow Go. Raw strings use backticks.
- Durations: `10ns`, `500us`, `2ms`, `30s`, `5m`, `1h`. A duration is a value
  of type `Duration` and is accepted where `Duration` is expected.

## 3. Types

Type declarations have one canonical form:

```hare
type User struct { ... };
type Shape enum { ... };
type Writer interface { ... };
type UserId = u64;
```

A struct, enum or interface declaration has no `=` before its body; an alias
keeps `=`. The older form with `=` before a struct, enum or interface body is
rejected.

### 3.1. Builtins

`void`, `bool`, `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `int`,
`uint`, `f32`, `f64`, `str`, `rune` (alias `i32`), `byte` (alias `u8`),
`usize` (alias `uint`), `isize` (alias `int`), `any`, `error`, `Duration`
(alias `i64`), `Type` (runtime type value, for reflection interop).

`void` is allowed only as a function result. It is not a value type: variables,
fields, parameters, arguments, returns with values and composite elements
cannot have type `void`.

### 3.2. Constructors

| Form | Meaning |
|---|---|
| `T[]` | slice of `T` |
| `T[N]` | array of `N` elements; `N` is a constant expression |
| `T[U]` | generic instantiation; `U` is a type |
| `map[K, V]` | map |
| `*T` | pointer |
| `fn(A, B) R` | function value |
| `fn(A, B) (R, S)` | function value with several results |
| `fn(A, B) R!` | fallible function value |
| `pkg.Type` | imported type |
| `(T)` | grouping |

`T[]` binds tighter than `*`: `*T[]` is a pointer to a slice. `(*T)[]` is a
slice of pointers.

### 3.3. Structs

```hare
type Buffer struct {
    data: u8[],
    name: str,

    fn Write(p: u8[]) int! {
        data = append(data, p...);
        return len(p);
    };
};
```

- Fields and methods live in the type body. Commas separate fields.
- `self` is the implicit pointer receiver and is never declared.
- Field access is bare inside methods unless a parameter or local shadows it;
  then `self.field` is required.
- Methods always use a pointer receiver.
- A method may not have the same name as a field of its type.

### 3.4. Enums

```hare
type Shape enum {
    Circle { radius: f64 },
    Rect { w: f64, h: f64 },
};
```

- A variant without fields is written `Empty,`.
- Variants are constructed as `Shape.Circle { radius: 2.0 }`.
- Variant names must be unique within a package so that bare `case` patterns
  are unambiguous; a qualified pattern `case pkg.Circle { ... }` is allowed.
- An enum is a sealed sum type. Its zero value is not constructible.
- `match` on an enum must be exhaustive.

### 3.5. Interfaces

Interfaces exist for Go interop and for explicitly declared contracts:

```hare
type Writer interface {
    Write(p: u8[]) int!;
};
```

Implementation is structural. No `implements`, no embedding.

### 3.6. Generics

```hare
type Stack[Item] struct {
    data: Item[],

    fn Push(x: Item) void {
        data = append(data, x);
    };
};

fn Map[In, Out](xs: In[], f: fn(In) Out) Out[] { ... };
```

- Type parameters are written in `[...]` after the function or type name.
- Constraints do not exist: every parameter behaves like Go `any`.
  Operations that need a constraint (`<`, `+`, indexing) are rejected by Go.
  This is a deliberate limit, not a preview gap.
- Type arguments are inferred when possible; explicit arguments are written
  `Map[i32, str](xs, f)`.
- Generic methods cannot introduce their own type parameters.

### 3.7. Aliases

```hare
type UserId = u64;
type Handler = fn(Request) Response!;
```

An alias does not create a new Go type.

### 3.8. Arenas

```hare
arena a {
    var p = a.New[Point]();
    p.X = 1;
    var xs = a.MakeSlice[i32](8);
    xs[0] = 2;
};
```

The block allocates from one Go arena and frees it at block exit, including
`return`, `break`, `continue` and panic. `a.New[T]()` returns `*T`;
`a.MakeSlice[T](n)` or `(n, cap)` returns `[]T`.

Values allocated in an arena must not escape the block: after it they are
use-after-free. `par` inside an arena is rejected. The feature uses Go's
experimental `arena` package; `ink` sets `GOEXPERIMENT=arenas` for build, run,
test and typecheck when a source uses an arena.

## 4. Variables, constants, literals

```hare
var x = 1;
var xs = u8[] {1, 2, 3};
var a = u8[3] {1, 2, 3};
var u = User { Name: "Alice", age: 30 };
var m = map[str, User] {};
var p: *Point = nil;
var e: error = nil;
var i, j: i32 = f();
const Max = 100;
const Greeting = "hello";
```

- `var` always has an initializer. The type may be written before `=`; without
  it, the type comes from the initializer.
- A typed `var` applies one type to all names: `var a, b: T = ...`.
- `const` is package-level only. A local `const` is an error; use `var`.
  `const` is a compile-time name, not immutable runtime storage: it can size
  arrays and appear in `case` patterns. It cannot have a type.
- Composite literals always carry a type.
- Struct fields are named `field: value`; omitted fields get zero values.
- Map literals use `key: value`.
- Bare `nil` takes its type from the declared or expected type:
  `var p: *Point = nil;`, `var e: error = nil;`, `return nil;`, `f(nil)`.
- `nil[T]` is an explicit escape hatch for contexts where the type is `any` or
  an interface, not the default spelling. `nil` is valid only for pointers,
  slices, maps, functions and interfaces.
- `_` is the only blank/discard construct. It is a result placeholder in
  multi-result bindings (`var a, _ = f();`) and an explicit discard assignment
  (`_ = a;`). There is no `discard` builtin.
- `has(m, key)` reports key presence.
- `*p` reads a pointer and `*p = v` writes through it.
- `&x` takes an address: a variable (`&n`), a field (`&s.X`), an array or
  slice element, or a composite literal (`&User { ... }`). This is the form
  required by Go APIs that write through a pointer: `json.Unmarshal(data, &u)`,
  `rows.Scan(&name)`, `flag.IntVar(&n, ...)`.
- `xs[i]` indexes a slice, array or map; on a string it is the byte at `i`.
  `xs[a:b]`, `xs[:b]`, `xs[a:]` and `xs[:]` slice a slice, array or string and
  share the underlying storage; bounds follow Go. Use `runes(s)` for code
  points.

### 4.1. Builtins

| Builtin | Meaning |
|---|---|
| `len(x)` | length of a slice, array, map or string |
| `cap(x)` | capacity |
| `append(s, x, ...)` | grow a slice; `append(s, xs...)` spreads |
| `copy(dst, src)` | copy elements |
| `delete(m, k)` | remove a map key |
| `has(m, k)` | key presence |
| `min(a, b, ...)`, `max(...)` | numeric minimum/maximum |
| `runes(s)` | `str` to `rune[]` |
| `bytes(s)` | `str` to `u8[]` |
| `str(runes_or_bytes)` | conversion to `str` |
| `make[T](n)`, `make[T](n, cap)` | `[]T` with length and capacity |
| `error(msg)` | a fresh `error` with that message |
| `panic(msg)` | abort |
| `assert(cond)`, `assert_eq(got, want)` | checks for tests and code |

String indexing follows Go: `s[i]` is the byte at `i`. Use `runes(s)` when
code points are needed.

Conversions use the type as a function: `i32(x)`, `u64(x)`, `f64(x)`,
`str(runes)`, `bytes(text)`.

Multiple names are allowed for multi-result calls:

```hare
var value, ok = m["key"];
var n, err = f();
var n, _ = f();
_ = n;
```

## 5. Functions

```hare
fn area(r: f64) f64 {
    return 3.14159 * r * r;
};

fn pair() i32, bool {
    return 1, true;
};

fn load() (User, Order)! { ... };

fn main() void! { ... };
```

- A function declaration has no `=` before its body. The older form with `=`
  before the body is rejected. `ink fmt` adds explicit `void` when the
  result is omitted.
- Result type is mandatory, except that `void` may be omitted in the current
  preview. `void` is written explicitly when present. `void` is allowed only as
  a function result; it cannot be used as a value type.
- Several results are a comma-separated list. This is not a tuple value:
  no tuple type, indexing or argument nesting. Allowed positions are `return`,
  `var a, b = f();` and `a, b = f();`.
- In a function type, several results are grouped: `fn(i32) (str, bool)`.
- The fallible form is `T!`, `(A, B)!` or `void!`.
  `or` propagates the error and yields every non-error result:
  `var a, b = f() or fail;`.
- `main` may be `void` or `void!`. An error prints to stderr and exits 1.
- Function literals are closures and the only local callable form; local named
  functions are deliberately absent.

```hare
fn(i: int, j: int) bool => i < j
fn(i: int, j: int) bool { return i < j; }
```

- Parameter types on function literals may be omitted when the expected
  function type is known: `sort.Slice(xs, fn(i, j) => xs[i].age < xs[j].age);`
- `return` is explicit. Every path in a non-void function must return.

## 6. Errors

`or` is a statement-level postfix operator on a call that returns `T!`.

```hare
var f = os.Open(path)
    or fail;

var f = os.Open(path)
    or err { ... };

os.Remove(path)
    or panic("cannot remove");
```

Forms:

- `call() or fail;` — propagate to the caller.
- `call() or fail("context");` — wrap the error with context and propagate.
- `call() or err { ... };` — handle here; the name is always `err`.
  Inside the block, `fail(err, "context");` wraps and returns.
- `call() or panic("message");` — panic with the message.
- `call() or panic();` — panic with the error value.

The handler always starts on a new line, indented once; `ink fmt` enforces it.

A fresh error is `fail(error("msg"));`. `fail("msg")` alone is not a fresh
error: it is only the shorthand after `or`, where the call supplies the error
to wrap. `errors.New` and `fmt.Errorf` still work through Go interop.

`or` may appear only as the entire initializer of a `var`, the entire right
side of an assignment, an expression statement, or a `return` operand. It may
not be nested inside another expression.

`if (err != nil) { return err; }` (and the multi-result form) is rejected:
propagation has one canonical spelling, `or fail`. `if err != nil` remains
valid for inspection, logging, retry and `errors.Is`; only the pure manual
propagation pattern is an error.

`defer` is block-scoped. Deferred calls run when the enclosing block exits,
including through `return`, `break`, `continue`, `or fail` and panic.

## 7. Control flow

```hare
if (x > 0) {
    ...
} else if (x == 0) {
    ...
} else {
    ...
};

for (var i = 0; i < n; i += 1) {
    ...
};

for (var x in xs) { ... };
for (var i, x in xs) { ... };
for (var k, v in m) { ... };
for () { ... };

match (shape) {
case Circle { radius } => ...;
case Rect { w, h } => ...;
};

match (msg) {
case tea.KeyPressMsg as k => ...;
case tea.WindowSizeMsg => ...;
else => ...;
};
```

- `match` is exhaustive for enums. `else` is the default arm for other types.
- Type patterns are allowed on `any` and interfaces: `case T as name` binds the
  value, `case T` only tests the type. For open types `else` is required.
- Literal patterns include negative numbers (`case -1`) and `nil` for nilable
  subjects (`case nil`). `case nil` is rejected on `any`, interfaces and enums.
- `case _` is not used; write `else`.
- `match` is an expression when every case body is an expression; it is a
  statement when case bodies are blocks.
- `switch` does not exist. Use `if` or `match`.
- `break` and `continue` exist; user labels do not. Inside a `match` case they
  target the enclosing loop: codegen labels the loop so Go's `break` does not
  leave the switch.
- `for` conditions require parentheses.

## 8. Concurrency

```hare
par {
    users = loadUsers(id)
        or fail;
    orders = loadOrders(id)
        or fail;
};

par for (var i = 0; i < len(urls); i += 1) limit 64 {
    sizes[i] = fetchSize(urls[i])
        or fail;
};

within 30s {
    par for (var i = 0; i < len(urls); i += 1) {
        sizes[i] = fetchSize(urls[i])
            or fail;
    };
};
```

- `par` runs its top-level statements as workers and waits at `}`. Statements
  inside a nested block, `if`, loop, `match`, `within` or function literal run
  sequentially in the enclosing worker. A nested `par` starts its own workers
  and error set. Each top-level statement is its own scope; a local declared
  in one worker is not visible in another.
- `par for` runs iterations as workers with an optional `limit` (default
  `GOMAXPROCS`); the whole body is sequential inside one worker.
- `par for` over a map is forbidden: a map has no ordinal, so the winner would
  be nondeterministic. Collect keys into a slice first and iterate the slice.
- `for ()` is forbidden in a `par`/`par for` body: a worker without a
  guaranteed exit can hang the `par`. Function literals are outside the static
  analysis.
- Every `par`/`par for` creates a cancelable context derived from the enclosing
  one, even when no worker can fail statically. `within D` derives a deadline
  from the current worker's context.
- One `ParErrors` per statement. Indexed workers report their ordinal; the
  lowest ordinal wins.
- `or fail` in a worker adds the error at its ordinal, cancels the siblings,
  runs the worker's deferred calls and returns from the worker. After `Wait`
  the statement returns the winning error, or re-panics it in a `void`
  function. Inside a worker, `or panic` is an error with the same delivery, not
  a Go panic.
- A panic escaping a worker becomes a `PanicError` (value + stack), is added at
  the worker's ordinal, cancels the siblings, and `Wait` still completes.
- `defer`, `within` and `or panic` are allowed inside a worker. Each worker has
  its own `DeferStack`; its deferred calls run LIFO at worker exit and before an
  error or panic is reported. A top-level `defer` in a `par` body is itself a
  worker: wrap it in a block when the cleanup belongs to the sibling
  statements.
- `return`, `break` and `continue` may not cross a `par`/`par for` boundary.
  `or fail` and `or panic` are the only ways out of a worker.
- Plain `par`: a captured root may be written by at most one top-level
  statement, and no other statement may read it. Writes through a captured
  pointer, to a captured map slot, to a global, or to `self` are rejected.
  Direct aliases
  (`var q = p`, `q = p`) are attributed to the root.
- `par for`: writes to captured roots are rejected except disjoint slots
  `base[i]`, `base[i].f` or `base[i][j]` where every base of the index chain is
  a slice or array and the outermost index uses the iteration variable. Map
  writes, pointer writes and `self` writes are always rejected.
- A C-style `par for` with an assignment init captures a per-iteration copy of
  each simple-ident LHS; the body sees the copy, while the condition and post
  use the outer variable. A direct write to that variable in the body is
  rejected as a captured write.
- `context` is invisible in Ink. A function receives a hidden context only
  when it or its callees need one. Go functions whose first parameter is
  `context.Context` receive it automatically.
- On a Go interface boundary, codegen emits a public method with the exact Go
  signature and an internal context version. The user writes one method; the
  wrapper lives in `zz_ink.go`.
- Checks are static. `go build -race` stays the dynamic oracle: writes
  through a pointer/interface passed to a free or Go function, writes inside
  function literals, and aliases other than `var q = p` are not tracked as
  writes. A method call on a captured interface value is conservatively a
  write.

## 9. Tooling

- `ink build [-o out] [go build flags]` — translate and compile.
- `ink run [-race] [-- args]` — build and run; `-race` uses Go's race detector.
- `ink check [--json] [--fast] [--race]` — parse, check and type-check
  without building. `--fast` stops after parse, resolve and sema; `--race`
  type-checks with Go's `-race` build (compile-time only).
- `ink test [go test flags]` — run `test` blocks with `go test`; flags such
  as `-race` pass through.
- `ink fmt [-l] [-w] [--json] [paths]` — canonical formatter; `-l` lists unformatted files;
  `--json` prints that list as a JSON array.
  It formats canonical input; old `=`-before-body declarations are rejected by
  the parser, not migrated.
- `ink lsp [--root dir]` — language server on stdio.
- `ink new <dir>` — scaffold a project, install the short `AGENTS.md`, an
  `opencode.json` with the Ink LSP and formatter, and the `ink` OpenCode
  skill with the full reference.
- `ink init [dir]` — add `ink.conf` and `AGENTS.md` to an existing
  directory; an existing `AGENTS.md` is not overwritten.
- `ink guide` — print the embedded agent guide.

`ink check --json` prints one array of `file`, `line`, `col`, `endLine`,
`endCol`, `severity`, `code`, `message` and `hint`. Positions are 1-based;
clean output is `[]`; the exit code is 1 on errors. `ink fmt -l --json`
prints an array of paths. Exit codes are 0 success, 1 diagnostics or failure,
2 usage.

Tests:

```hare
test "append grows the slice" {
    var xs = u8[] {};
    xs = append(xs, 1);
    assert_eq(len(xs), 1);
};
```

`assert(cond)` and `assert_eq(got, want)` are builtins. In tests they report a
failure with `file:line`; outside tests they panic.

## 10. Deliberately absent

These are not planned, not merely missing:

- local named functions; function literals are the only local callable form;
- generic constraints; every type parameter behaves like `any`;
- generic enums;
- guards and tuple/struct patterns in `match`; patterns stay literal, variant
  and type patterns;
- lambda parameter inference without an expected type;
- expression-level `or`; `or` is statement-level only;
- user-declared variadic functions; Go variadic functions can still be called
  with `...`;
- a `discard` builtin; `_` is the only blank/discard construct;
- doc comments attached to declarations; comments are formatter input only.

`go`, `chan`, `select`, `async`, `await`, `try`, `catch`, `?`, `export`,
`let`, `:=`, attributes, traits, lifetimes, `unsafe` in application
code, method declarations outside a type, UFCS, `switch`, labeled control flow,
operator overloading, exceptions.

## 11. Limits

Behavior that follows from §1–10 but can surprise a user is listed in
[`EDGES.md`](EDGES.md), with the workaround and whether the limit is open or
accepted. Deliberate non-goals are in §10.
