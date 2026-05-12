return {
  {
    "nvim-treesitter/nvim-treesitter",
    opts = function(_, opts)
      opts.ensure_installed = {}
      opts.auto_install = true
      opts.sync_install = false
      return opts
    end,
  },
}
