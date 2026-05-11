# Trellis for Neovim

Filetype detection plus an LSP client for `*.trellis` sidecar files. Tested
with Neovim 0.10+ and `nvim-lspconfig` (any recent version).

## Install

1. Make sure `trellis` is on your `$PATH` (see [`../README.md`](../README.md)).
2. Drop [`trellis.lua`](trellis.lua) into your Neovim runtime, e.g.
   `~/.config/nvim/lua/trellis.lua`.
3. Load it from your `init.lua`:

   ```lua
   require("trellis").setup()
   ```

That's it — open a `.trellis` file and diagnostics show up as you type.

## What you get

- `*.trellis` files are recognized as filetype `trellis` (with `#` line
  comments via `commentstring`).
- `trellis lsp` is registered as the language server for that filetype and
  attaches automatically.
- Full-document sync, so each keystroke triggers a fresh lint pass.

## Optional: tree-sitter highlighting

The grammar lives in a separate repo (`tree-sitter-trellis`) and isn't yet
published to the `nvim-treesitter` registry. To wire it up locally:

```lua
local parser_config = require("nvim-treesitter.parsers").get_parser_configs()
parser_config.trellis = {
  install_info = {
    url = "~/code/github.com/norlinga/tree-sitter-trellis", -- or a git URL
    files = { "src/parser.c" },
    branch = "main",
  },
  filetype = "trellis",
}
-- then: :TSInstall trellis
```

The `queries/highlights.scm` in the grammar repo will be picked up
automatically once the parser is installed.

## Troubleshooting

- `:LspInfo` should list `trellis-lsp` as attached when you open a `.trellis`
  file. If it doesn't, check `:checkhealth lsp` and confirm `trellis` is on
  `$PATH` in the environment Neovim was launched from.
- Run `:lua vim.lsp.set_log_level("debug")` and check `:LspLog` for spawn
  failures or protocol errors.
