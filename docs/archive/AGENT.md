> **Archived.** Frozen design note, kept for history. Not current documentation.
> Current: [SPEC.md](../SPEC.md), [FEATURES.md](../FEATURES.md), [EDGES.md](../EDGES.md).

# Ink — agent readiness roadmap

**Status:** implemented for the 0.2 preview; A2 and A7 are partial.
**Why:** an AI agent must be able to work from the repository and the `ink`
binary alone: read the canonical syntax, write code, get precise diagnostics,
fix them, format, test and run — without guessing or reading historical design
notes.

The 1.0 surface is not the bottleneck. The bottleneck is the feedback loop.
Agent readiness is mostly tooling and documentation, so it can be finished
without freezing the language.

## P0 — agent contract and feedback

These are prerequisites. Nothing in P1–P3 starts before P0 is usable.

| ID | Deliverable | Status | Evidence |
|---|---|---|---|
| A0 | Agent guide (`AGENTS.md` or `docs/agent-guide.md`) | done | Root `AGENTS.md` is a short ambient contract; the full reference is `internal/agent/guide.md`, printed by `ink guide` and exposed as the `ink` OpenCode skill. `ink new` / `ink init` install the short guide; `internal/agent/guide_test.go` enforces both files. |
| A1 | Machine-readable diagnostics | done | `ink check --json` returns `file`, `line`, `col`, `endLine`, `endCol`, `severity`, `code`, `message`, `hint`; clean output is `[]`, exit 1 on errors. `ink fmt -l --json` returns paths. Codes and hints live in `internal/diag`. |
| A2 | Precise positions | partial | Lexer and parser errors carry exact token ends; sema and typecheck spans are one-character fallbacks (`driver.spanEnd`). Codegen emits relative `//line path:line:col`, so type errors have real columns and no absolute project path; non-exact sema ends remain (G34). |
| A3 | Fast loop | done | `ink check --fast` stops after parse, resolve and sema. The LSP runs sema on `didChange` and debounces the generated-Go typecheck by 150 ms, dropping stale generations; full `check` includes `test` blocks. |
| A4 | Canonical oracle | done | `tests/run.sh` checks `examples/tour` (run, test, output) and `examples/problems` (positions, codes, messages); `tests/cases` covers the implemented surface, and a case with `expected-fast.txt` gates `ink check --json --fast` while `expected-check.txt` gates full `ink check --json` (non-zero exit plus position/code/message). Test cases run `ink test -v`, count declared `test` blocks against `--- PASS:` lines, and fail on `[no test files]` or a count mismatch. `tests/acceptance/agent.sh` walks `new` → fast JSON → `const-local`/`err-manual` → `fmt -l --json` → `test` (one `TestCounter` PASS) → `run` → cleanup, and `bin/check` runs it. |
| A5 | CLI contract | done | `cmd/ink/main.go` usage lists flags and exit codes 0/1/2; README and `ink guide` document `--json`, `--fast`, `--race`, and generated `zz_ink.go` / `.ink/`. Commands do not prompt. |

## P1 — agent editing

| ID | Deliverable | Status | Evidence |
|---|---|---|---|
| A6 | LSP refactors | done | `textDocument/references`, `prepareRename` and `rename` cover locals, parameters, globals, constants, functions, types (including imported local-package types), fields and enum variants; tests are in `internal/lsp/refactor_test.go`. Go library symbols and package aliases are excluded, and rename does not check conflicts. |
| A7 | LSP quick fixes | partial | Code actions exist for `const-local`, `decl-equals`, `iface-fn`, `import-simple`, `import-alias`, `err-manual` and `nil-untyped`. `nil-untyped` is conservative: it fires only when a typed `var`, return result or call parameter supplies the expected type. Rename has no conflict check. |
| A8 | LSP coverage | done | Diagnostics on `didChange` with a 150 ms debounced typecheck and stale-result drop; `Tests: true` includes test blocks; `didClose` publishes empty diagnostics for the buffer and re-reads disk; `workspace/symbol` is implemented. Covered by `internal/lsp/server_test.go` and `refactor_test.go`. |

## P2 — concurrency confidence

| ID | Deliverable | Status | Evidence |
|---|---|---|---|
| A9 | Race path | done | `ink run -race`, `ink test -race` and `ink check --race` pass through to Go; `check --race` sets `packages.Config.BuildFlags` to `-race`. Documented in README, getting started and `ink guide`. |
| A10 | Boundary docs | done | `ink guide` names G10 and G11; [`examples/limits/README.md`](../../examples/limits/README.md) has annotated G10/G11 snippets; `examples/tour` and `examples/problems` are named as the valid/invalid oracles. |

## P3 — 1.0 polish

Only after P0–P2:

- G32–G38 codegen/runtime cleanup.
- Remaining diagnostics and formatter work: G43, G44 and optional wrapping.
- Performance, compatibility policy and the 1.0 freeze.

## Definition of done for agent readiness

A fresh agent, given only the repository and a small task, can:

1. find the canonical syntax and commands from one guide;
2. create a project and write code without using rejected syntax;
3. run `ink check --json`, get 1-based positions, codes and hints, and fix
   every diagnostic (sema and typecheck end columns are one-character spans);
4. run `ink fmt -w`, `ink test` and `ink run`;
5. use LSP references/rename for a small refactor within project declarations;
6. run `ink run -race` where concurrency is involved;
7. finish without reading `design/SYNTAX.md` or `design/NIL.md` as current
   documents.

## Not in this roadmap

- A Ink stdlib: Go stdlib is the stdlib.
- Generic constraints, generic enums, iterators.
- Full IDE features beyond A6–A8.
- Dynamic race instrumentation: `ink run -race` / `ink test -race` remain
  the oracle.
