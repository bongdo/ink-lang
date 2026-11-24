# Blind task: SQLite-backed ledger CLI

You are testing Ink as a language for an AI agent, blind: you have never used
Ink before and must learn it only from `ink guide`.

## Setup

This directory is a template. Create a fresh project outside the Ink
repository and work only there, for example:

```sh
cd /tmp && /home/ubuntu/Projects/Ink/bin/ink new blind-sqlite && cd blind-sqlite
cp /path/to/tests/blind/sqlite/TASK.md .
```

## Hard rules

- Work only in the copied project directory.
- `ink` is not on PATH. Use `/home/ubuntu/Projects/Ink/bin/ink` (call it `INK`).
- Do **not** read, list or grep anything under `/home/ubuntu/Projects/Ink`
  (compiler source, examples, docs). The only allowed reference is
  `$INK guide` output and `$INK --help`.
- Do not edit files under `/home/ubuntu/Projects/Ink`. Do not use the
  `explore` agent or web search.
- Module downloads work; `modernc.org/sqlite` is in the module cache.
- Record friction **while working**, not from memory at the end.

## Task

Build a small ledger CLI backed by SQLite (file `ledger.db` in the working
directory):

```
ink run -- init
ink run -- open ACC_ID
ink run -- balance ACC_ID
ink run -- transfer FROM TO AMOUNT
ink run -- history ACC_ID [N]
```

Rules:

- `init` creates tables (accounts with cents stored as integers, entries for
  movements) and two demo accounts if the file is new; idempotent.
- `open` creates an account with balance 0 and prints its id.
- `transfer` moves integer cents: reject negative/zero amount, unknown
  account, insufficient funds; all checks happen inside one transaction;
  either both rows change or neither does.
- `balance` prints cents, `history` prints the last N entries (default 10),
  newest first, one line each.
- Bad usage prints a message to stderr and exits 1.
- Money is never float; use i64 cents.

Acceptance:

1. `$INK check --json` prints `[]`.
2. `$INK test` green with tests against a temp SQLite file covering: init is
   idempotent, transfer changes both balances, insufficient funds rolls back
   (balances unchanged), unknown account rejected, history order.
3. Run the full CLI sequence against a temp dir and paste exact output:
   init, open, balance, transfer, balance both sides, history, insufficient
   transfer rejected (non-zero exit), balances unchanged.
4. `$INK fmt -l --json` prints `[]`.

Note: plain `database/sql` uses a driver that must be registered. Figure out
the supported way in this project and describe exactly what you did in the
report. Keep the SQL access behind one small module so the rest of the app does
not see Go types.

## REPORT.md (required, in this directory)

```md
# SQLite ledger — blind report

## Result
check/test/CLI outputs.

## Driver registration
Exactly how SQLite got registered and what the guide said (or did not say).

## Cycles
- check runs before first `[]`:
- test runs before green:
- total tool calls (approx):
- wall time:

## Friction log
| # | what I tried | exact error / surprise (quote the message) | cost | what would have fixed it |
|---|---|---|---|---|

## Guide gaps
Exact sentences missing from `ink guide`, ready to paste.

## Suspected Ink bugs
Minimal repro per suspicion, or "none".

## Verdict
Score 1-10 for agent-friendliness of a real DB task, biggest time sink.
```

Budget: under 45 minutes and 60 tool calls. Never invent syntax; if blocked
after 3 honest attempts, stop that part and report it as a gap.
