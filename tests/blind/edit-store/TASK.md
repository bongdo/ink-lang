# Blind task: add a feature to an existing CLI and library

This is a working Ink inventory CLI (`main.ink`) over a library
(`lib/store.ink`) with tests (`store_test.ink`). Read all three first: types,
error paths, the `usage()` text and the test style. Keep them; do not rewrite
the files and do not change existing tests.

## Task

Add a `transfer FROM TO QTY` command:

- both SKUs must exist, `QTY` must be positive, and `FROM` equal to `TO` is an
  error;
- the total quantity across all items must not change;
- errors use the existing error path: the CLI prints the message and exits 1;
- `usage()` lists the new command;
- tests: success conserves totals, unknown SKU fails, non-positive quantity
  fails, same SKU fails.

## Acceptance

1. `/home/ubuntu/Projects/Ink/bin/ink check --json` prints `[]`.
2. `ink test` is green and the existing tests are untouched.
3. `ink fmt -l --json` prints `[]`.
4. `ink run -- ...` transcript: one successful transfer and each failure case.
5. `REPORT.md` here: result, cycles, friction with exact diagnostics, guide
   gaps, verdict 1-10.
