-- lua/plugins/dart.lua
return {
  -- LSP dartls
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        dartls = {
          -- cmd = { "/etc/profiles/per-user/your-username/bin/dart", "language-server", "--protocol=lsp" },
        },
      },
    },
  },

  -- Treesitter
  {
    "nvim-treesitter/nvim-treesitter",
    opts = {
      ensure_installed = { "dart" },
    },
  },
}
