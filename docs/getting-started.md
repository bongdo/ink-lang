# Getting started

## Create a project

```sh
ink new hello
cd hello
```

`ink new` writes `ink.toml`, `main.ink`, a short `AGENTS.md`,
`opencode.json`, `.opencode/skills/ink/SKILL.md`, `go.mod` and `.gitignore`,
and prints `created hello`. The short `AGENTS.md` is the ambient agent contract;
`ink guide` prints the full embedded reference. `opencode.json` starts
`ink lsp` for `.ink` and formats on edit; the skill carries the full
reference and loads on demand. To add the short guide to an existing directory
without overwriting an existing `AGENTS.md`:

```sh
ink init .
```

`ink init` refuses to run when `ink.toml` already exists.

`ink.toml`:

```toml
name = "hello"
module = "example.com/hello"
go = "1.27"
```

`main.ink`:

```hare
use fmt;

fn main() void! {
    fmt.Println("hello from hello")
        or fail;
};
```

## Run and build

```sh
ink run
ink run -race
ink build
./bin/hello
```

`ink build` performs three steps every time: parse and type-check the
Ink sources, generate Go next to them, and invoke `go build`. Go modules
are created and tidied automatically; external Go imports are added with
`go mod tidy`. `ink run -race` builds the same way with Go's race detector.

## Agent loop

For an agent, the shortest reliable loop is:

```sh
ink check --json --fast   # parse, resolve, sema
ink fmt -w .
ink check --json          # completion gate: generated-Go typecheck
ink test                  # add -race for concurrency
ink run [-- args]         # add -race for the race detector
```

`--fast` stops after parse, resolve and sema, so it is the quickest feedback.
Full `check --json` type-checks the generated Go, including `test` blocks. It
prints one array of `file`, `line`, `col`, `endLine`, `endCol`, `severity`,
`code`, `message` and `hint`; positions are 1-based, clean output is `[]`, and
errors exit 1. `ink fmt -l --json` prints an array of paths that would
change. `ink guide` (or the `ink` skill in OpenCode) has the full contract
and the canonical fix for each diagnostic code.

## Add a local package

`lib/math.ink`:

```hare
fn Square(x: i32) i32 {
    return x * x;
};
```

`main.ink`:

```hare
use fmt;
use ./lib;

fn main() void! {
    fmt.Println(lib.Square(7))
        or fail;
};
```

Only exported (capitalized) names are visible across packages, exactly as in
Go.

## Types and constants

```hare
type User struct {
    Name: str,
    age: u32,
};

type Shape enum {
    Circle { radius: f64 },
    Empty,
};

type Writer interface {
    Write(p: u8[]) int!;
};

type UserId = u64;

const MaxUsers = 100;

var p: *User = nil;
```

`const` is package-level only and has no type; a local `const` is an error, use
`var`. `var` may carry a type (`var p: *User = nil`), which gives bare `nil` its
expected type. Old declarations with `=` before a type or function body are
rejected; `ink fmt` formats canonical input.

## Errors

```hare
use os;

fn exists(path: str) bool! {
    var f = os.Open(path)
        or fail;
    defer f.Close();
    return true;
};
```

- `call() or fail;` propagates.
- `call() or fail("context");` wraps and propagates.
- `call() or err { ... };` handles the error here; the name is always `err`.
- `call() or panic("message");` aborts.
- The handler always starts on a new line, indented once.

## Tests

```hare
test "square" {
    assert_eq(2 * 2, 4);
    assert(true);
};
```

`ink test` runs every `test` block with `go test`. Extra arguments are
passed through, so `ink test -race` runs the suite under Go's race detector.

## Concurrency

```hare
fn load(id: i32) i32! { ... };

fn main() void! {
    var users = 0;
    var orders = 0;
    par {
        users = load(1)
            or fail;
        orders = load(2)
            or fail;
    };

    var xs = i32[] {1, 2, 3};
    var out = i32[3] {};
    par for (var i = 0; i < len(xs); i += 1) limit 2 {
        out[i] = xs[i] * 2;
    };

    within 30s {
        out[0] = load(3)
            or fail;
    };
};
```

Captured state may be read concurrently. Writes must go to a slot indexed by
the iteration variable or be the only writer of that variable inside `par`.
`par`/`within` errors cancel the remaining work. The checks are static;
`ink run -race` and `ink test -race` are the dynamic oracle. Hidden
context stops at package and function-value boundaries, and aliases or
escapes the static pass cannot see are covered only by `-race`; see
[`examples/limits`](../examples/limits/README.md).

## Formatting

```sh
ink fmt -w .
ink fmt -l .          # list files that would change
ink fmt -l --json .   # same, as a JSON array
```

Four-space indentation, one statement per line, mandatory semicolons.
