# Ink for VS Code

- syntax highlighting (`source.ink`)
- LSP client: diagnostics and basic editing support (hover, definition, completion, symbols, formatting);
  signature help

## Build

```sh
npm install
npm run compile
```

Open this folder in VS Code and press F5. The extension starts `ink lsp`
from `PATH`; set `ink.serverPath` if it lives elsewhere.
