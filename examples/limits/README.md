# Concurrency limits

This program compiles and passes `ink check`. It shows why the static checks
are not the whole story. `ink check` is the source of truth for compile-time
diagnostics; `ink run -race` and `ink test -race` are the runtime oracle for
data races.

## Static `go` checks cannot see every write

```hare
fn store(p: *i32, v: i32) void {
    *p = v;
};

fn main() void {
    var n = i32(0);
    par {
        store(&n, 1);
    };
};
```

Sema rejects direct captured writes, map writes, pointer writes and direct
aliases (`var q = p`, `q = p`). It does not track a pointer passed to a free or
Go function, aliases from a field/index read or a call result, or writes inside
function literals; a method call on a captured interface value is conservatively
treated as a write. This snippet has one worker, so it is race-free. Add a
second worker that reads `n` and neither `ink check` nor `ink check
--race` will report the race; only the runtime detector does:

```sh
ink run -race
ink test -race
```

`ink check --race` type-checks with `-race`, but it does not execute the
program. See `ink guide` §Concurrency, [`docs/product/SPEC.md`](../../docs/product/SPEC.md) §8 and
[`docs/product/EDGES.md`](../../docs/product/EDGES.md).
