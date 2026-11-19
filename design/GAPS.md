# Ink — gap list (0.2 preview)

This is the honest list of what is missing or only partially implemented.
It is not a 1.0 checklist: it is the work needed before calling the language
finished.

**Status:** 0.2 preview. The compiler, formatter, LSP and tests work end to end.
0.2 adds multiple results, type patterns, typed `var` with bare `nil`, context
wrappers for Go interfaces, Go type diagnostics in `check`/LSP, interface
conformance checks, and the `par` core (G1–G9). It also ships the agent
contract: `check --json` with codes, hints and end positions, `--fast`,
`--race`, the short `AGENTS.md`, the full `ink guide` reference and the
`ink` OpenCode skill, and LSP
references/rename/code actions/workspace symbols with test-block diagnostics.
The current surface also has canonical `type`/`fn` declarations, package-only
`const`, `break` in `match`, `void` checks, versioned import aliases, and the
`par` map/`for ()` restrictions. The remaining concurrency gaps are the context
boundary (G10) and the static-analysis limit covered by the race detector
(G11).

Effort: **S** — hours, **M** — days, **L** — a week or more.

Gap IDs are stable. Numbers missing from this list were dropped as won't-do;
[`SPEC.md`](SPEC.md) §10 is the canonical list of deliberate absences.

## What works today

- `.ink` files, one directory = one package, `ink.toml`, local and Go imports.
- canonical declarations: `type User struct`, `type Shape enum`,
  `type Writer interface`, alias `type UserId = u64`, `fn f() R { ... }`.
  Old declarations with `=` before the body are rejected.
- structures with fields/methods and implicit `self`; enums with exhaustive
  `match`; interfaces; generics without constraints; aliases.
- package-level `const` (compile-time name, no type, sizes arrays and matches
  case patterns); typed `var` (`var p: *T = nil`, `var a, b: T = ...`);
  bare `nil` with declared/expected type and `nil[T]` as an escape hatch;
  slices/arrays/maps/pointers, `runes`/`bytes`/`str` conversions,
  `len/cap/append/copy/delete/has/min/max`.
- multiple results: `fn f() A, B or error`, `return a, b;`, `var a, b = f();`,
  `var a, b = f() or return;`.
- `match` on `any`/interfaces with `case T as name`, `case T` and `else`;
  `break`/`continue` inside a case target the enclosing loop.
- `T or error` with `or return`, `or err { ... }`, `or panic(...)`;
  block-scoped `defer`; statement-level forms. `void` is only a function
  result; value positions are rejected.
- `if`, all `for` forms, `match` statements/expressions.
- `par`, `par for ... limit`, `within`; panic recovery, per-worker
  `defer`/`within`, deterministic indexed errors, captured read/write rules.
  `par for` over a map and `for ()` in a `par` body are rejected; aliases
  beyond direct assignment remain a static limit (G11).
- `ink check` is the authoritative whole-project check: it parses, checks and
  type-checks generated Go for every file in the project, including `test`
  blocks. `--fast` stops after sema, `--json` prints machine-readable
  diagnostics, and `--race` type-checks with the race detector.
  `ink build/run/test` still rely on `go build` for type errors.
- LSP diagnostics and editing support (references, rename, code actions,
  workspace symbols, test-block diagnostics, 150 ms debounced typecheck);
  VS Code grammar/client; Neovim setup.

## A. Concurrency (`par`, `par for`, `within`) — core closed

G1–G9 are fixed in 0.2; [`PAR.md`](PAR.md) is the normative description of the
implemented semantics. The remaining items are G10 (the Go boundary remainder)
and G11: the checks are static, and the Go race detector stays the dynamic
oracle.

### G1. Panic inside `par` — fixed in 0.2
Every worker runs inside a recovery boundary. A panic becomes a `*PanicError`
(value + stack) at the worker's ord, cancels the shared context and lets `Wait`
finish. An error-returning function returns the winning error; a `void`
function re-panics it at the `par` statement. `or panic` inside a worker adds
an error, cancels and returns instead of raising a Go panic.

### G2. `within` inside `par` — fixed in 0.2
Each worker owns its `DeferStack`. A `within` inside a worker derives from the
`par` cancel context, applies only to that worker, and sibling cancellation
still reaches it.

### G3. `defer` inside `par` — fixed in 0.2
Deferred calls go to the worker's own stack and run LIFO at worker exit, before
an error or panic is reported. A top-level `defer` in a `par` body is its own
worker; wrap it in a block when the cleanup belongs to the sibling statements.

### G4. `or panic` inside `par` — fixed in 0.2
`or panic(msg)` and `or panic()` add an error at the worker's ord, cancel the
siblings and return from the worker. They make the enclosing `par` fallible.

### G5. Map writes and map ranges — fixed in 0.2
Any write to a slot of a captured map is rejected in `par` and `par for`, even
with the iteration variable as the index: `writing to map <root> inside par`.
A map range in `par for` is rejected outright:
`map iteration is not allowed in par for; collect keys into a slice first`.
Collect keys sequentially, then run `par for` over the slice.

### G6. Pointer and alias writes — fixed in 0.2
A write through a pointer-typed captured root (`*p`, `p.f`, `p[i]`, a non-pure
`p.M()`) and its direct aliases (`var q = p`, `q = p`) is rejected: `writing to
<root> captured from outside par`. A method proven pure is allowed. Writes to fields of a captured value struct are
attributed to the struct root. Aliases from field/index reads and call results
are not tracked; those remain in the static-analysis limit (G11).

### G7. Plain `par` read/write sets — fixed in 0.2
Each top-level statement is its own scope; a local declared in one worker is
not visible in another. For each top-level statement sema computes captured
read and write sets. A root may have one writer, and no other statement may
read it:
`read by another par statement: <root>`. Writes to `self` and to globals are
always rejected; a global is shared with the whole program.

### G8. C-style `par for` capture — fixed in 0.2
`par for (i = 0; ...)` with an outer `i` captures a per-iteration copy
(`_s_iN := i`); the body sees the copy while the condition and post use the
outer `i`. A direct write to `i` in the body is rejected as a captured write.
Iteration variables declared by `var` init or range are worker-local: writes to
them are allowed. Non-identifier init targets are rejected by sema.

### G9. Map error order — fixed in 0.2, now unreachable
Indexed workers report their spawn ordinal, so the lowest index wins. The
runtime still understands ord `-1` and `ParErrors.First()` prefers any
non-negative index, then the first completed error. Since G5 now rejects map
ranges in `par for`, no Ink source produces `-1`; the path stays in the
runtime for compatibility.

### G10. Context coloring at the Go boundary
**Статус:** partially fixed in 0.2 for direct calls, methods and Go callbacks;
the boundary remainder is open.
**Сейчас:** a function or method that needs a hidden context gets a public
wrapper with the exact Go signature and an internal `_s_..._ctx` body. The
wrapper calls the internal function with `context.Background()`; Ink calls
inside a `within`/`par` pass the real context. Internal functions called across
package boundaries go through the public wrapper.
**Остаток:** the limit is the boundary itself. Interface dispatch through a
Ink interface, package-level initializers and function values are not
context-colored: a call through them uses the public wrapper with
`context.Background()`, so a `within` deadline does not cross a package
boundary or a function value. Timers and IO are executed by `tea.Program`, not
by Ink.
**Документация:** `ink guide` §Concurrency and
[`examples/limits/README.md`](../examples/limits/README.md).

### G11. `par` checks are static only
**Статус:** documented and covered by the race path; the static limit remains.
**Сейчас:** `ink run -race` builds with Go's race detector,
`ink test -race` runs tests with it, and `ink check --race` type-checks
with `-race` (compile-time only, no runtime instrumentation). Map ranges in
`par for` and `for ()` in `par` are sema errors; the remaining static limit is
alias, escape and function-literal analysis.
**Почему:** the Ink compiler has no runtime instrumentation.
**Фикс:** none for the race path; Go's detector stays the dynamic oracle.
**Документация:** `ink guide` §Concurrency and
[`examples/limits/README.md`](../examples/limits/README.md).
**Оценка:** S.

## B. Language surface

### G13. Multiple results are implemented, tuples are not
**Статус:** fixed in 0.2.
**Синтаксис:** `fn f() A, B or error`, `return a, b;`, `var a, b = f();`,
`a, b = f();`. Function types use `fn(i32) (str, bool)`.
**Границы:** no tuple type, no tuple values, no indexing or nesting in
arguments. `or` handles every non-error result: `var a, b = f() or return;`,
`a, b = f() or return;`, `return f() or return;`. `return f();` passes a
fallible multi-result call through. This unlocks `tea.Model` and Go APIs.

### G14. `match` on `any` and interfaces
**Статус:** fixed in 0.2.
**Синтаксис:** `case T as name => ...` binds, `case T => ...` tests only,
`else => ...` is the default. Enums keep variant patterns. Open types require
`else`.
**Границы:** patterns are literal, variant or type patterns only; no guards, no
tuple/struct patterns, no multi-type case lists.

### G18. Type arguments on calls — fixed
**Статус:** fixed in 0.2 preview. Sema resolves every explicit `TypeArg` with
`resolveType`, stores `TypeParams`/`TypeArgs` on `CallInfo`, and instantiates
Ink parameter and result types. Explicit arguments on Go generic functions are
accepted (codegen already emits them) and their arity is checked. Wrong arity
and type arguments on a non-generic Ink function are errors.
**Остаток:** when explicit arguments are omitted, sema still leaves Ink type
parameters unresolved and lets Go infer them; Go generic result types are not
substituted into Ink hover types.
**Оценка:** S.

### G20. `nil` typing — fixed in 0.2
**Сейчас:** typed `var` gives `nil` an expected type: `var p: *Point = nil;`,
`var e: error = nil;`. One type applies to all names:
`var a, b: T = ...`. Bare `nil` is the default; `nil[T]` remains an explicit
escape hatch for `any`/interface contexts. Only pointers, slices, maps,
functions and interfaces are accepted. `const` cannot have a type, and typed
`void` is rejected. Generated nil is `(T)(nil)`, not `*new(T)`, so the value is
a real nil.

### G21. `break` inside a `match` case — fixed in 0.2
**Сейчас:** `break` and `continue` inside a `match` case target the enclosing
loop. Codegen labels loops that contain a match with `break`/`continue` and
emits `break _s_loop` / `continue _s_loop`, so Go's switch is not left. User
labels still do not exist.

### G23. String indexing returns a byte — documented
**Статус:** fixed in 0.2 preview. `s[i]` is Go's byte and sema types it as
`byte`; `runes(s)` is the documented form for code points. Changing `s[i]` to
rune was rejected because it would surprise Go users.
**Оракул:** `TestStringIndexIsByte` asserts the sema `ExprTypes` entry and
fails on the pre-G23 nil type. `tests/cases/string_index_byte` pins the
positive declaration (`var b: byte = word[0]`), and
`tests/cases/string_index_rune` pins the exact `go-type` error for
`var r: rune = word[0]` at 3:20. `tests/cases/strings` only pins runtime
byte-vs-rune output; it is not a sema regression oracle.
**Оценка:** S.

## C. Sema and type checking

### G26. Type errors in `check` and LSP — fixed (prototype) in 0.2
**Сейчас:** `ink check` runs `internal/typecheck.Check` after a clean
parse/sema pass. It generates Go in memory, loads it through `go/packages` with
an overlay, and maps Go diagnostics back through `//line`; `ink check` exits
1 on `var n = 1 + "x";`. The LSP runs the same check on `initialize`,
`didOpen`, `didSave` and `didClose`, and after a debounced `didChange`, and
publishes diagnostics with source `ink`.
**Границы:** `build/run/test` still rely on `go build` for type errors; the
LSP type-check includes `test` blocks and reports real start columns through
`//line`, but sema and typecheck end spans are one-character fallbacks (G34).
Sema still runs synchronously on every change (G43).
[`TYPECHECK.md`](TYPECHECK.md) is the detailed contract.
**Остаток:** per-package cache and exact end spans.

### G27. Interface conformance — fixed in 0.2
**Сейчас:** sema computes method sets for local structs, local interfaces and
Go types. Assignments, arguments, returns and composite elements are checked at
the source position: missing method, wrong parameter/result, missing error,
unexported cross-package method, value receiver where a pointer is required
(`does not implement Writer: ...`).
**Остаток:** signatures that cannot be named are skipped rather than guessed:
generic types and type arguments, enum variants, array types, function values
and variadic methods. An explicit Go `context.Context` parameter is compared as
a normal parameter; a hidden-ctx implementation does not match it. Multi-result
assignments/returns are currently skipped because the call info has several
results.

### G28. Enum variant names — fixed in 0.2
**Сейчас:** duplicate variant names in one package are rejected
deterministically (`enum variant Y is already declared in A`), so bare `case`
patterns are unambiguous. A `<Enum><Variant>` GoName colliding with a later
type can still be missed depending on map iteration order; that is a codegen
symbol-table limit, not a variant-name ambiguity.
**Фикс (remaining):** final package-wide symbol table pass after collection for
GoName collisions.
**Оценка:** S.

### G29. Top-level forward references — fixed
**Статус:** fixed in 0.2 preview. Package variables are resolved to a fixpoint,
so `var a = b; var b = 1;` gives `a` the type of `b`. The cycle graph covers
declared types too: `var a: i32 = b; var b = a + 1;` is reported by
`ink check --fast` as `initialization cycle: a -> b`. References inside
non-invoked function literal bodies are skipped; immediately invoked literals
are traversed, so `var a: i32 = fn() i32 { return b; }(); var b = a + 1;` is
reported by `ink check --fast` as well.
**Остаток:** the graph does not traverse free function calls. A cycle through
`fn getB() { return b; }; var a = getB(); var b = a + 1;` is clean under
`--fast` and is caught only by the full Go typecheck (`initialization cycle for
a`).
**Оценка:** M.

### G30. `void` value positions — fixed in 0.2
**Сейчас:** `void` is allowed only as a function result. Sema rejects it in
variables (local and package), fields, parameters, arguments, returns with
values, composite elements, slice element types and generic arguments.
**Почему:** `void` is not a value; the restriction is checked before Go sees
the generated code.

### G31. Unused locals/dead code — partly fixed by G26
**Сейчас:** `ink check` type-checks generated Go, so unused locals and
similar Go diagnostics are reported there; the LSP gets them on open/save and
after the debounced check on change.
**Остаток:** no fast unused-local pass for LSP, and dead-code/vet diagnostics
are not a separate Ink pass.
**Оценка:** S/M.

## D. Codegen and runtime

### G32. Generated Go can contain unreachable `panic("unreachable")`
**Статус:** partly fixed in 0.2 preview. Sema now reports `missing-return` when
a non-void function can fall off its syntactic end (`return`, `panic`, complete
`if/else`, complete `match`, and an infinite `for` with no `break` all count).
**Остаток:** codegen still appends a trailing `panic("unreachable")` to every
function with a result, so `go vet` can warn; the check is syntactic and not
flow-sensitive: an `if/else` whose branches terminate syntactically but fall
through semantically remains a false negative. Function literals are not
checked at all, so a non-void literal can still fall through and `ink run` still
panics with `panic: unreachable`.
**Фикс (codegen):** emit the panic only when the flow analysis proves a path can
fall through.
**Оценка:** S.

### G33. Duplicate std imports when the user imports `fmt`/`os`
**Сейчас:** generated code can contain both `_s_fmt "fmt"` and `fmt "fmt"`;
Go accepts it, but it is noise.
**Почему:** fixed generated aliases and user aliases coexist.
**Фикс:** build a path→alias table; use the user alias for generated code when
it already imports the same path.
**Оценка:** S.

### G34. `//line` columns and paths
**Статус:** fixed for paths in 0.2 preview. `//line` uses a path relative to
the generated file, so `zz_ink.go` no longer embeds the absolute project path;
`typecheck` maps relative paths and package test bundles back to source files,
including paths with spaces. Columns are kept; `check --json` carries
`endLine`/`endCol`.
**Остаток:** sema and typecheck end positions are still one-character
fallbacks; Go diagnostics carry no end column.
**Оценка:** S.

### G35. `GoType.Text` can reference packages that are not imported
**Сейчас:** inferred Go callback types (e.g. `fs.DirEntry`) may print a package
name without an import, breaking generated code.
**Почему:** `types.TypeString` uses a qualifier that falls back to the package
name.
**Фикс:** collect required imports while printing Go types; add them to the
file import set; fail with an Ink error if a type cannot be named.
**Оценка:** M.

### G36. `or` in multiple RHS / a `for` post is rejected
**Статус:** by design. `or` stays statement-level; these positions give a clear
error. The restriction is listed in [`SPEC.md`](SPEC.md) §10.

### G37. Generation always rewrites whole files — fixed
**Сейчас:** `zz_ink.go` and `zz_ink_test.go` are written only when their bytes
change. `.ink/generated.json` lists every generated file; a build removes the
outputs of packages that no longer exist. A missing or broken manifest is
ignored and rewritten.
**Оценка:** S.

### G38. Runtime is copied on every build — fixed
**Сейчас:** `.ink/runtime/runtime.go` is compared with the embedded source and
written only when it differs, so a compiler upgrade still refreshes it.
**Оценка:** S.

## E. Tooling and LSP

### G39. LSP type diagnostics — fixed in 0.2
**Сейчас:** `initialize`, `didOpen`, `didSave` and `didClose` run the Go
type-check and publish mapped diagnostics with source `ink`; `didChange`
schedules the same check after a 150 ms debounce and drops stale results.
Parser/sema diagnostics work as before, and the type-check includes `test`
blocks. Start columns come from `//line path:line:col`.
**Остаток:** exact end columns (G34), per-package cache; the sema pass itself is
still synchronous (G43).

### G43. LSP sema is synchronous; typecheck is debounced — fixed
**Статус:** fixed. `didChange` only snapshots the documents and submits a
generation; parse and sema run on an analysis worker with a latest-version
queue, so a burst of edits runs the last one and an outdated result is not
published. The debounced typecheck is scheduled from the published result and
stopped on shutdown.
**Остаток:** a running analysis is not cancelled mid-pass; a newer generation
waits for it to finish. Shutdown waits for the current pass.
**Оценка:** M.

### G44. LSP shim drops `didChange` when its queue is full — not reproduced
**Статус:** the current `internal/lsp` transport has no bounded outbound queue:
notifications are written synchronously under a mutex, and `didChange` messages
are read and handled in order. If a client shim reintroduces a queue, coalesce
per URI (keep only the latest text) instead of dropping updates.

### G45. LSP diagnostics for closed project files — fixed for buffers
**Статус:** `didClose` publishes empty diagnostics for the closed URI, deletes
the in-memory buffer and re-analyzes from disk. Diagnostics for the file on
disk can appear again by design.

### G46. Formatter comment fidelity and wrapping
**Статус:** fixed for trailing comments in 0.2 preview. A comment after
`use`, a top-level `};`, a struct field, a statement, a block or the last
`match` arm stays on its line, and one `ink fmt -w` is a fixed point:
`ink fmt -l` is empty after the first write. The golden test also formats the
output a second time.
**Остаток:** no line-width wrapping; an optional `--wrap` remains a later
feature, not a language gap.
**Оценка:** M.

## Bubble Tea

Bubble Tea is Go and is callable from Ink. 0.2 removes the two language
blockers:

- `Update` returns `(tea.Model, tea.Cmd)` — multiple results (G13).
- messages are `any` and are handled with `case T as name` (G14).

No Go adapter is needed. `*Model` implements `tea.Model` structurally; if
`Update`/`View` use `par`/`within`, G10 generates a public method with the exact
Go signature and an internal context version. `Cmd` is executed by
`tea.Program`. The compile-only smoke lives in
`tests/smoke/bubbletea` and pins `charm.land/bubbletea/v2 v2.1.0`; soft wrap,
clipboard, themes and completion are separate UI work.

## Priorities

Agent readiness is priority 1, ahead of 1.0 polish. The detailed roadmap is in
[`AGENT.md`](AGENT.md). The order below replaces the old 1.0-first plan.

1. **P0 — agent contract and feedback.** Implemented: agent guide, JSON
   diagnostics with codes/hints, `--fast`, diagnostics on change and a stable
   CLI contract. A2 remains partial: parser ends are exact, sema/typecheck ends
   are one-character spans; `//line` paths are relative (G34).
2. **P1 — agent editing.** Implemented: LSP references/rename, code actions,
   test diagnostics and workspace symbols. A7 remains partial: the
   `nil-untyped` fix is conservative and rename does not check conflicts.
3. **P2 — concurrency confidence.** Implemented: `-race` path
   (`run`/`test`/`check`), G10 boundary docs, G11 static-analysis docs.
   G1–G9 are closed; G10 and G11 remain as documented limits.
4. **P3 — 1.0 polish.** G32–G38, G43, G44 and optional formatter wrapping,
   performance and the compatibility freeze.

G13, G14, G18, G20, G21, G23, G28, G29, G30, G34, G39, G45 and G46 are fixed.
G43 and A2/A7 are partial.
G44 is not reproduced in the current transport; G45 is fixed for buffer
diagnostics. G36 is a clear diagnostic, not a missing feature.

## Commands

CLI:

```sh
ink build [-o out] [go build flags]
ink run [-race] [-- program args]
ink check [--json] [--fast] [--race]
ink test [go test flags]
ink fmt [-l] [-w] [--json] [paths]
ink lsp [--root dir]
ink new <dir>
ink init [dir]
ink guide
ink version
ink help
```
