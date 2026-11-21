# Test cases pass without running any test

## Problem

The conformance harness treats `ink test` exit code 0 as "the test blocks
passed". `go test` also exits 0 when the generated `zz_ink_test.go` contains no
tests at all, and says so:

```
$ ink test
?   	example.com/tests	[no test files]
$ echo $?
0
```

`tests/run.sh` only diffs the output when a case has `expected-test.txt`, and
the single case with a `test` file (`tests/cases/tests`) has none. So the
branch that is supposed to cover `test` blocks passes if they are never
generated. `tests/acceptance/agent.sh` has the same hole: it writes a
`test "counter"` block, runs `ink test`, and checks nothing but the exit code.

A regression in `pkgHasTests`/test generation, in the `//line` mapping of test
bodies, or in an empty test body would keep `bin/check` green.

## Smallest change

In the `test` branch of `tests/run.sh`:

- run `ink test -v` (the non-verbose output has no per-test lines);
- count declared tests with `grep -h '^test "' <case>/**/*.ink`;
- count executed tests with `grep -c '^--- PASS:'`;
- fail unless the two counts are equal and greater than zero, and fail on
  `[no test files]`.

The existing `expected-test.txt` diff keeps working: its regex
`^(ok|FAIL|---)` still matches the `-v` output, and no case ships that file
today.

In `tests/acceptance/agent.sh`, run `ink test -v` and require exactly one
`--- PASS:` line for the counter test, so the end-to-end gate also proves a
test executed.

No Go code and no generated-file changes.

## Acceptance check

- `bin/check` is green, and `tests/cases/tests` reports `2 declared, 2 passed`.
- Negative probe: copy `tests/cases/tests/main.ink`, remove both `test`
  blocks, run the same two commands — declared 0, passed 0, and the new guard
  fails the case (documented in the task, since the harness hardcodes its
  root).
- A failing test still fails: change `assert_eq(add(2, 2), 4)` to `5` in the
  copy; `ink test` exits 1 and the harness reports FAIL as before.
- `tests/acceptance/agent.sh` fails if `test "counter"` is deleted from its
  fixture, where it passed before.

## Risks

- `go test -v` output is a Go contract, but test names come from
  `testGoID`; a future rename of the generated `TestX` shape changes only the
  prefix check, not the count.
- A case that passes `-run` or other filters through `ink test` would make the
  counts disagree. No case does that today; if it appears, it needs its own
  expected count in the fixture.
- Counting `test "` lines assumes the canonical declaration spelling enforced
  by `ink fmt`; a non-canonical fixture would undercount. The cases are
  formatted by `bin/check`, so this is bounded.
