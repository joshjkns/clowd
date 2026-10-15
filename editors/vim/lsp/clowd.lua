-- The language server for Neovim 0.11+, read by vim.lsp.config("clowd").
-- Enable it with vim.lsp.enable("clowd"). It gives diagnostics, hovers,
-- completion, formatting and semantic highlighting.
return {
	cmd = { "clowd", "lsp" },
	filetypes = { "clowd" },
	root_markers = { "clowd.lock", ".git" },
}
