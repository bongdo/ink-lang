# Blind tests

An agent that has never seen Ink solves a normal task using only `ink guide`.
The goal is not the artifact but the friction log: where it guessed, what the
diagnostic said, and what would have saved a cycle.

## Rules

- The only reference is `$INK guide` (or the project's `ink` skill) and
  `$INK --help`. No compiler source, no `docs/`, no `examples/`, no web search,
  no `explore` agent.
- `ink` is `/home/ubuntu/Projects/Ink/bin/ink` until it is on `PATH`.
- Copy the task out of this repository first: a blind agent must not be able to
  `ls` its way into `internal/`.
- Every task ends with `REPORT.md`: result, cycles, friction table with the
  exact diagnostic, guide gaps as pasteable sentences, suspected compiler bugs
  with a minimal repro, and a 1-10 verdict.
- Do not fix the compiler during the blind run. Fixes happen after triage, so
  the next round measures something new.

## Tasks

| dir | task | acceptance |
|---|---|---|
| `web` | JSON HTTP service, stdlib, graceful shutdown, httptest tests | `check` clean, tests green, curl transcript |
| `snake` | raylib Snake, pure-logic tests | compiles headless with `GOFLAGS=-tags=nocgo`, tests green |
| `bugs` | five planted logic bugs in an inventory data layer | `check` clean, tests green, CLI transcript |
| `sqlite` | SQLite ledger CLI with transactions and rollback | `check` clean, tests green, CLI transcript |
| `port` | port a concurrent Go word-frequency CLI | byte-identical output vs the Go binary |
| `edit-web` | extend an existing HTTP service: filter, replace, stats | `check` clean, old tests untouched and green, new tests |
| `edit-store` | add `transfer` to an existing CLI + library | invariants tested, old tests untouched and green |

The `edit-*` tasks are the realistic case: the agent reads an existing Ink
project first and adds a feature. Score the edit experience, not from-scratch
creation — most agent work is editing, not bootstrapping.

`bugs` is deliberately red: `ink test` fails until the five bugs are fixed.
`port` contains a nested Go module with the reference implementation.

## Round 1 — 2026-11-23

Five parallel agents, one per task, 6-7 tool calls per minute, roughly
10-30 minutes each depending on the task.

| task | score | cycles before clean check | biggest time sink |
|---|---|---|---|
| web | 7/10 | ~20 | test syntax: `Test*` functions and `*_test.ink` silently produce `[no test files]` |
| snake | 7/10 | ~9 | test syntax; `int` vs `i32`; raylib cgo headers with no hint |
| bugs | 7/10 | 0 (check already clean) | one `assert_eq` panic stopped the whole test binary |
| sqlite | 6/10 | 3 | `str(n)` prints a rune; driver blank import undiscoverable |
| port | 7/10 | 8 | test syntax; a single missing block `;` produced 60 errors |

### Fixed after the round

- `ink guide`: control flow, block terminators, composite literals, `int`
  defaults, conversions and the `str(n)` rune trap, imports/aliases/blank
  imports, dependencies and `GOFLAGS`, a Tests section, reserved words, and the
  deliberately absent list.
- Project `AGENTS.md` and the generated `ink` skill now carry the test form,
  the `str(n)` trap and the alias/blank-import rule.
- A failed assertion or panic fails only its own test; the rest still report.
- `ink test` with no `test` blocks explains the syntax instead of only printing
  `[no test files]`.
- `ink run` forwards SIGINT/SIGTERM to the child and exits with its code.
- `use _ pkg;` points at `use pkg as _;`; an unused import hint mentions the
  blank import.
- `ctx.Err() != nil` no longer fails with `cannot name Go type error`; `error`
  is nameable.
- `ink run --help`, `ink check --help`, `ink fmt --help`, `ink lsp --help`
  print usage and exit 0.

### Open decisions

- `str(65)` is `"A"`. Keep Go semantics, or make numeric `str` a decimal
  conversion, or warn on it? Warning is the cheap middle ground.
- `defer` of a fallible call drops the error silently (for example
  `defer boom();` where `boom() i32!`). Reject it, panic, or keep the discard?
- No channels or `select` by design; the guide now says so, but a blocking wait
  still needs a Go adapter or polling.

## Running a round

```sh
cp -r tests/blind/web /tmp/blind-web
```

Then give the agent `TASK.md` and the rules above. Keep `REPORT.md` with the
artifact; triage reports into guide text, diagnostics, or a decision note.

## Round 2 — 2026-11-24

Three tasks (`web`, `bugs`, `sqlite`; `snake` needs raylib headers, `port` a
nested module). Blind agents ran in `/tmp/opencode/blind-*`, guide only.

| task | score | cycles before clean | biggest time sink |
|---|---|---|---|
| bugs | 9/10 | 1 test run → 1 fix pass | assertion prefix named a neighbouring line; success printed no test count |
| web | 7/10 | 7 | methods at file scope: 69 cascading errors, none pointing at the cause |
| sqlite | 7/10 | 8 | the same: whole file rewritten after the receiver error |

### Fixed after the round

- A file-scope method is one diagnostic naming the type
  (`method-outside-type`); the body is skipped instead of cascading.
- A sibling method called without `self` says so: `method Add is called on its
  receiver: self.Add(...)`; `or` no longer adds a second error on top.
- A dropped `fmt.Println` result names `_ = ...` as the discard; the guide says
  Go functions keep their Go signatures.
- `ink test` prints `tests: N passed`; an assertion prefix points at the test,
  the message still carries the exact assertion line.
- Guide: `&&`, `||`, `!` in conditions; methods live in the body; `self.Method`
  for siblings; `_ =` for ignorable errors.
- `http.Server` copy-lock gets `copy-lock` with "use a pointer".
- `ink build` outside a project says "not an Ink project", not
  `open /boot/efi: permission denied`.
- `ink new` on a non-empty directory points at `ink init`.

### Open after the round

- `or` inside an expression: one clear error plus a few `expected )` cascades.
- The SQL driver name (`modernc.org/sqlite`) is not discoverable from the
  guide; the agent guessed it from Go knowledge.

## Round 3 — 2026-11-24, editing existing code

Brownfield tasks (`edit-web`, `edit-store`): the agent reads a working project
and adds a feature without rewriting it. This is the common agent case.

| task | first run | after fixes | cycles |
|---|---|---|---|
| edit-store (CLI + lib) | 9/10 | 9/10 | 2, zero code fixes |
| edit-web (HTTP service) | 8/10 | 9/10 | 1, zero code fixes |
| sqlite (from scratch, retest) | 7 → 8 → 8 | 9 | 3 |

### Fixed from round 3

- A file inside the test that contradicted the task (PUT → 405) was a task
  bug, not a language bug: the baseline now uses `TRACE`.
- `var x: T;` says `var needs a value` with `var-value` and a hint.
- `_ =` rules documented: values only; `void` calls are statements.
- JSON key order follows the struct, not the `wire` block, and is documented.
- `q.Get("k")` vs `q["k"]`: absent and empty are different, documented.
- SQLite: driver name, transactions, `ErrNoRows`, `defer tx.Rollback()`, raw
  strings — all documented.
- `ink test -run ...` no longer claims `tests: N passed` when the filter
  selected nothing.
- `return` inside an inline `on err` in a fallible function warns
  `err-return`: the caller gets a value and a nil error.
- A reserved word as a parameter or name is one `reserved` diagnostic.
- `ink -C dir <command>` works like `go -C`.
- `ink check` refreshes `zz_ink.go` when it already exists, so an on-disk
  generated file cannot go stale behind the agent's back.
- Generated names never leak into diagnostics: `_s_v8` in a Go message is
  rewritten to `value`.
- A reserved word as a variable or in an expression is a `reserved` error, not
  a cascade; the guide says a test uses its own package's names and must not
  import its own directory.

### Still open

- `ink test -run <filter>` with no match exits 0 (same as `go test`); the
  misleading count line is gone, but the exit code is Go's.
- The LSP experience is not part of the score: blind agents ran without it.
- `snake` and `port` remain unrun (raylib headers, nested Go module).

## Round 4 — 2026-11-24, same edit tasks with MCP

Headless `opencode run` in a copy of each task, `opencode.json` with `ink lsp`
and `ink mcp` (`codemode: false`). The agent could use shell and the eight
`ink_*` tools; the logs count both.

| task | score | cycles | what worked |
|---|---|---|---|
| edit-web | 9/10 | 1, no diagnostics | `ink_complete` listed the new `Store.Replace` next to existing methods; `ink_symbols` fixed the naming style; `ink_references` found the only call site |
| edit-store | 7/10 → 8/10 (two retests) | 1 (`int` vs `i32`) | `ink_definition`, `ink_references` (all 6 call sites) and `ink_hover` found `Transfer` across files |

### Fixed from round 4

- `ink_symbols` listed only top-level declarations; methods are now children of
  their type (`Store` → `Add`, `Receive`, `Transfer`, …).
- `ink_quickfix` said "no quick fixes" when a diagnostic had no mechanical fix;
  it now returns `no automatic fix here; hint: …` so the agent gets the hint.
- `ink_hover` could pick a same-named symbol from another part of the line via
  the by-name fallback; it is used only when the word occurs once on the line.
- `ink_diagnose` caches by file fingerprint and accepts `fast: true`.
- Hover on a literal shows its type (`str literal`, `int literal`, `f64`).
- Guide: `self` is a pointer (a method mutates the caller's fields, maps and
  slices are shared), and a fallible method returns `void!`, not `void`.
- `ink build` and `ink test` typecheck before Go, so a type error shows the
  Ink code and hint instead of a Go build failure.
- Text diagnostics print `hint: …` under the message, not only in `--json`.

### Measured tool mix (web / store)

- web: `ink_symbols` 1, `ink_diagnose` 1, `ink_references` 2, `ink_hover` 3,
  `ink_complete` 1; shell 19, of them 12 `ink …`.
- store: `ink_quickfix` 1, `ink_symbols` 1, `ink_hover` 2, `ink_definition` 2,
  `ink_references` 1, `ink_rename` 1, `ink_diagnose` 1, `ink_complete` 1;
  shell 10, of them 8 `ink …`.
