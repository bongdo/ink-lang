# Blind task: fix logic bugs in an existing database layer

You are testing Ink as a language for an AI agent, blind: you have never used
Ink before and must learn it only from `ink guide`.

## Setup

This directory is a template. Copy it to a scratch location outside the Ink
repository and work only in the copy, for example:

```sh
cp -r tests/blind/bugs /tmp/blind-bugs && cd /tmp/blind-bugs
```

## Hard rules

- `ink` is not on PATH. Use `/home/ubuntu/Projects/Ink/bin/ink` (call it `INK`).
- You may run `$INK guide` and read `$INK --help`. You must NOT read, list or
  grep anything else under `/home/ubuntu/Projects/Ink`.
- Do not edit files under `/home/ubuntu/Projects/Ink`.
- Do not use the `explore` agent or web search.
- Record friction **while working**, not from memory at the end.

## Situation

A colleague left this inventory service:

- `lib/store.ink` — store, orders, JSON persistence.
- `main.ink` — CLI against `$STORE` (`store.json` by default), which loads the
  file on start and saves after every mutation.
- `store_test.ink` — the acceptance tests. They are red.

`ink check --json` is clean; the logic is wrong. There are five independent
bugs. One of them makes `ink run -- top 2` panic on an empty store.

## Task

1. Find and fix all five logic bugs. Root causes only: no test changes, no
   weakening assertions, no dead code around the bugs.
2. Keep `$INK check --json` at `[]` and make `$INK test` fully green.
3. Prove the CLI works after the fix with one `$STORE`: `add`, `receive`,
   `value`, `top`, then a second process reads the same state. Quote exact
   output.
4. In the report, list each bug: symptom, root cause, why it was hard or easy
   to see, and which Ink rule (if any) would have prevented it.

Do not add new features, files or dependencies beyond what the fix needs.

## REPORT.md (required, in this directory)

```md
# Database bugs — blind report

## Bugs
For each: symptom / root cause / line / how many cycles to find.

## Result
check/test/CLI outputs.

## Cycles
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
Score 1-10: how well could an agent debug this? Biggest time sink?
```

Budget: under 45 minutes and 60 tool calls. Never invent syntax; if a language
rule blocks you after 3 honest attempts, report it as a gap.
