# Trellis for VS Code

Minimal client extension that spawns `trellis lsp` and surfaces its
diagnostics inline. Intentionally tiny — when the LSP is mature enough to
warrant its own marketplace release, this scaffold will graduate into the
`trellis-vscode` repository called out in `trellis-plan/PLAN.md`.

## Prerequisites

- The `trellis` binary on `$PATH`. From the repo root:
  ```sh
  go build -o ./trellis ./cmd/trellis
  sudo install -m 0755 ./trellis /usr/local/bin/trellis
  ```
- Node.js 18+ (only for building the extension).
- VS Code 1.85+.

## Build and install locally

```sh
cd editors/vscode
npm install
npm run compile

# Option A — sideload via vsce
npx @vscode/vsce package
code --install-extension trellis-vscode-0.1.0.vsix

# Option B — load unpacked (for development)
# In VS Code: F5 from this folder launches an Extension Development Host
# with the extension active.
```

## Configuration

| Setting               | Default     | Purpose                                                    |
|-----------------------|-------------|------------------------------------------------------------|
| `trellis.executable`  | `"trellis"` | Path to the trellis binary. Use absolute path if needed.   |
| `trellis.trace.server`| `"off"`     | LSP trace verbosity (`off` / `messages` / `verbose`).      |

## What works today

- `*.trellis` files are recognized as language `trellis`.
- TextMate-based syntax highlighting (frontmatter keys, block keywords,
  scenario kinds, step keywords, RFC 2119 keywords, handles, dates,
  comments, single-quoted literals).
- `#` line comments toggle with the standard VS Code shortcut.
- The extension starts `trellis lsp` on stdio and forwards
  open/change/save events; lint diagnostics appear in the Problems panel
  and inline as red/yellow squiggles.
- Hover on a `Consumes:` handle shows the provider's feature name,
  summary, and source filename. Hover on a `Provides:` handle
  self-documents.
- `Go to Definition` (F12, Ctrl-click) on a `Consumes:` handle jumps to
  the matching `Provides:` entry in the producer sidecar.

## Not yet implemented

- Find-references / reverse jump-to-callers.
- Workspace re-scan on file create/delete (cross-file lookups currently
  refresh on save of an existing file; new files require an editor
  reload to enter the index).
- Tree-sitter-grade highlighting (would need a separate VS Code
  extension that bridges tree-sitter; the included TextMate grammar
  covers the visible cases without that dependency).

## Troubleshooting

- Output panel → "Trellis Language Server" shows the server's logs.
- Set `trellis.trace.server` to `verbose` to log every JSON-RPC message
  exchanged with the server.
- `Developer: Reload Window` after changing the `trellis.executable`
  setting.
