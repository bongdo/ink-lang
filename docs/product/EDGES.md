# Ink — edges

Every place where the current language can surprise a user. Ranked by how
likely it is to bite, not by effort. No issue numbers: an entry either lives
here or is a feature in [`FEATURES.md`](FEATURES.md).

**open** — needs a fix or an explicit decision · **accepted** — we keep it, do
not re-file.

## Open

| # | Symptom | Today | Workaround |
|---|---|---|---|
| 1 | A `within` deadline silently does not apply across a package boundary, an Ink interface, a function value or a package initializer. | The public wrapper starts from `context.Background()`. | Keep the deadline and the call in one package; pass the deadline down explicitly. Needs a decision: exported ctx method vs function-value representation. |
| 2 | A `defer` written at the top level of a `par` body cleans up its own worker, not its siblings. | Every top-level statement is a worker. | Wrap the related statements in a block: `par { { defer f(); ... }; };`. Candidate for a rule change or a diagnostic. |
| 3 | `ink check` is clean, then the program panics with `panic: unreachable`. | `missing-return` is syntactic and not checked inside function literals. | Keep the literal's every path ending in `return`; the runtime message points at the literal. |
| 4 | JSON output has `{"Name": ...}` and small-letter fields disappear. | No struct tags; `encoding/json` uses Go field names and visibility. | Return `map[str, any]` or map fields in a Go binding. Needs a decision. |
| 5 | `//go:embed` does not exist; templates and static files are read from disk. | No attributes. | `os.DirFS` or a Go binding. Needs a decision. |
| 6 | `ink check` misses an initialization cycle that goes through a free function call. | The cycle graph does not traverse calls. | Full `ink check` (without `--fast`) still reports it through Go. |
| 7 | Conformance checking silently skips some signatures. | Generic types, enum variants, arrays, function values, variadic methods and multi-result calls are not compared. | `go build` is the backstop. Explicit Go `context.Context` parameters never match a hidden-ctx implementation. |
| 8 | Rename can produce a name conflict; references cover project declarations only. | No conflict analysis; Go library symbols are out of scope. | Run `ink check` after rename; use editor search for Go symbols. |
| 9 | Sema and typecheck diagnostics end one character after their start. | Parser and lexer ends are exact; Go diagnostics carry no end column. | Use the start position; end spans are cosmetic. |
| 10 | LSP waits for the current sema pass even after a newer edit. | A running analysis is not cancelled mid-pass. | None; the newer generation runs next. |
| 11 | `ink check` re-type-checks every package on each run. | No per-package cache. | `--fast` for rename-level feedback. |
| 12 | Hover on a Go generic call does not show the inferred result type. | Implicit type arguments are left to Go. | Write the explicit type arguments. |
| 13 | `within` only cancels calls that observe context. A blocking call that ignores it (`time.Sleep`, `os.Stat`, CPU work) runs to completion. | The deadline cancels the hidden context; it cannot stop a function that never reads it. | Put context-aware Go calls inside `within`; measure elapsed time instead of assuming the block was cut. |
| 14 | `ink check --fast` is clean, then `m[k].f = v`, `&m[k]` or a pointer method on `m[k]` fails in the Go typecheck. | Go does not address map elements, and Ink has no diagnostic of its own. | Copy out, mutate, write back: `var p = m[k]; p.f = v; m[k] = p;`. |
| 15 | Writing to a nil map panics at run time; `ink check` stays clean. | `nil` is a valid map value; only writes need storage. | Reads, `len`, `has`, `delete` and `for` are safe on nil. Start with `map[K, V] {}`. |
| 16 | `str(42)` is `"*"`, the rune, not `"42"`; no builtin formats a number. | `str` keeps Go's rune/byte conversion, and `rune` is an alias of `i32`. | Use `strconv.Itoa`, `strconv.FormatFloat` or `fmt.Sprint`. Needs a decision. |

## Accepted

| Symptom | Why it stays |
|---|---|
| `T or error` and `or return` are rejected. | `T!` and `or fail` replaced them; diagnostics point at the old form. |
| Arena values must not escape the `arena` block. | Freeing happens at block exit; using a value after is use-after-free, as in Go. Sema rejects the common escapes; a value stored by a callee or in a captured container may still leak. |
| Arena memory never crosses a function boundary. | A callee could keep the pointer and the block would free it; the check is syntactic, so helpers cannot take arena values. Copy data out with `copy`. |
| An arena value may be read as a field, an index or a value copy. | A struct or array copy is allowed only when its type holds no references; a copy of references would still point into the arena. |
| A worker writes arena elements indexed by its own iteration variable. | `par` proves disjoint slots through the loop variable; an index from another loop is rejected as a captured write. |
| `new` and `make` inside `par` inside `arena` are rejected. | The Go arena is not safe for concurrent use; allocate before `par` and let workers fill the memory. |
| A worker may open its own `arena { ... };`. | It creates a separate handle in that goroutine, so no shared arena state. |
| `fail` is a reserved word. | It is the error action; a user function or variable named `fail` must be renamed. |
| `new` and `make` are builtin names. | Go parity; a user function with either name is rejected and must be renamed. |
| The handler error is always named `err`. | One name; an outer `err` is shadowed inside the block. |
| Import paths cannot contain spaces or quotes. | Paths are bare; valid Go module paths fit the path charset. |
| Unsuffixed integer literals are Go `int`; `i32` is int32 and needs `i32(...)`. | Matches Go; `len`, `cap` and `os.FileInfo.Size` return Go integers. |
| `s[i]` is a `byte`, not a rune. | Matches Go; `runes(s)` is the documented form for code points. |
| Map writes are rejected in `par` even for disjoint keys, and `par for` cannot range over a map. | A map has no ordinal and no provable disjointness; collect keys, then iterate a slice. |
| Two writes to different fields of one captured struct are rejected. | One root, one writer: without alias analysis the compiler cannot prove they are disjoint. |
| A method call on a captured value struct or interface counts as a write. | The receiver may mutate; the checker is conservative. Copy the value locally when the call is pure. |
| A pointer passed to a free or Go function, or an alias from a field/index read or a call result, is not tracked as a write. | No interprocedural alias or escape analysis. `ink run -race` and `ink test -race` are the dynamic oracle. |
| `for ()` is rejected inside `par`. | A worker without a guaranteed exit can hang the block. |
| A C-style `par for (i = 0; ...)` makes the body read a per-iteration copy of `i`; after the loop `i` holds the post value. | Deterministic capture without a closure per iteration. |
| `or` is statement-level only: no `f(g() or fail)`, no `or` in multi-RHS or a `for` post. | No expression-level control flow; split the call into two statements. |
| A Go function returning `(T, bool)` needs two variables, not `or`. | `or` is for `error`; `ok` is a normal result. |
| A local `const` is an error. | `const` is a package-level compile-time name; use `var` locally. |
| Manual `if err != nil { return err; }` is rejected. | `or fail` is the one canonical propagation form. |
| `match` on `any` requires `else`. | Open types have no exhaustiveness; `else` makes the default explicit. |
| Enum variants cannot share a name in one package, and an enum's zero value is not constructible. | Bare `case` patterns stay unambiguous; construction is explicit. |
| `nil` needs an expected type or `nil[T]`. | `nil` is untyped; typed `var` covers most positions. |
| `chan`, `go`, `select` do not exist in app code. | Channels break the `par` model and the static write analysis. Keep them in a Go binding. |
| Methods always use a pointer receiver. | One receiver form; no value/pointer method-set split. |
| No doc comments attached to declarations. | Comments are formatter input only; `ink guide` is the reference. |
