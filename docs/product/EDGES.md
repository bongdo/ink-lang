# Ink — edges

Every place where the current language can surprise a user. Ranked by how
likely it is to bite, not by effort. No issue numbers: an entry either lives
here or is a feature in [`FEATURES.md`](FEATURES.md).

**open** — needs a fix or an explicit decision · **accepted** — we keep it, do
not re-file.

Варианты решения по каждому открытому краю — [`../proposals/edges.md`](../proposals/edges.md).

## Open

Нет открытых краёв. Каждый прежний либо исправлен, либо закрыт решением —
ниже, в Accepted, с ценой и границей.

## Accepted

| Symptom | Why it stays |
|---|---|
| A method set that cannot be named (generic instantiation, Go variadic) is not compared. | The warning `conformance-unchecked` marks it and `go build` is exact; the assignment exists in the generated Go either way. |
| A rename is verified by re-resolving references in a sema pass over the edited snapshot. | Renames of locals, globals, functions and types are rejected when they break or rebind; Go symbols are out of scope, and a field rename is name-only so it cannot rebind. |
| A running sema pass stops between declarations, not inside one. | A pathological single declaration still completes; `ink check` is one process, the LSP discards the stale result. |
| `ink check` re-type-checks the project, not a package. | Measured: 0.58 s warm on `examples/tour`, 0.5 s with `--fast`; Go's build cache already skips unchanged packages. Revisit above ~2 s. |
| `asset(path)` reports not found when the binary was built without `-embed`. | The flag is build-time by decision; the language has no embed attribute. Build with `ink build -embed dir`. |
| JSON names and subsets are declared in `wire`/named blocks inside the struct, not Go tags. | One declaration owns the external names; unlisted fields are excluded; named views are explicit. |
| Named views are package-local and encode-only. | Their generated method is lowercase, so another package cannot use it, and the unused-view check stays sound. Decoding uses the default `wire` view. |
| Contract blocks on generic structs are rejected. | A scope cut for this version; generic receiver methods are legal Go, so this can be lifted without changing the syntax. |
| `T or error` and `or return` are rejected. | `T!` and `or fail` replaced them; diagnostics point at the old form. |
| `par` is rejected and `within` is removed. | The concurrency keyword is `go`; a deadline is `context.WithTimeout`, passed as an ordinary value. |
| `go f(args)` is owned by the program, not by the calling function. | A detached task may outlive a request handler. State travels through its arguments; its errors are its own, so a fallible task needs `go f(...) on err { ... };` at the launch site. |
| Process exit does not wait for detached tasks. | Go's contract: when `main` returns the process exits, the tasks and their `defer`s are dropped. Use `go { ... }` or an explicit `sync.WaitGroup` when the work must finish. |
| `go { }` and `go for` join and never cancel early. | All workers run to completion, then the lowest ordinal error is delivered. A deadline does not stop them. |
| A detached task's `on err` handler runs in the task, not in the launching scope. | It sees `err` and package-level names only; locals of the launcher are rejected at compile time, so the handler cannot race with the caller. |
| A panic is a Go panic. | `on err` never receives it and `go` does not recover it; the process exits with status 2. This is the Go contract, not a gap. |
| A scope `on err` cannot `return` but can `fail`; a task handler can do neither. | The scope listener escalates to its function. A detached task has no caller, so its handler only logs, records or panics. |
| An empty `on err { }` is an error. | A silent handler would swallow the error with no trace; write `_ = err;` to swallow deliberately. |
| The value bound after an inline `on err` falls through is the zero value. | The handler may continue instead of leaving the scope, and the expression still needs a value. |
| A fallible call cannot be a bare statement. | `f();` would drop the error; `_ = f();` is the explicit discard. The rule also applies inside `go` branches. |
| Arena values must not escape the `arena` block. | The block returns its buffer for reuse; a value that outlived it would read the next execution's data. Sema rejects the common escapes; a value stored by a callee or in a captured container may still leak. |
| Arena memory never crosses a function boundary. | The check is syntactic; helpers cannot take arena values, because a callee could keep the pointer past the block. Copy data out with `copy`. |
| An arena value may be read as a field, an index or a value copy. | A struct or array copy is allowed only when its type holds no references; a copy of references would still point into the buffer. |
| A worker writes arena elements indexed by its own iteration variable. | `go` proves disjoint slots through the loop variable; an index from another loop is rejected as a captured write. |
| `new` and `T[](n)` inside `go` inside `arena` are rejected. | A worker shares the block's batch with the loop, so allocate before `go` and let workers fill the memory. |
| A worker may open its own `arena { ... };`. | The block starts and ends inside the worker, so its batch is local to that goroutine. |
| Arena memory is released after the enclosing loop. | Reuse inside the loop needs the buffer to outlive one execution; a one-shot arena releases at block exit. A program-long loop keeps its buffer until it ends. |
| `fail` is a reserved word. | It is the error action; a user function or variable named `fail` must be renamed. |
| `nil-map-write` sees variables declared `= nil`, not struct fields or parameters. | A field is nil until assigned and a parameter may be nil at run time; without flow analysis any warning on them would be false most of the time. |
| A map is nil until it is assigned; there is no implicit allocation. | `map[K, V] {}` is empty, `map[K, V](nil)` is nil; `nil[T]` also works. Writing to a nil map is the warning `nil-map-write`. |
| The slice constructor needs a nameable element type: `*Point[](n)` parses as a dereference. | `T[](n)` is the type applied to a size; a pointer or nested slice element needs an alias. | `type PointPtr = *Point;` then `PointPtr[](n)`. |
| `new` is a builtin name; `make` was removed. | A user function named `new` is rejected and must be renamed; `make` in source is a diagnostic with a hint. |
| The handler error is always named `err`. | One name; an outer `err` is shadowed inside the block and the listener. |
| Import paths cannot contain spaces or quotes. | Paths are bare; valid Go module paths fit the path charset. |
| Unsuffixed integer literals are Go `int`; `i32` is int32 and needs `i32(...)`. | Matches Go; `len`, `cap` and `os.FileInfo.Size` return Go integers. |
| `s[i]` is a `byte`, not a rune. | Matches Go; `runes(s)` is the documented form for code points. |
| Map writes are rejected in `go` even for disjoint keys, and `go for` cannot range over a map. | A map has no ordinal and no provable disjointness; collect keys, then iterate a slice. |
| Two writes to different fields of one captured struct are rejected. | One root, one writer: without alias analysis the compiler cannot prove they are disjoint. |
| A method call on a captured value struct or interface counts as a write. | The receiver may mutate; the checker is conservative. Copy the value locally when the call is pure. |
| A pointer passed to a free or Go function, or an alias from a field/index read or a call result, is not tracked as a write. | No interprocedural alias or escape analysis. `ink run -race` and `ink test -race` are the dynamic oracle. |
| `for ()` is rejected inside `go`. | A worker without a guaranteed exit can hang the block. |
| A C-style `go for (i = 0; ...)` makes the body read a per-iteration copy of `i`; after the loop `i` holds the post value. | Deterministic capture without a closure per iteration. |
| `or` is statement-level only: no `f(g() or fail)`, no `or` in multi-RHS or a `for` post. | No expression-level control flow; split the call into two statements. |
| A Go function returning `(T, bool)` needs two variables, not `or`. | `or` is for `error`; `ok` is a normal result. |
| A local `const` is an error. | `const` is a package-level compile-time name; use `var` locally. |
| Manual `if err != nil { return err; }` is rejected. | `or fail` is the one canonical propagation form. |
| `match` on `any` requires `else`. | Open types have no exhaustiveness; `else` makes the default explicit. |
| Enum variants cannot share a name in one package, and an enum's zero value is not constructible. | Bare `case` patterns stay unambiguous; construction is explicit. |
| `nil` needs an expected type or `nil[T]`. | `nil` is untyped; typed `var` covers most positions. |
| `chan` and `select` do not exist in app code. | Channels are a second concurrency model; keep them in a Go binding. `go` is the only Ink concurrency keyword. |
| Methods always use a pointer receiver. | One receiver form; no value/pointer method-set split. |
| No doc comments attached to declarations. | Comments are formatter input only; `ink guide` is the reference. |
