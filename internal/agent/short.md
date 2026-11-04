# Ink agent guide

Ink is a Go-backed application language. Do not invent syntax: the full
reference, canonical forms, diagnostic codes and concurrency limits are in
`ink guide`; in OpenCode load the `ink` skill instead.

Source of truth: `design/SPEC.md` (wins); `examples/tour` is valid;
`examples/problems` is invalid on purpose. Historical only: `design/SYNTAX.md`,
`design/NIL.md`.

## Loop

```sh
ink check --json --fast   # parse, resolve, sema
ink fmt -w .
ink check --json          # completion gate: generated-Go typecheck
ink test                  # -race for concurrency
ink run [-- args]         # -race for the race detector
```

`check --json` prints one array of `file`, `line`, `col`, `endLine`, `endCol`,
`severity`, `code`, `message`, `hint`; positions are 1-based, `[]` is clean, and
errors exit 1. `fmt -l --json` prints an array of paths.

## Layout

One directory is one package; no package clause. Local imports:
`use "./lib";`. Generated and never edited: `zz_ink.go`,
`zz_ink_test.go`, `.ink/`.

OpenCode starts `ink lsp` for `.ink` and formats on edit when configured
(see `docs/lsp.md`). Build `bin/ink` first in this repository (`bin/build`);
installed projects use `ink` from `PATH`.
