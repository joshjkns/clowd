# Clowd for Vim and Neovim

Support for [Clowd](https://clowd.jooo.sh) `.clowd` files:

- **File type** detection, with two-space indentation and `#` comments.
- **Syntax highlighting** from a Vim syntax file.
- **Language server** setup for Neovim 0.11+ (`lsp/clowd.lua`): diagnostics,
  hovers, completion, formatting and semantic highlighting from
  `clowd lsp`. Once the server attaches, its colours come from the real
  parse and take priority over the syntax file.

The server needs `clowd` on your `PATH`:

```sh
go install github.com/joshjkns/clowd/cmd/clowd@latest   # into ~/go/bin
```

## Install

The plugin is the `editors/vim` folder of the Clowd repository, so point
your plugin manager at that folder.

**Neovim 0.12 (`vim.pack`)**

```lua
vim.pack.add({ "https://github.com/joshjkns/clowd" })
vim.opt.rtp:append(vim.fn.stdpath("data") .. "/site/pack/core/opt/clowd/editors/vim")
vim.lsp.enable("clowd")
```

**lazy.nvim**

```lua
{
  "joshjkns/clowd",
  ft = "clowd",
  init = function() vim.filetype.add({ extension = { clowd = "clowd" } }) end,
  config = function(plugin)
    vim.opt.rtp:append(plugin.dir .. "/editors/vim")
    vim.lsp.enable("clowd")
  end,
}
```

**vim-plug**

```vim
Plug 'joshjkns/clowd', { 'rtp': 'editors/vim' }
```

**From a checkout**

```lua
vim.opt.rtp:append("~/code/clowd/editors/vim")
vim.lsp.enable("clowd")
```

To pass your completion plugin's capabilities, or a `clowd` that isn't on
`PATH`, extend the config before enabling it:

```lua
vim.lsp.config("clowd", {
  capabilities = require("cmp_nvim_lsp").default_capabilities(),
  cmd = { vim.fn.expand("~/go/bin/clowd"), "lsp" },
})
```

## Checking it works

Open a `.clowd` file and run `:checkhealth vim.lsp`: `clowd` should be
listed as an active client. `:Inspect` on a word shows the semantic token
the server gave it, such as `@lsp.type.operator.clowd` on `connects to`.
