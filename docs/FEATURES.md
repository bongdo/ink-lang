# Ink — features

The surface of the 0.2 preview. `SPEC.md` is normative; this file is the only
place with status.

**✅ implemented** · **◐ partial** · **○ planned** · **— deliberate absence**

## A. Source and packages

| Feature | Status |
|---|---|
| `.ink` files, UTF-8 | ✅ |
| one directory = one package, no package clause | ✅ |
| `ink.toml` with `name` and Go module path | ✅ |
| local import `use ./lib as lib;` | ✅ |
| Go import `use fmt;`, bare paths, `as` alias | ✅ |
| versioned import path requires an alias | ✅ |
| blank import `as _` | ✅ |
| generated `zz_ink.go`, `zz_ink_test.go`, `.ink/` | ✅ |
| project-wide check across files and `test` blocks | ✅ |

## B. Declarations and types

| Feature | Status |
|---|---|
| `type X struct { ... }` with fields and methods | ✅ |
| implicit `self`, pointer receivers | ✅ |
| `type X enum { ... }`, sealed, variant construction | ✅ |
| exhaustive `match` on enums | ✅ |
| `type X interface { ... }` | ✅ |
| interface conformance (local and Go interfaces) | ◐ signatures that cannot be named are left to `go build` |
| `type X = T` alias | ✅ |
| generics `Stack[T]` | ✅ |
| generic constraints | — every type parameter behaves like `any` |
| generic enums | — |
| generic methods with their own type parameters | — |
| duplicate variant names rejected package-wide | ✅ |
| GoName collision check | ✅ |
| package-level `const`, untyped | ✅ |
| typed `var`, bare `nil`, `nil[T]` escape hatch | ✅ |
| top-level forward references, direct init cycles | ✅ |
| init cycle through a free function call | ◐ only full `ink check` reports it |
| `T[]`, `T[N]`, `map[K, V]`, `*T`, `fn(A) R` | ✅ |
| user-declared variadic functions | — Go variadics are callable with `...` |
| tuples as values | — multiple results only |

## C. Values and control flow

| Feature | Status |
|---|---|
| struct literals with named fields (`field: value`), zero values | ✅ |
| map literals `key: value` | ✅ |
| `&x` for variables, fields, literals; `*p` read/write | ✅ |
| slicing, shared storage | ✅ |
| `s[i]` on a string is a `byte`; `runes(s)` for code points | ✅ |
| `len`, `cap`, `append`, `copy`, `delete`, `has`, `min`, `max` | ✅ |
| `if`, all `for` forms, `match` statement/expression | ✅ |
| `break`/`continue`, including inside a `match` case | ✅ |
| user labels, `goto` | — |
| `switch` | — use `if` or `match` |
| `void` as a value | — result only |
| non-void function that can fall off its end | ✅ `missing-return` |
| `missing-return` inside a function literal | ◐ not checked; runtime `panic: unreachable` |
| `T!` with `or fail`, `or err { }`, `or panic(...)` | ✅ |
| `fail(err, "context")` wraps and returns; `fail` is a reserved word | ✅ |
| manual `if err != nil { return err; }` | — rejected; `or fail` is the canonical form |
| expression-level `or`, `or` in multi-RHS or a `for` post | — statement-level only |
| block-scoped `defer` | ✅ |
| local `const` | — package level only |
| `:=` | — `var` only |
| doc comments attached to declarations | — comments are formatter input |

## D. Concurrency

| Feature | Status |
|---|---|
| `par { ... }` — one worker per top-level statement | ✅ |
| `par for (...) limit N` — one worker per iteration | ✅ |
| `within D { ... }` | ✅ |
| panic in a worker becomes an error, cancels siblings | ✅ |
| deterministic error order, lowest ordinal wins | ✅ |
| per-worker `defer` and `within` | ✅ |
| static captured read/write sets | ✅ |
| map writes and map ranges rejected | ✅ |
| `for ()` rejected inside `par` | ✅ |
| C-style `par for` captures a per-iteration copy | ✅ |
| hidden context through direct calls and methods | ✅ |
| hidden context across a package boundary or function value | ◐ resets to `context.Background()` |
| static race detection | — `ink run -race` / `ink test -race` is the oracle |
| `chan`, `go`, `select` | — keep them in a Go adapter |

## E. Go boundary

| Feature | Status |
|---|---|
| call Go packages with `use` | ✅ |
| multiple results, including `(T, bool)` via two variables | ✅ |
| Go interfaces implemented by Ink structs | ✅ |
| Go callbacks passed to Go code | ✅ |
| Go functions taking `context.Context` first | ✅ hidden context inserted |
| explicit type arguments on Go generic calls | ✅ |
| implicit Go generic inference | ◐ Go infers; Ink hover does not show the result type |
| Go variadic calls with `...` | ✅ |
| unexported Go API | — Go visibility rules |
| struct tags for JSON/DB | ○ decision needed |
| `//go:embed` | ○ decision needed |
| `context.Context` in a public Ink signature | — hidden only |

## F. Tooling

| Feature | Status |
|---|---|
| `ink build`, `run`, `test`, `check`, `fmt`, `lsp`, `new`, `init`, `guide` | ✅ |
| `check --json` with codes, hints and end positions | ✅ |
| `check --fast` (parse, resolve, sema) | ✅ |
| `check --race` (type-check with `-race`) | ✅ |
| generated-Go typecheck in `check` and LSP | ✅ |
| per-package typecheck cache | ○ |
| exact end spans in sema/typecheck diagnostics | ◐ one-character fallback |
| LSP diagnostics, hover, definition, completion, symbols, signature help | ✅ |
| LSP references and rename | ◐ project declarations only; no conflict check |
| LSP code actions and quick fixes | ✅ |
| formatter, idempotent, trailing comments | ✅ |
| formatter line wrapping | ○ |
| unused locals / vet-style diagnostics as an Ink pass | ◐ comes from the Go typecheck |
| `ink run -race`, `ink test -race` | ✅ |

## G. Agent contract

| Feature | Status |
|---|---|
| `ink guide` printed by the binary | ✅ |
| short `AGENTS.md` installed by `ink new` / `ink init` | ✅ |
| `ink` OpenCode skill installed by `ink new` | ✅ |
| `opencode.json` with LSP and format-on-edit | ✅ |
| stable diagnostic codes and hints | ✅ |
| project tests checked by `ink check` | ✅ |

## H. Deliberately absent

The canonical list is [`SPEC.md`](SPEC.md) §10. In short: local named
functions, generic constraints, generic enums, guards and struct patterns,
lambda parameter inference without an expected type, expression-level `or`,
user variadics, `discard`, doc comments, and the Go-isms `go`, `chan`,
`select`, `async`, `await`, `try`, `catch`, `?`, `export`, `let`, `:=`,
attributes, traits, lifetimes, arenas, `unsafe`, `switch`, labeled control flow,
operator overloading, exceptions.
