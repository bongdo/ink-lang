# Ink — language specification (0.2 preview)

This document is the language contract. Older design notes were removed; their
decisions are in [`../maintain/decisions.md`](../maintain/decisions.md). This is
not a 1.0 promise: feature status is in [`FEATURES.md`](FEATURES.md).

## 1. Source files and packages

- Source files end in `.ink`, UTF-8.
- A package is one directory. Every `.ink` file in it belongs to the same
  package. There is no package clause; the directory name is the package name.
- The project root is the directory holding `go.mod`:

```text
module example.com/hello
go 1.27
```

  `go.mod` is the only project file. There is no Ink-specific config. It wins
  over a nested `main.ink`; `main.ink` only marks a root when no `go.mod` exists.

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
- `errors`, `sync` and `sync/atomic` are rejected in `.ink`; so is
  `fmt.Errorf`. Errors are `?`, `on err` and `check`; concurrency is `par`,
  `spawn` and `serve` for shared object state. The generated Go and a Go adapter
  may still use those packages.
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
struct User { ... };
enum Shape { ... };
interface Writer { ... };
alias UserId = u64;
```

A struct, enum or interface declaration has no `=` before its body; an alias
keeps `=`. Declarations start with `struct`, `enum`, `interface` or `alias`;
the `type` keyword is gone.

### 3.1. Builtins

`void`, `bool`, `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `int`,
`uint`, `f32`, `f64`, `str`, `rune` (alias `i32`), `byte` (alias `u8`),
`any`, `error`, `Duration` (alias `i64`), `Type` (runtime type value, for
reflection interop).

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
struct Buffer {
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

### 3.3.1. Contract blocks

A struct body may declare contract blocks. `wire` is the default external view;
any other lowercase block name is a named view of the same contract type:

```hare
struct User {
    ID: i64,
    Email: str,
    Pass: str,

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
        Email: "email",
    };
};
```

- A block is an allowlist: only listed fields are serialized; unlisted fields
  are excluded from that view.
- An entry is `Field: "external-name"`; names are arbitrary strings that
  `encoding/json` accepts as a field tag. Empty names, `-`, quote, backslash,
  comma and control characters are rejected.
- Fields in a view must be exported. A field appears once per block, and an
  external name appears once per block.
- `wire` may appear once per type and is the default view used by
  `encoding/json`. Named views are encode-only in this version; only `wire`
  decodes.
- Named view names start with a lowercase letter and generate a package-local
  method with the same name returning `map[str, any]`: `u.admin()`. A named view
  that is never referenced by a selector is an error.
- Contract blocks are not supported on generic structs in this version.
- A type without blocks keeps Go's default JSON behavior.
- `check`, `store`, `doc` and `policy` are ordinary view names in this version;
  they do not validate, map a database or document anything.

### 3.4. Enums

```hare
enum Shape {
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
interface Writer {
    Write(p: u8[]) int!;
};
```

Implementation is structural. No `implements`, no embedding.

### 3.6. Generics

```hare
struct Stack[Item] {
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
alias UserId = u64;
alias Handler = fn(Request) Response!;
```

An alias does not create a new Go type.

### 3.8. Arenas

```hare
arena {
    var p = new[Point]();
    p.X = 1;
    var xs = i32[](8);
    xs[0] = 2;
};
```

Inside the block `new[T]()` and `T[](n)` or `T[](n, cap: m)` take memory from a
per-type scratch buffer; outside they are ordinary Go allocations. The buffer is
reused by the next execution of the same block, so a loop allocates only until
its high-water mark and then does not allocate at all. Every execution starts
zeroed, so `new` and the slice constructor keep their Go semantics.

The buffer lives as long as the arena can run again. An arena outside a loop
releases its buffer at block exit; an arena inside a loop keeps it for the loop
and releases it after the loop ends. Releasing drops the buffer to the GC, so
memory comes back once the arena cannot be entered again.

Arena memory never leaves the block. Sema rejects assigning it to a captured
variable, returning it, passing it as a call argument or receiver, deferring a
call with it, and taking `&T{}` inside the block. Reading stays: a field, an
index, `len`/`cap`/`min`/`max`, a value copy of a struct or array whose type
holds no references, and `copy` out when the element type holds no references.
So `n = p.X` and `copy(px, row)` work, while `f(p)` and `p.Move()` do not. The
rule is what makes reuse safe: nothing can read the buffer after the block
returns it.

`par` is allowed inside an arena. `new` and `T[](n)` must run before `par`: a
worker shares the block's batch, so an allocation inside a worker is an error.
A worker may write an arena element indexed by its own iteration variable
(`rows[y] = row`) and may open its own `arena { ... };`.

The buffer is an ordinary Go object, so the GC traces it. A buffer whose type
holds no pointers is not scanned, and after the first execution the block
allocates nothing. `arena` does not use Go's experimental `arena` package and
needs no `GOEXPERIMENT` flag.

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
- A map type is its constructor: `map[K, V] {}` is empty and
  `map[K, V](nil)` is nil. There is no implicit allocation.
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
| `asset(path)` | `(u8[], bool)` from `ink build -embed dir` |
| `str(runes_or_bytes)` | conversion to `str` |
| `T[](n)`, `T[](n, cap: m)`, `T[](cap: m)` | `[]T` with length and capacity |
| `error(msg)` | a fresh `error` with that message |
| `panic(msg)` | Go panic; abort |
| `assert(cond)`, `assert_eq(got, want)` | checks for tests and code |

String indexing follows Go: `s[i]` is the byte at `i`. Use `runes(s)` when
code points are needed.

Conversions use the type as a function: `i32(x)`, `u64(x)`, `f64(x)`,
`str(runes)`, `bytes(text)`. `str` on an integer follows Go's `string(rune)`:
`str(65)` is `"A"`, not `"65"`; format numbers with `fmt.Sprint`.

Multiple names are allowed for multi-result calls:

```hare
var value, ok = m["key"];
var n, err = f();
var n, _ = f();
_ = n;
```

### 4.2. Value and reference semantics

Assignment and argument passing copy the value. Slices, maps and pointers are
descriptors that share storage:

| Value | `var b = a` | Effect |
|---|---|---|
| struct, array | copy | fields and elements are independent; slice, map and pointer fields still share |
| slice | header copy | one backing array until `append` grows past `cap` |
| map | descriptor copy | one map; writes through either name are visible |
| pointer | address copy | both names point at the same value |
| interface | interface copy | the underlying value or pointer is shared, not deep-copied |

A parameter behaves like `var param = arg`. `append` may write into the shared
backing array when capacity allows, so keep the returned slice. Map iteration
order is unspecified; collect the keys and sort them when order matters. Go's
`slices` and `maps` packages work through `use`.

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
  `?` propagates the error and yields every non-error result:
  `var a, b = f()?;`.
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

`T!`, `(A, B)!` and `void!` mark a fallible result. Two words handle it:
`?` propagates, `on err` handles.

```hare
var f = os.Open(path)?;

var f = os.Open(path)
    on err { ... };
```

Forms:

- `call()?` — propagate to the caller. `?` is postfix and may appear anywhere
  in an expression; the call must return an error and the enclosing function
  must be fallible (`!`), otherwise it is a compile error. In a `par` worker
  `?` adds the error to the worker set.
- `expr on err { ... };` — handle this expression here; the name is always
  `err`. The expression keeps the zero value of its type when the call fails.
- `on err { ... };` as the first statement of a function or block — a scope
  listener for unhandled errors of `par` constructs (§8). It must be the first
  statement, must not be empty, and may not use `return`.
- `check(cond, "msg");` — return a fresh error with `msg` when `cond` is false.
  It needs a fallible function or a `par` listener and is the only error
  constructor in app code.
- `print(...)` — write a line to stdout like `fmt.Println`; it has no error
  result and needs no `_ =`.
- `panic(...)` — Go's panic, not an error handler (§8.6).

An inline handler may continue instead of leaving the scope; the expression
then keeps the zero value of its type. A scope listener is captured by each
task at launch and may run after its scope returns: use it for logging,
metrics and escalation, not for mutating scope state.

`errors` is not available in `.ink`: `check` creates, `?` propagates, `on err`
handles, and classification is an Ink enum or a Go adapter. `fmt.Errorf` is
rejected too. `error("msg")` is rejected: `error` is a type, not a constructor.

A fallible call used as a bare expression statement is a compile error: its
result would be dropped. Write `call()?;`, `call() on err { ... };` or
`_ = call();`. The rule also applies inside `par { }` and `par for` bodies.

`if (err != nil) { return err; }` (and the multi-result form) is rejected:
propagation has one canonical spelling, `?`. `if err != nil` remains valid for
inspection, logging and retry; a sentinel is compared directly
(`err == sql.ErrNoRows`), and richer classification belongs in an adapter.
Only the pure manual propagation pattern is an error.

`defer` is block-scoped. Deferred calls run when the enclosing block exits,
including through `return`, `break`, `continue`, `?` and panic. A deferred call
takes no handler: its result and its error are discarded.

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
    users = loadUsers(id)?;
    orders = loadOrders(id)?;
};

par for (var i = 0; i < len(urls); i += 1) limit 4 {
    sizes[i] = fetchSize(urls[i])?;
};

spawn saveUser(1, "hello")       // detached task
    on err { log(err); };
```

Two scheduling keywords, `par` and `spawn`. `serve` (§8.4) is the shared-state
primitive, not a third scheduler. No pools, queues, `send`, `close`, `wait` or
handles.

### 8.1 `par { }`

- Each top-level statement of the body is a worker; the block joins them.
  Statements inside a nested block, `if`, loop, `match` or function literal run
  sequentially in the enclosing worker. A nested `par { }` starts its own
  workers and error set. Each top-level statement is its own scope; a local
  declared in one worker is not visible in another.
- `defer` inside a worker is allowed and runs LIFO at worker exit.
- Errors: a worker's unhandled error is collected. At the join the
  lowest-ordinal error is delivered to the enclosing scope's `on err`, if it
  has one; otherwise the enclosing function returns it and must be `T!`. In a
  `void` function without a listener each worker must handle its own errors.
- No early cancellation: all workers run to completion, then the first error
  is delivered.
- Panics are not recovered (§8.6).

### 8.2 `par for`

- One worker per iteration, at most `limit N` at a time. `limit` is optional;
  the default is `GOMAXPROCS`. The body is sequential inside one worker.
- `map` range is rejected: a map has no ordinal, so the winning error would be
  nondeterministic. Collect keys into a slice first and iterate the slice.
- Errors: same delivery as `par { }`; the lowest iteration index wins.
- Panics are not recovered.

### 8.3 `spawn f(args)` — detached task

- `f` must return `void` or `void!`. The call returns immediately.
- Arguments and the receiver are evaluated before the task starts. There is no
  closure: a detached task cannot capture locals. State travels through
  arguments only. For a method call on a value, its address is taken before
  launch, as in Go, so the task shares that variable; mutating it concurrently
  is a data race.
- The task belongs to the program, not to the calling function, so it can
  outlive a request handler.
- A fallible task carries its own handler: `spawn f(args) on err { ... };`. The
  handler runs in the task, after the call, and receives `err`. Without it a
  `void!` call is rejected: on a detached task there is no caller to return to,
  so the error must be handled where it happens.
- The handler body may use `err`, package-level names and its own locals; it
  cannot read locals of the launching function, `fail` or `return`. It may
  `panic`.
- `?` after `spawn f(...)` is rejected: there is no function return to use.
- A scope `on err` listener never receives a detached task error; it belongs to
  `par { }` and `par for` in its own scope.
- Panics are not recovered.

### 8.4 Served objects

- `serve obj;` marks the struct type served and adds one hidden lock to it. It
  is a top-level statement on a whole local or package variable; it cannot
  appear inside `if`/`for`, `par` or `spawn`, and `serve o.inner` is not a
  target.
- Every instance of a served type carries the lock, including one created after
  the `serve` statement or never named by it. A missed `serve` is not a data
  race; write it near the first creation to document intent.
- Every method call on a served type is serialized. A free function with a
  served `*T` parameter is serialized as a whole; when it is called from inside
  a served method, the already-held lock is used.
- Fields of a served object are internal: outside its methods, use methods or a
  free function with `*T`. The served type is not copyable: pass `*T`, never a
  value.
- A method may not return a pointer, slice, map or other reference to its own
  state, nor a value that contains one; `append` of served state, wrapper
  structs and converting served state to `any`/an interface are rejected.
  Return a value with no reference fields, or copy the data into fresh storage.
- `par` workers may call methods on a served object; those calls serialize.
  There is one lock per type; independent parts are separate types.
- `par`/`spawn` inside a served method or a `*T` free function is rejected: the
  lock is already held. Aliasing `self` or a served parameter is rejected.
- `*p = T {}` that overwrites a served object is rejected: it zeroes the mutex.
  Reset fields in a method instead.
- A served object cannot be passed to Go or a builtin. A served type must come
  from the same package; generic types, generic methods and methods that need an
  injected context are not served.
- A same-package factory may return a served object. A type from another
  package cannot be served; keep its lock behind the package that owns it.
- `serve` does not make several method calls atomic and does not prevent
  deadlock between two served objects. A composed update belongs in one method.

### 8.5 `on err` and error order

- A scope listener is `on err { ... };` as the first statement of a function
  or block; its rules are in §6. The nearest lexical listener wins and it
  handles the error; the scope continues.
- An inline `expr on err { ... };` handles that expression only. `par` and
  `spawn` statements have no inline handler.
- `par` and `par for` deliver only the winning error; the others are dropped.
  The winner is the lowest spawn ordinal, which is iteration order for
  `par for`.
- `return`, `break` and `continue` may not cross a `par` boundary. `?` is the
  way out of a worker.

### 8.6 Panic

- `panic` is Go's panic: not an error, never delivered to `on err`, never
  recovered or wrapped by `par`.
- A panic in any worker runs that goroutine's deferred calls, prints the stack
  and exits the process with status 2. Other goroutines are not unwound.

### 8.7 Write rules

- Plain `par`: a captured root may be written by at most one top-level
  statement, and no other statement may read it. Writes through a captured
  pointer, to a captured map slot, to a global, or to `self` are rejected.
  Direct aliases (`var q = p`, `q = p`) are attributed to the root.
- `par for`: writes to captured roots are rejected except disjoint slots
  `base[i]`, `base[i].f` or `base[i][j]` where every base of the index chain is
  a slice or array and the outermost index uses the iteration variable. Map
  writes, pointer writes and `self` writes are always rejected.
- A C-style `par for` with an assignment init captures a per-iteration copy of
  each simple-ident LHS; the body sees the copy, while the condition and post
  use the outer variable. A direct write to that variable in the body is
  rejected as a captured write.
- `for ()` is rejected inside a worker without a guaranteed exit. Function
  literals are outside the static analysis.
- Checks are static. `ink run -race` and `ink test -race` stay the dynamic
  oracle: writes through a pointer/interface passed to a free or Go function,
  writes inside function literals, and aliases other than `var q = p` are not
  tracked as writes. A method call on a captured interface value is
  conservatively a write.

### 8.7 Context

- There is no hidden context. `context.Context` is an ordinary value, passed
  explicitly to the Go APIs that need it. Functions take one only when they
  pass it to a Go API or derive a deadline; most do not.
- `par { }` branches capture `ctx` lexically like any other local; nothing is
  passed implicitly.
- `par` does not create, cancel or derive a context. Cancelling a deadline does
  not stop workers; they run to completion.
- No `_ctx` wrapper and no second function: a function using `par` keeps its
  exact signature, so Bubble Tea's `Update` with `par` still satisfies
  `tea.Model`.

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
- `ink init [dir]` — add `go.mod` and `AGENTS.md` to an existing
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
failure with `file:line`; outside tests they panic. A failed assertion or a
panic in one test fails that test only; the remaining tests in the package
still run.

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

`chan`, `select`, `async`, `await`, `try`, `catch`, `export`,
`let`, `:=`, attributes, traits, lifetimes, `unsafe` in application
code, method declarations outside a type, UFCS, `switch`, labeled control flow,
operator overloading, exceptions.

## 11. Limits

Behavior that follows from §1–10 but can surprise a user is listed in
[`EDGES.md`](EDGES.md), with the workaround and whether the limit is open or
accepted. Deliberate non-goals are in §10.
