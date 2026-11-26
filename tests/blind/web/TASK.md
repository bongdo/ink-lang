# Blind task: web service

You are testing Ink as a language for an AI agent, blind: you have never used
Ink before and must learn it only from `ink guide`.

## Setup

This directory is a template. Create a fresh project outside the Ink
repository and work only there, for example:

```sh
cd /tmp && /home/ubuntu/Projects/Ink/bin/ink new blind-web && cd blind-web
cp /path/to/tests/blind/web/TASK.md .
```

## Hard rules

- Work only in the copied project directory.
- `ink` is not on PATH. Use `/home/ubuntu/Projects/Ink/bin/ink` (call it `INK`).
- Do **not** read, list or grep anything under `/home/ubuntu/Projects/Ink`
  (compiler source, examples, docs). The only allowed reference is
  `$INK guide` output and `$INK --help`. Reading the compiler repo invalidates
  the test.
- Do not edit files under `/home/ubuntu/Projects/Ink`. Do not install system
  packages.
- Do not use the `explore` agent or web search. No copying an existing Ink HTTP
  server from anywhere.
- Record friction **while working**, not from memory at the end: every
  diagnostic code, every doc gap, every wrong guess, every cycle spent.

## Task

Build a small JSON HTTP service in Ink (stdlib only, no frameworks, no
external modules):

- In-memory todo store, safe for concurrent HTTP handlers. `use sync` is rejected; use the language's `serve` object.
- `GET /healthz` -> `{"ok":true}`.
- `GET /items` -> JSON array (empty array, not null, for an empty store).
- `POST /items` with `{"title":"..."}` -> 201, created item with `id`.
- `GET /items/{id}` -> item or 404.
- `PATCH /items/{id}` with `{"done":true}` -> updated item or 404.
- `DELETE /items/{id}` -> 204 or 404.
- Malformed JSON or empty title -> 400.
- Unknown path -> 404. Wrong method on a known path -> 405.
- Listens on `:8080` (port overridable by `PORT` env var).
- Graceful shutdown on SIGINT/SIGTERM with a 5s timeout.
- One structured log line per request with method, path, status, duration.

Acceptance:

1. `$INK check --json` prints `[]`.
2. `$INK test` is green with tests using `net/http/httptest` covering:
   create -> list -> get -> patch -> delete, 404, 400 on bad JSON, 405.
3. `$INK run` starts the server; verify with `curl` for every route and kill it
   with SIGINT, confirming clean shutdown. Include the exact curl output in the
   report.
4. `$INK fmt -l --json` prints `[]`.

## REPORT.md (required, in this directory)

Write in Russian or English, be blunt and specific.

```md
# Web service — blind report

## Result
What works, exact commands and outputs (check/test/curl/shutdown).

## Cycles
- check runs before first `[]`:
- test runs before green:
- total tool calls (approx):
- wall time:

## Friction log
| # | what I tried | exact error / surprise (quote the message) | cost | what would have fixed it |
|---|---|---|---|---|

## Guide gaps
Exact sentences that were missing from `ink guide`, phrased so they could be
pasted into it.

## Suspected Ink bugs
Minimal repro (a few lines of .ink) per suspicion, or "none".

## Verdict
Score 1-10: could a competent agent with no compiler knowledge ship this?
What was the single biggest time sink?
```

Do not pad the report. Do not paste whole source files into it.
Budget: aim for under 45 minutes and 60 tool calls. If a language rule blocks
you after 3 honest attempts, stop that part and report it as a gap instead of
inventing syntax.
