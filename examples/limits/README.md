# Concurrency limits

These two programs compile and pass `ink check`. They show why the static
checks are not the whole story. `ink check` is the source of truth for
compile-time diagnostics; `ink run -race` and `ink test -race` are the
runtime oracle for data races.

## Hidden context stops at package and function-value boundaries

`lib/lib.ink`:

```hare
fn Work() i32 {
    within 10s {
        return 1;
    };
    return 0;
};
```

`main.ink`:

```hare
use fmt;
use ./lib;

fn main() void! {
    within 1ms {
        var n = lib.Work();
        fmt.Println(n)
            or fail;
    };
};
```

`within 1ms` applies to the caller's context. `lib.Work` is called across a
package boundary through its public wrapper, which starts from
`context.Background()`, so its own `within 10s` is not shortened by the caller.
The same loss happens through a function value, interface dispatch and
package-level initializers. Inside one package, direct calls and methods keep
the hidden context.

## Static `par` checks cannot see every write

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
