# Missing-return is silently accepted

## Problem

`ink check --json` returns `[]` for a non-void function that can fall off its
end, because codegen appends `panic("unreachable")` to every function with a
result (`internal/gen/decl.go`, `funcBody`). Go's own missing-return rule never
fires, sema has no equivalent, and the agent learns about the bug only at
runtime, from a message that points at a generated position:

```
$ cat main.ink
use fmt;

fn f() i32 {
    fmt.Println("no return") or panic();
};

fn main() void or error {
    var n = f();
    fmt.Println(n) or return;
    return;
};

$ ink check --json
[]
$ ink run
no return
panic: unreachable
    ... /tmp/gocommand-.../main.ink:9
```

The same shape is accepted for `fn f() i32 { };` and for an `if` without
`else`. The reported line is past the end of the 7-line source, so even the
runtime message is not actionable. `docs/archive/GAPS.md` G32 covers the `go vet`
noise, but not the fact that `check` calls this program clean.

## Smallest change

One sema pass in `checker.checkFunction` (`internal/sema/check.go`): when the
function has at least one result (`len(c.fn.results) > 0`) and its body does not
provably terminate, report
`missing return in function returning T` at `fn.Body.End`.

Termination helper, kept to the constructs Ink already has:

- `return` and a bare `panic(...)` statement;
- a block whose last statement terminates;
- `if` with an `else` where both branches terminate;
- `match` where every case body terminates (exhaustiveness is already checked);
- `for` with no condition and no range, when its body has no `break`.

Add one `internal/diag` rule: match `missing return`, code `missing-return`,
hint `end every path with return;`.

`void` functions, error-only functions and `test` blocks are untouched: their
generated fall-through is `return nil` by design.

## Acceptance check

- A temp project with `fn f() i32 { };` gives exit 1 and exactly one JSON
  diagnostic with `code: "missing-return"`, a non-empty `hint`, and a position
  inside the file (the closing brace of `f`).
- `examples/problems` gains this program and `tests/run.sh` pins its
  `line:col:missing-return:missing return` line; the exact-count check also
  rejects a second copy of the diagnostic.
- `bin/check` stays green: `examples/tour`, `tests/cases` and `tests/smoke`
  must not gain a single new diagnostic. Any false positive is a bug in the
  helper, not in the fixture.
- `ink run` on the fall-through program never reaches `panic: unreachable`.

## Risks

- False positives are the whole risk. Functions ending in `match`, `if/else`,
  `for ()`, nested blocks and `panic(...)` are used in the tour; the helper has
  to accept all of them. Start with the list above and widen only with a
  failing fixture.
- An `if/else` whose branches terminate syntactically but fall through
  semantically stays a false negative; the Go typecheck still cannot see it.
  That is acceptable for the first patch and should be written down in
  `docs/archive/GAPS.md` next to G32.
- The check is syntactic, not flow-sensitive; `for ()` with a conditional
  `break` becomes a false positive unless the helper ignores any `break`.
