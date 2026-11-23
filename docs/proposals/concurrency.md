# Ink — concurrency and errors (proposal)

Status: proposal, not implemented. Replaces the `par` rules in
[`../product/PAR.md`](../product/PAR.md) and the handler set in
[`../product/SPEC.md`](../product/SPEC.md) §10. `arena` is unchanged.

## Decisions

- One keyword for concurrency: `go`.
- No hidden context. `ctx` is an ordinary value, passed explicitly to the Go
  APIs that need it. Nothing is coloured, nothing is wrapped.
- `within` is removed. Use `context.WithTimeout` explicitly.
- Two words for errors: `on err { }` to handle, `or fail` to propagate.
- `panic` is Go's panic: not an error, not wrapped, not recovered.
- No pools, queues, `send`, `close`, `wait`, or handles.

## Syntax

```ink
go { ... };                       // branches, join at the block
go for (...) limit 4 { ... };     // parallel loop, join at the loop
go f(args);                       // detached task: start and continue

on err { ... };                   // scope listener for unhandled async errors
expr on err { ... };              // handle this expression here
expr or fail;                     // propagate
expr or fail("context");          // wrap and propagate
fail(err, "context");             // inside a handler: wrap and propagate
fail(error("msg"));               // inside a handler: fresh error and propagate
error("msg");                     // construct an error
panic("msg"); panic(err);         // Go panic
```

Removed: `par`, `within`, `or err`, `on fail`, `or panic`, `or return`.

## `go { }`

- Each top-level statement of the body is a worker; the block joins them.
- Captured roots follow the existing write rules: one writer per root, no other
  worker reads it, no writes to globals or `self`; pointer writes through a
  captured root, map writes, `break`/`continue` leaving a worker, and `for ()`
  inside a worker are rejected.
- Errors: a worker's unhandled error is collected. At the join the
  lowest-ordinal error is delivered:
  - to the enclosing scope's `on err`, if it has one;
  - otherwise the enclosing function returns it and must be `T!`. In a `void`
    function without a listener, each worker must handle its own errors; a
    branch that can fail unhandled is a compile error.
- No early cancellation: all workers run to completion, then the first error is
  delivered.
- Panics are not recovered: a panic in a worker unwinds that goroutine, runs its
  `defer`s, prints the stack and exits the process with status 2.

## `go for`

- One worker per iteration, at most `limit N` at a time. `limit` is optional;
  the default is `GOMAXPROCS`.
- The body is sequential inside one worker.
- `map` range is rejected; collect keys into a slice first.
- Errors: same delivery as `go { }`; the lowest iteration index wins.
- Panics are not recovered.

## `go f(args)` — detached task

- `f` must return `void` or `void!`. The call returns immediately.
- Arguments are evaluated before the task starts. There is no closure: a
  detached task cannot capture locals. State travels through arguments only.
- The task belongs to the program, not to the calling function, so it can
  outlive a request handler.
- Error delivery: an unhandled error from `f` goes to the `on err` listener of
  the scope where the call was written, captured at launch. Without a listener
  it crashes the program with that error.
- `or fail` after `go f(...)` is rejected: there is no function return to use.
- Panics are not recovered.

## `on err { }`

- At the start of a function or block: the listener for unhandled async errors
  of that scope (`go { }`, `go for`, `go f(...)`). The nearest lexical listener
  wins.
- After an expression: the handler for that expression's error; it takes
  precedence over the scope listener. This form is for synchronous fallible
  calls only; `go` statements have no inline handler.
- The block is never empty, inline or scope. It may use `err` and may either leave the scope
  (`or fail`, `fail(...)`, `return`, `panic`) or continue. A handler that
  continues leaves the expression's value at its zero value; this is the Go
  contract and is documented rather than forbidden.
- The listener is captured by each task at launch and may run after the scope
  returns. It must not rely on the scope still being alive: use it for logging,
  metrics and escalation, not for mutating scope state.

## Errors

- `T!`, `(A, B)!`, `void!` mark a fallible result.
- A fallible call must be handled in place with `on err` or `or fail`; an
  unhandled fallible result is a compile error.
- `or fail` returns the error unchanged; `or fail("ctx")` wraps it with
  `fmt.Errorf("%s: %w", ctx, err)`.
- `fail(err, "ctx")` inside a handler wraps and returns.
- `fail(error("msg"))` inside a handler creates a fresh error and returns.
- `error("msg")` constructs an error; it is not a handler.
- `panic(...)` is Go's panic.

## Panic

- Not an error; `on err` never receives a panic.
- No runtime recovery, no `PanicError`, no wrapping in `go`.
- A panic in any goroutine runs that goroutine's deferred calls and then
  terminates the process with a stack trace. Other goroutines are not unwound.

## Context

There is no hidden context. `context.Context` is an ordinary value:

```ink
fn handler(w: http.ResponseWriter, r: *http.Request) void {
    var ctx, cancel = context.WithTimeout(r.Context(), 2s);
    defer cancel();

    var rows = db.QueryContext(ctx, "select 1")
        on err {
            http.Error(w, err.Error(), 500);
            return;
        };
};
```

- Functions take `ctx: context.Context` explicitly only when they need to pass
  it to a Go API or derive a deadline. Most functions do not.
- `go { }` branches capture `ctx` lexically like any other local; nothing is
  passed implicitly.
- `go` does not create, cancel or derive a context. Cancelling a deadline does
  not stop workers; they run to completion.
- No `_ctx` wrappers, no second function, no `Background` fallback.
- A function using `go` keeps its exact signature: Bubble Tea's `Update` with
  `go` still satisfies `tea.Model`.

## Examples

Repository and HTTP:

```ink
fn saveUser(db: *sql.DB, name: str) void! {
    db.Exec("insert into users(name) values($1)", name)
        or fail("save user");
};

fn user(w: http.ResponseWriter, r: *http.Request) void {
    var u = loadUser(r.URL.Query().Get("name"))
        on err {
            http.Error(w, err.Error(), 500);
            return;
        };
    var data = json.Marshal(u)
        on err {
            http.Error(w, err.Error(), 500);
            return;
        };
    w.Write(data)
        on err {};
};
```

Server with a listener and a detached job:

```ink
fn handle(w: http.ResponseWriter, r: *http.Request) void {
    on err { log(err); };

    go saveUser(1, "hello");
    w.Write(bytes("ok"))
        on err {};
};

fn main() void! {
    http.HandleFunc("/job", handle);
    http.ListenAndServe(":8080", nil)
        or fail("serve");
};
```

The listener is the first statement of a function or block; there are no
top-level statements, so it cannot be declared at package level.

Parallel computation:

```ink
fn load() (i32, i32)! {
    var a = i32(0);
    var b = i32(0);
    go {
        a = fetchA()
            or fail;
        b = fetchB()
            or fail;
    };
    return a, b;
};
```

Frame:

```ink
fn frame(rows: Row[], t: f64) void! {
    go for (var y = 0; y < H; y += 1) limit 4 {
        rows[y] = shadeRow(y, t);
    };
};
```

Deadline from Go:

```ink
fn user(w: http.ResponseWriter, r: *http.Request) void {
    var ctx, cancel = context.WithTimeout(r.Context(), 2s);
    defer cancel();

    var u = loadUser(ctx, r.URL.Query().Get("name"))
        on err {
            http.Error(w, err.Error(), 500);
            return;
        };
};
```

Bubble Tea command:

```ink
fn loadMsg() tea.Msg {
    var data = os.ReadFile("state.json")
        on err {
            return errMsg { Err: err };
        };
    _ = data;
    return nil;
};
```

## Migration

- `par { }` → `go { }`.
- `par for` → `go for`.
- `par f(...)` → `go f(...)`.
- `or err { ... }` → `on err { ... }`.
- `or panic(...)` → `on err { panic(...) }`; in a worker use `or fail("ctx")`.
- `within D { ... }` → `var ctx, cancel = context.WithTimeout(ctx, D); defer cancel();`
  and pass `ctx` explicitly.
- Empty `on err { }` → error; write `_ = err;` to swallow deliberately.

## Tests to add or rewrite

- `go { }`, `go for`: error delivery to listener vs function; no early
  cancellation; void function without listener and an unhandled branch error.
- `go f(...)`: detached lifetime outliving a handler; listener captured;
  `or fail` rejected; panic aborts.
- `on err`: non-empty; scope listener; inline overrides; fall-through zero
  value; handler may `fail`.
- Panic: not caught by `on err`; not wrapped in `go`.
- Remove `within`, `or panic`, `_ctx` wrapper and `par` tests; rewrite the tour
  and Bubble Tea smoke accordingly.

## Open items

1. `go for` is the only awkward spelling; alternatives are `go each` or a
   separate `parallel for`. Confirm `go for`.
2. Panic from a detached task: crash (proposed).
3. A bound value after a fall-through `on err` is the zero value (proposed).
