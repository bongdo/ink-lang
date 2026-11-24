# Ink agent guide

Ink is a Go-backed application language. Do not invent syntax: the full
reference, canonical forms, diagnostic codes and concurrency limits are in
`ink guide`; in OpenCode load the `ink` skill instead.

`ink guide` is the only reference: it is printed by the binary and this file is
its short form. `docs/` exists only in the compiler repository; a project
created by `ink new` has no `docs/` or `examples/`, so do not look there.

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
`use ./lib;`. Generated and never edited: `zz_ink.go`,
`zz_ink_test.go`, `.ink/`.

## Tests

`test "name" { assert_eq(got, want); };` in any `.ink` file; run `ink test`.
A `Test*` function is not a test. A failed assertion fails only its own test.
`str(65)` is the rune `"A"`, not `"65"`: format numbers with `fmt.Sprint`.
Every block keeps parentheses and ends with `};`. An import's local name is
the last path element; alias it when the Go package name differs, and use
`use pkg as _;` for a blank import.

OpenCode starts `ink lsp` for `.ink` and formats on edit when configured.
In the compiler repository, build `bin/ink` first (`bin/build`); installed
projects use `ink` from `PATH`.
