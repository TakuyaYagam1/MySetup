return {
  {
    "catppuccin/nvim",
    name = "catppuccin",
    priority = 1000,
    event = "VimEnter",
    config = function()
      require("catppuccin").setup({
        flavour = "macchiato",
        transparent_background = true,
        term_colors = true,
      })
      vim.cmd.colorscheme("catppuccin")

      for _, group in ipairs({
        "Normal",
        "NormalNC",
        "NormalFloat",
        "SignColumn",
        "LineNr",
        "CursorLineNr",
        "EndOfBuffer",
        "NeoTreeNormal",
        "NeoTreeNormalNC",
      }) do
        vim.api.nvim_set_hl(0, group, { bg = "none" })
      end
    end,
  },
}
