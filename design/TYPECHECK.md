# Type-checking generated Go (G26)

Status: works for `ink check` and LSP diagnostics. `ink check` also
type-checks generated test packages (`test` blocks). Sema remains synchronous in
the LSP; the generated-Go typecheck is debounced.

## What runs

`internal/typecheck.Check` takes the checked `sema.Program`, generates Go in
memory with `internal/gen` (same generator as the build), and loads the result
with `golang.org/x/tools/go/packages`. Nothing is written to the source tree:

- every generated `zz_ink.go` path is passed to `packages.Config.Overlay`;
- `ink check` also generates and overlays `zz_ink_test.go`, and sets
  `packages.Config.Tests` so the compiler checks the test variants;
- the embedded runtime is overlaid at `<module>/.ink/runtime/runtime.go`;
- the packages are loaded from the real module root, so `go.mod`, `go.sum`
  and the module cache resolve imports normally;
- `//line /abs/path/file.ink:line:col` directives emitted by codegen make the
  Go compiler report positions in `.ink` files, including the column;
  `typecheck` parses those positions and returns `Error{Path, Pos, Msg}`.

The load mode is `LoadImports | NeedExportFile`, not `LoadTypes`: requesting
`go/types` objects makes `go/packages` fall back to source-typing every
dependency because an overlay invalidates export data. Asking for export files
instead runs the Go compiler on the generated package (`go list -export`) and
returns its errors, which is the same type oracle and roughly 4-8x faster on
the projects measured here.

`ink check` calls it after a clean parse/sema pass:

```sh
ink check   # now exits 1 on `var n = 1 + "x";`
```

`ink build/run/test` still rely on `go build` for type errors, so they do not
pay for a second type-check.

## Cache

Results are cached in memory, keyed by SHA-256 of:

- every generated Go file (path and contents, including `//line` paths);
- `go.mod` and `go.sum`;
- the embedded runtime source;
- the `Tests` and `Race` options.

Identical input returns the previous diagnostics without invoking `go list`.
The cache holds the last 16 results. Infrastructure failures (missing module,
network, go list errors that cannot be mapped to a `.ink` file) are not cached.
A local `replace` target that changes in place is not part of the key: restart
the LSP after editing one.

## LSP

`internal/lsp` runs the same check and publishes mapped diagnostics with
source `ink` and a stable `code` from `internal/diag`:

- `initialize`, `didOpen`, `didSave` and `didClose` run it immediately;
- `didChange` runs parse + sema immediately and schedules the typecheck after
  150 ms; a newer edit stops the timer and stale results are dropped, so type
  diagnostics are briefly absent while typing;
- `Tests: true` is used, so `test` blocks are type-checked and mapped back to
  `.ink` files;
- if parsing or sema has a non-warning error, type-checking is skipped;
- projects without `go.mod` are skipped, because packages cannot be resolved
  reliably.

Sema still blocks the message loop on every change; the generated-Go typecheck
runs in a timed worker and publishes only for the current generation. Measured
on this machine, type-checking adds about 1-2 s to a small project; a warm
in-process cache makes repeated opens and saves free.

## Known limits

- Start columns are exact from `//line path:line:col`; end columns are
  one-character fallbacks for sema and typecheck diagnostics, and generated
  paths are absolute (G34).
- Errors are only as good as Go's position mapping: an error in generated
  code that is not covered by a `//line` directive becomes a project-level
  error. If the package also has a `.ink` parse/sema error, that unmapped
  project-level error can be dropped while the source error is reported.
- External Go imports must already be present in `go.mod`/`go.sum`. The LSP
  does not run `go mod tidy`.
- The cache key does not include dependency sources, hand-written `.go` files
  in the project, or `GOOS`/`GOARCH`. Restart the LSP after editing one of
  those (there is a `shortcut:` in `typecheck.go`).
- The LSP still does the whole-project sema pass on every keystroke (G43);
  only the generated-Go typecheck is debounced and generation-checked.

## Next steps

1. Move sema to a worker with a latest-version queue and cancellation (G43).
2. Cache per package instead of per project, so editing one package does not
   reload unrelated ones.
3. Use relative `//line` paths and exact end positions (G34).

## Verification

- `internal/driver` test `TestCheckTypeErrors`: a type error in a local
  package is reported at `lib/lib.ink:2`, and no `zz_ink.go` is written to
  the source tree.
- `internal/driver` test `TestCheckTestTypeErrors`: a type error in a `test`
  block of a nested package is reported at `lib/lib.ink:2`, and no
  `zz_ink_test.go` is written to the source tree.
- `internal/lsp` test `TestTypeDiagnostics`: `int + "x"` is published on
  `didOpen` with its column, cleared on the immediate `didChange` publish, and
  published again on save from cache.
- `internal/lsp` test `TestDidChangeDebounceAndStaleness` covers the 150 ms
  typecheck debounce and stale-result drop; `TestTestBlockTypeDiagnostic`
  covers type errors inside `test` blocks.
- Manual: all `tests/cases` still pass `ink check` with the new
  type-checking enabled.
