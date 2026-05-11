# Editor integrations for Trellis

Drop-in configuration for editors that speak LSP. Every integration spawns
`trellis lsp` on stdio, so the only prerequisite is having the `trellis` binary
on `$PATH`:

```sh
cd ~/code/github.com/norlinga/trellis
go build -o ./trellis ./cmd/trellis
sudo install -m 0755 ./trellis /usr/local/bin/trellis   # or any $PATH dir
```

Once that's in place, open any `*.trellis` file in your editor of choice and
edits will surface lint diagnostics inline.

## Editors

| Editor              | Folder              | Status                                                        |
|---------------------|---------------------|---------------------------------------------------------------|
| Neovim              | [`neovim/`](neovim) | Filetype detection + LSP client (lspconfig).                  |
| VS Code / Cursor    | [`vscode/`](vscode) | Client extension: spawns `trellis lsp`, TextMate syntax highlighting, hover, jump-to-definition. Cursor consumes VS Code extensions natively, so the same `.vsix` works in both. |

Both integrations track every LSP capability the server advertises:
full-document sync, `textDocument/publishDiagnostics`,
`textDocument/hover`, and `textDocument/definition`.
