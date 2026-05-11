-- Neovim integration for Trellis sidecar files.
--
-- Usage:
--   require("trellis").setup()             -- defaults: cmd = { "trellis", "lsp" }
--   require("trellis").setup({ cmd = ... }) -- override the spawn command
--
-- Requires nvim-lspconfig. Tested with Neovim 0.10+.

local M = {}

local function register_filetype()
  vim.filetype.add({
    extension = {
      trellis = "trellis",
    },
    pattern = {
      [".*%.trellis"] = "trellis",
    },
  })

  vim.api.nvim_create_autocmd("FileType", {
    pattern = "trellis",
    callback = function(args)
      vim.bo[args.buf].commentstring = "# %s"
      vim.bo[args.buf].comments = ":#"
    end,
  })
end

local function register_lsp(cmd, on_attach, capabilities)
  local ok, lspconfig = pcall(require, "lspconfig")
  if not ok then
    vim.notify("trellis: nvim-lspconfig not found; LSP client not registered",
               vim.log.levels.WARN)
    return
  end

  local configs = require("lspconfig.configs")
  if not configs.trellis then
    configs.trellis = {
      default_config = {
        cmd = cmd,
        filetypes = { "trellis" },
        root_dir = lspconfig.util.root_pattern(".git", ".trellis-policy") or
                   lspconfig.util.path.dirname,
        single_file_support = true,
        settings = {},
      },
    }
  end

  lspconfig.trellis.setup({
    on_attach = on_attach,
    capabilities = capabilities,
  })
end

function M.setup(opts)
  opts = opts or {}
  local cmd = opts.cmd or { "trellis", "lsp" }
  register_filetype()
  register_lsp(cmd, opts.on_attach, opts.capabilities)
end

return M
