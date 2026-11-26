# Ink — `par`, `par for`

Normative annex to [`SPEC.md`](SPEC.md) §8: where SPEC is silent or looser,
this file wins. It covers `par { }`, `par for`, the detached `spawn f(args)` and the
`on err` listener. Removed: `go`, `within`, `or err`, `on fail`, `or panic`,
`or return`.

## 1. Terms

**Worker** — the unit that runs concurrently.

- `par { s1; s2; }`: one worker per **top-level statement** of the body. A
  top-level statement is an immediate element of the body block. Statements
  inside a nested `if`, `for`, `match`, plain block or function literal are
  *not* workers: they run sequentially inside the enclosing worker. A nested
  `par { }` starts its own workers and its own error set. Each top-level
  statement is its own scope: a local declared in one worker is not visible in
  another.
- `par for (...) { body }`: one unit of work per iteration; the whole body
  block is sequential inside that unit. The generator may give a contiguous
  chunk of iterations to one goroutine, so the live goroutine count is bounded
  by `limit`, not by the iteration count. Each iteration keeps its own ordinal
  and error: `?` or `check` records the error and moves to the next iteration,
  not to the next chunk. Bodies with `defer`, `on err`, nested `par`/`spawn`,
  `arena` or `return` keep the one-goroutine-per-iteration path.
- `spawn f(args)`: one detached task, not a worker of any `par` statement. It is
  owned by the program and may outlive the function that launched it. Arguments
  and the receiver are evaluated before the task starts. A method call on a
  value takes its address before launch (as in Go), so the task shares that
  variable.
- **ord** — the worker's index: its spawn ordinal (0, 1, 2, …) for `par`, and
  for `par for` over a slice, array, `str` or C-style loop. Spawn order is
  iteration order for all of these, so "lowest ord" is "lowest index". A `map`
  range in `par for` is rejected by sema: collect keys into a slice first, then
  iterate the slice.
- **Captured root** — a name declared outside the worker: local of the
  enclosing function, parameter, `self`, global. Names declared inside the
  worker are locals.

## 2. Error ordering and delivery

One `ParErrors` per `par`/`par for` statement. Only the winner is delivered; the
other errors are dropped.

- Indexed workers: winner = lowest ord.
- `ParErrors.First()` precedence: the lowest non-negative index wins. Entries
  with `-1` are legacy runtime behavior for map iterations; source-level map
  ranges are rejected, so no Ink program produces them. If present, they are
  considered only when no non-negative entry exists, and then the earliest
  `Add` wins. An indexed error is never hidden by an unordered one.

Delivery:

- No cancel context is created. All workers run to completion; the winner is
  delivered only at the join. There is no early cancellation of siblings.
- `?` in a worker: run the worker's deferred calls, then add the error at
  the worker's ord and leave the worker. A panic in a deferred call is a Go
  panic (§3), not a collected error.
- `par`/`par for` with an error-returning enclosing function: at the join the
  statement is `err := <first>; if err != nil { <listener or return> }`; the
  enclosing block's deferred calls run by normal rules.
- Enclosing scope has an `on err` listener: the winner is passed to it and the
  scope continues.
- Enclosing function cannot return an error (`void` or no `!`) and has no
  listener: a branch that can fail unhandled is a compile error. Every worker
  must handle its own errors.
- `return`, `break`, `continue` may not cross a `par` boundary. `?` is the
  only way out of a worker.
- A fallible call as a bare statement is rejected inside a worker too:
  `f();` drops the error. Use `f()?;`, `f() on err { ... };` or `_ = f();`;
  `_ = f();` is the explicit discard.
- The scope listener is written first in a function or block, is non-empty,
  and cannot `fail` or `return`. It is captured by each task at launch and may
  run after the scope returns.

## 3. Panic

`panic` is Go's panic. There is no recovery boundary and no wrapping:

- A panic in a worker unwinds that goroutine, runs its deferred calls, prints
  the stack and terminates the process with status 2. The other workers are not
  unwound and are not waited for.
- A panic in a detached task does the same; the process does not survive it.
- `on err` never receives a panic. `panic(...)` is not an error constructor and
  `or panic` does not exist.
- There is no `PanicError` and no `AsError`.

Runtime API (normative; `runtime/source.go`, package `inkrt`):

```go
type ParErrors struct{ /* mutex + slice */ }
func (p *ParErrors) Add(index int, err error)
func (p *ParErrors) First() error
```

## 4. Defer per worker

- Each worker owns a `DeferStack` (`var _s_d _s_rt.DeferStack` declared inside
  the goroutine). No worker touches the enclosing function's stack.
- Deferred calls run LIFO at worker exit and before an error is reported
  (`?`). A panic in a worker also runs them, then aborts the process.
- A `defer` written as a top-level statement of a `par` body is itself one
  worker: the deferred call runs when that worker ends, not when a sibling
  ends. Put related statements in a block (`par { { defer f(); ... }; };`) when
  the cleanup must belong to the same worker. This follows from §1: only a
  block is one worker.
- Deferred calls obey the write rules of §5.

## 5. Write rules

Before codegen, for each worker body compute:

- `R(s)` — every captured root referenced by top-level statement/iteration
  `s`, including the base of a write target.
- `W(s)` — every captured root the statement may write.
- Alias: `var q = p;` or `q = p;` where `p` resolves to a captured root makes
  `q` an alias of that root inside the worker (transitive). Aliased writes are
  attributed to the root.

Plain `par`, reject:

1. A captured root in `W` of two or more top-level statements.
2. A captured root in `W(s1)` and in `R(s2)`, `s1 != s2`:
   `read by another par statement: <root>`.
3. Any write through a pointer-typed captured root — deref `*p`, field `p.f`,
   slot `p[i]`, a non-pure method `p.M()` when `p` is pointer-typed, or an
   alias of one of those. Always rejected, even with a single writer:
   `writing to <root> captured from outside par`. Without alias/escape analysis
   disjointness cannot be proven, so it is not attempted.
4. Any write to a slot of a captured `map`, whatever the index:
   `writing to map <root> inside par`.
5. `self`: always a write when the receiver root is `self`, including
   `self.M()`: `writing to self inside par`.

`par for`, reject:

1. A map range: `map iteration is not allowed in par for; collect keys into a
   slice first`.
2. `for ()` in the body: `for () is not allowed inside par; use a bounded loop`.
   Function literals are outside the static analysis.
3. The `self` rule and the map rule above.
4. Any write through a pointer-typed captured root (§5.3).
5. Any other write to a captured root, unless the target is a **disjoint
   slot**: `base[i]`, `base[i].f` or `base[i][j]` where every base of the
   index chain is a slice or array (never a map, never a pointer) and the
   outermost index expression contains the loop's iteration variable. The
   exemption is per write target.
6. Writes to iteration-body locals are free, including iteration variables
   declared by the loop (`var` init, range names). A name assigned by a
   C-style init (`par for (i = 0; ...)`) is not worker-local: a direct write to
   it in the body is rejected as a captured write (§6).

Allowed in both: reads of captured roots, reads of `self` fields, writes to
locals declared inside the worker, and method calls on a served object. A served
object has its own hidden lock; the call is not a captured write. `serve obj;`
itself is rejected inside a worker: the object must be served before sharing. Writes to globals are always rejected in
both forms: a global is shared with the whole program, not only with the other
workers. A write to a captured *value* struct (`c.n = v`) is attributed to `c`
and obeys the same root rules; in `par for` a non-slot write to `c` is rejected.
A method call on a captured value struct (`c.M()`, implicit `&c`) is also a
write to `c`.

## 6. C-style `par for` with assignment init

`par for (i = 0; i < n; i += 1)`, `i` declared outside:

- `i` is an iteration variable of this loop: it is in `iter` for the disjoint
  slot exemption, but it is not worker-local. A direct write to `i` inside the
  body is rejected like any other captured write; reads see the per-iteration
  copy.
- Each worker captures a per-iteration copy before the spawn (`_s_iN := i`);
  body references resolve to that copy. The condition and post still use the
  outer `i`; after the loop `i` holds what the post left.
- Several simple-ident LHS (`i, j: 0, 0`) behave the same way.
- Any other init (call, field target, index target) is rejected in sema. Keep
  that error local to the `par for` init.

## 7. Detached `spawn f(args)`

- `f` must be a declared function or a Go function returning `void` or `void!`.
  A call that returns a value is rejected: use `par { ... }` to collect results.
- Arguments are evaluated before the task starts and passed by value like any
  call. There is no closure; captured locals are not available.
- The task belongs to the program. It may outlive the calling function, a
  request handler or a `par { }` block that launched it. There is no join and
  no `sync.WaitGroup`; if the work must finish, use `par { ... }` instead.
- Error delivery: the task has no caller to return to, so a fallible call
  carries its own handler at the launch site: `spawn f(args) on err { ... };`.
  A `void!` call without a handler is rejected. The handler runs in the task,
  sees `err` and package-level names only, and cannot `fail` or `return`.
  A scope `on err` listener never receives a detached task error.
- `?` after `spawn f(...)` is rejected: there is no function return to use.
- Panics are not recovered (§3).

## 8. Known limits

- No interprocedural alias or escape analysis. Aliases made by a field read
  (`var q = p.next`), an index read or a call result are not tracked; only
  `var q = p` / `q = p`.
- Writes inside function literals are not analysed.
- A method call on a captured interface value is conservatively a write and is
  rejected. Mutations reachable through a pointer/interface passed to a free
  Ink or Go function, or through Go callback state, are not tracked as writes.
- Two writes to different fields of one captured value struct, or to two
  literal slots of one captured slice, are rejected: one root, one writer.
- Struct copies that share a map or pointer field are caught through the field
  type when the write goes through `.m[...]` or `.p...`; slice-field copies
  are only checked for disjoint iteration slots and remain in the static limit.
- Map writes are rejected even for disjoint keys; use a slice indexed by the
  iteration variable. `sync` and `sync/atomic` are rejected in `.ink`: shared
  mutable state belongs in a Go adapter, and `go build -race` is the oracle
  for that adapter.
- No dynamic checks in the compiler. `go build -race` stays the runtime oracle.
- `par` is not tied to a context. Cancelling a `context.Context` does not stop a
  worker or a detached task; both run to completion.
- A growable slice written by a worker cannot be appended to across workers:
  `append` may reallocate, so collect per-worker results and merge after the
  join.

## 9. Checks

Section §5 is enforced by sema. The test suite pins the rules with `par` and
`par for` cases; `ink check --fast` must stay clean for accepted programs and
report the messages quoted in §5. The dynamic oracle remains
`ink run -race` / `ink test -race`.
