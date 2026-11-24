# Blind task: extend an existing service

This is a working Ink HTTP service. Read `main.ink` first: types, `dispatch`,
`Store`, helpers and the tests show the style. Keep that style; do not rewrite
the file and do not change existing tests.

## Task

1. `GET /items?done=true|false` filters the list. Any other value for `done`
   is a client error (400) with an error body.
2. `PUT /items/{id}` with `{"title":"...","done":true}` replaces the item
   completely: 200 with the item, 404 when missing, 400 on malformed JSON or an
   empty title.
3. `GET /stats` returns `{"total":N,"done":N,"open":N}`.
4. Every existing behaviour stays: the existing tests must pass untouched.

## Acceptance

1. `/home/ubuntu/Projects/Ink/bin/ink check --json` prints `[]`.
2. `ink test` is green, including new tests for the filter, the replace and the
   stats endpoint (use the existing `call`/`decodeItem` helpers).
3. `ink fmt -l --json` prints `[]`.
4. `ink run` plus `curl` for the three new endpoints; include the exact output.
5. `REPORT.md` here: result, cycles, friction with exact diagnostics, guide
   gaps, verdict 1-10.
