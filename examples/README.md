# Syntax examples

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
fallible results (`or return`, `or err`, `or panic`), block-scoped `defer`,
`if`/`for`/`match`, `any` type patterns, negative and `nil` patterns, typed vars
and bare `nil`, `par`, `par for ... limit`, `within`, builtins, Go interop and
`test` blocks.

## `problems/` — intentionally invalid

`problems/main.ink` is a catalogue of known 0.2 limits. `ink check` fails
with diagnostics for each function; `problems/expected.txt` pins their
positions, codes and message prefixes. Use it with the LSP to see the exact
positions; `tests/run.sh` runs it as an intentional-failure oracle.

## `limits/` — concurrency boundaries

`limits/README.md` explains G10 (hidden context stops at package and
function-value boundaries) and G11 (static `par` checks cannot see every
alias). `ink run -race` / `ink test -race` are the dynamic oracle.

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
- Neovim setup is in `docs/lsp.md`.
