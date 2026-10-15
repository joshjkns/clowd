-- Neovim: detect .clowd files even when the plugin is loaded after startup
-- (lazy.nvim, or a runtimepath appended in init.lua), when ftdetect/ has
-- already been read.
vim.filetype.add({ extension = { clowd = "clowd" } })
