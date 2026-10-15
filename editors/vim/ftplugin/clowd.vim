if exists('b:did_ftplugin')
  finish
endif
let b:did_ftplugin = 1

" Indentation is spaces only (a tab is an error) and clowd fmt uses two.
setlocal expandtab shiftwidth=2 softtabstop=2
setlocal commentstring=#\ %s comments=:#

let b:undo_ftplugin = 'setlocal expandtab< shiftwidth< softtabstop< commentstring< comments<'
