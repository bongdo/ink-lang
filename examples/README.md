# Syntax examples

## `small/` — every construct, one file

`small/main.ink` is the dense tour: imports (Go, local), `const`/`var`, aliases,
struct with methods and `wire`/named contract views, enum and exhaustive
`match`, interface, generics, multiple and fallible results with `or fail`,
inline `on err` and the scope listener, block-scoped `defer`, all loop forms,
builtins, pointers, `any`/literal/`nil` patterns, `go { }`, `go for ... limit`,
detached `go f(args)`, `arena`, Go interop and `test` blocks. It runs without
extra dependencies:

```sh
cd examples/small
../../bin/ink check
../../bin/ink test
../../bin/ink run
```

`expected.txt` pins the output; `tests/run.sh` checks it.

## `tour/` — valid, runnable

`tour/main.ink` is the full syntax tour. It compiles and runs:

```sh
cd examples/tour
../../bin/ink check
../../bin/ink run
../../bin/ink test
pero main.ink        # open with the LSP and .ink highlighting
```

`tour/lib/lib.ink` is a local package used by the tour. It also demonstrates
`Point[lib.N]` (an imported constant as an array length).

Sections in the tour: imports, `const`/`var`, aliases, structs and implicit
`self`, enums and exhaustive `match`, interfaces, generics, multiple results,
fallible results (`or fail`, `on err`), block-scoped `defer`,
`if`/`for`/`match`, `any` type patterns, negative and `nil` patterns, typed vars
and bare `nil`, `go`, `go for ... limit`, `go f()`, builtins, Go interop and
`test` blocks.

## `frame/` — arena and `go for` with raylib

`frame/main.ink` rasterizes a frame into an arena-allocated row buffer, shades
rows in parallel, copies them into one persistent buffer and uploads it once per
frame. It needs `libraylib` at runtime; see `frame/README.md` for the build flags
and the measured fill times.

## `problems/` — intentionally invalid

`problems/main.ink` is a catalogue of known 0.2 limits. `ink check` fails
with diagnostics for each function; `problems/expected.txt` pins their
positions, codes and message prefixes. Use it with the LSP to see the exact
positions; `tests/run.sh` runs it as an intentional-failure oracle.

## `limits/` — concurrency boundaries

`limits/README.md` explains why the static `go` checks cannot see every write,
and why `ink run -race` / `ink test -race` are the dynamic oracle.

## Editor highlighting and LSP

Syntax highlighting is an editor feature; LSP itself provides diagnostics,
hover, definition, completion, symbols, signature help, formatting,
references, rename and quick fixes. In this repo:

- `pero` (the Go editor) has an Ink chroma lexer for `.ink` and a
  `ink = "ink lsp"` default server entry. If `ink` is not on `PATH`,
  set it in `~/.config/pero/config.toml`:

  ```toml
  [lsp]
  servers = { ink = "/home/bongdo/Projects/ink/bin/ink lsp" }
  ```

- `pero-inked` (the Ink editor) already highlights `.ink` and starts
  `ink lsp` from `PATH`; `bin/run` sets `INK_LSP` to
  `../ink/bin/ink` automatically.
- VS Code uses `editors/vscode/syntaxes/ink.tmLanguage.json` plus the
  `ink lsp` client.
- Neovim setup is in `docs/guides/lsp.md`.
