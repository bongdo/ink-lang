# Editor setup

The server is `ink lsp`. It speaks LSP over stdio and uses the project root
with `go.mod` (or the current directory).

## Neovim

Add to `init.lua`:

```lua
vim.filetype.add({ extension = { ink = "ink" } })

vim.api.nvim_create_autocmd("FileType", {
  pattern = "ink",
  callback = function(args)
    vim.lsp.start({
      name = "ink",
      cmd = { "ink", "lsp" },
      root_dir = vim.fs.root(args.buf, "go.mod"),
    })
  end,
})
```

Format on save:

```lua
vim.api.nvim_create_autocmd("BufWritePre", {
  pattern = "*.ink",
  callback = function()
    vim.lsp.buf.format({ name = "ink" })
  end,
})
```

## pero

`pero` preconfigures `ink = "ink lsp"` for `.ink` files and has an Ink
chroma lexer for syntax highlighting. Put `ink` on `PATH`, or override the
command in `~/.config/pero/config.toml`:

```toml
[lsp]
servers = { ink = "/path/to/ink lsp" }
```

`pero-inked` starts the same server automatically; `bin/run` sets
`INK_LSP` to `../ink/bin/ink`.

## VS Code

```sh
cd editors/vscode
npm install
npm run compile
```

Open the folder in VS Code and press F5 (Run Extension). The grammar is
`source.ink`, files end in `.ink`. `ink.serverPath` points at the
executable if it is not on `PATH`.

## OpenCode

OpenCode disables LSP by default. The repository ships `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "lsp": {
    "ink": {
      "command": ["bin/ink", "lsp"],
      "extensions": [".ink"]
    }
  },
  "formatter": {
    "ink": {
      "command": ["bin/ink", "fmt", "-w", "$FILE"],
      "extensions": [".ink"]
    }
  }
}
```

Build `bin/ink` first (`bin/build`); outside this repository use `ink`
from `PATH`. Restart OpenCode after changing `opencode.json`; LSP is disabled
by default.
After `edit`/`write`, OpenCode touches the file, waits for LSP
diagnostics and appends errors to the tool result. Its `lsp` tool uses
definition, references, hover and document/workspace symbols. OpenCode edits
files through `edit`/`write`/`patch`, not through LSP; code actions and rename
are available to editors, not to the OpenCode agent. `ink check --json`
remains the fallback and the completion gate.

## Features

- diagnostics from parsing, sema and generated-Go type-checking, including
  `test` blocks; full text sync
- hover with types and signatures
- go-to-definition for local, package and cross-file symbols where the symbol is resolved
- references for project declarations: locals, parameters, globals, constants,
  functions, types, fields and enum variants; `includeDeclaration` controls
  whether the declaration is part of the result
- rename (`prepareRename` and `rename`) for the same project declarations,
  returning a multi-file `WorkspaceEdit`; the declaration is included
- code actions for `const-local`, `decl-equals`, `iface-fn`, `import-simple`,
  `import-alias`, `err-manual` and `nil-untyped`
- completion for keywords, builtins, locals and package members; struct fields and Go selectors are limited
- document symbols and workspace symbols (types, variants, methods, functions,
  variables, constants and tests; case-insensitive substring query)
- signature help
- document formatting

Diagnostics are recomputed on every change. Parse and sema run on the analysis worker;
the generated-Go typecheck waits 150 ms after the last edit and stale results
are dropped. `didOpen`, `didSave` and `didClose` typecheck immediately.
`didClose` publishes empty diagnostics for the closed buffer, drops it and
re-analyzes from disk, so diagnostics for the file on disk can appear again.
The typecheck runs only when the project has `go.mod` and
`go.mod`, and no parse or sema errors.

### References and rename limits

- Only declarations in the project are renamed. Go library symbols, builtins
  and package aliases are not rename targets.
- Rename validates the new identifier and refuses to shorten an exported symbol
  that is used from another local package. It does not check for name conflicts.
- References and rename use the current semantic analysis; unsaved buffers are
  included.

### Quick fixes

Code actions are attached to diagnostics the client sends in `codeAction`.
`nil-untyped` is conservative: it fires only when the expected type is visible
from a typed `var`, a return result or a call parameter; otherwise no action is
offered. The CLI does not apply fixes; `ink fmt` only formats.

Go type errors are reported by `ink build` / `ink check`; the editor
shows the Ink-specific diagnostics immediately. Generated Go contains
`//line` directives with columns, so compiler errors point back to `.ink`
files.
