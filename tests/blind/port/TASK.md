# Blind task: port a Go program to Ink

You are testing Ink as a language for an AI agent, blind: you have never used
Ink before and must learn it only from `ink guide`.

## Setup

This directory is a template. Copy it to a scratch location outside the Ink
repository and work only in the copy, for example:

```sh
cp -r tests/blind/port /tmp/blind-port && cd /tmp/blind-port
```

## Hard rules

- `ink` is not on PATH. Use `/home/ubuntu/Projects/Ink/bin/ink` (call it `INK`).
- You may run `$INK guide` and read `$INK --help`. You must NOT read, list or
  grep anything else under `/home/ubuntu/Projects/Ink` (compiler source,
  examples, docs). Reading the compiler repo invalidates the test.
- Do not edit files under `/home/ubuntu/Projects/Ink`. Do not use the
  `explore` agent or web search.
- Record friction **while working**, not from memory at the end.

## Task

`reference.go` is a working Go CLI: concurrent word frequency with `-n` (top
count) and `-min` (minimum count) flags, aligned output `COUNT<TAB>WORD`, errors
to stderr with exit 1. Build the reference binary and keep it for diffs:

```sh
go build -o wordfreq reference.go
```

`sample.txt` is the sample input.

Port it to Ink in this directory with the same observable behaviour:

- Same flags, same defaults (`-n 10`, `-min 1`), same output format, same
  sorting (count desc, then word asc), same word normalization (trim
  non-letter/non-digit except `'`, drop words under 2 runes, lowercase).
- Multiple file arguments, stdin when none.
- Keep the concurrency shape: one worker per input file, merge after join.
- Errors: missing file -> `wordfreq: open ...: no such file or directory` on
  stderr, exit 1.
- Add `ink test` tests for: normalization, the 2-rune cutoff, sort tie-break,
  min filter, n limit.

Acceptance:

1. `$INK check --json` prints `[]`.
2. `$INK test` green.
3. Output is byte-identical to the Go binary for:
   `-n 5 sample.txt`, `-n 3 -min 2 sample.txt`, `sample.txt sample.txt`
   (run both, paste `diff` results — must be empty).
4. `$INK fmt -l --json` prints `[]`.

Do not add dependencies. Do not copy the Go file into the Ink project.

## REPORT.md (required, in this directory)

```md
# Go -> Ink port — blind report

## Result
check/test/diff outputs.

## Cycles
- check runs before first `[]`:
- test runs before green:
- total tool calls (approx):
- wall time:

## Friction log
| # | what I tried | exact error / surprise (quote the message) | cost | what would have fixed it |
|---|---|---|---|---|

## Go -> Ink translation table
Go construct -> Ink construct, only the ones that needed thought.

## Guide gaps
Exact sentences missing from `ink guide`, ready to paste.

## Suspected Ink bugs
Minimal repro per suspicion, or "none".

## Verdict
Score 1-10: how mechanical was the port? Biggest time sink?
```

Budget: under 45 minutes and 60 tool calls. Never invent syntax; if blocked
after 3 honest attempts, stop that part and report it as a gap.
