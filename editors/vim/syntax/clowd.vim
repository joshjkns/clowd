" Vim syntax file for Clowd (.clowd).
"
" In Neovim with `clowd lsp` running, the server's semantic tokens colour the
" file from the real parse and take priority over this; this is the fallback
" for Vim, and for the moment before the server starts.
"
" Mirrors the parser: keywords are positional and case-insensitive, verbs are
" matched before kinds, and lines under `on <cloud>` are raw provider settings.
" The groups match what the semantic token types link to by default.

if exists('b:current_syntax')
  finish
endif

syntax case ignore

" Shared pieces, used inside the line matches below.
syntax match clowdComment /\%(^\|\s\)\zs#.*$/
syntax region clowdString start=/"/ skip=/\\./ end=/"/ oneline contained
syntax match clowdNumber /\<\d\+\%(\.\d\+\)\?\%(s\|m\|h\|d\|mb\|gb\)\?\>/ contained
syntax match clowdPunct /,/ contained
syntax keyword clowdKeyword contained in is and to on with tls for times days off
syntax keyword clowdCloud contained aws gcp azure

" use <cloud> [in <region>]
syntax match clowdUseLine /^use\>.*$/ contains=clowdUse,clowdCloud,clowdKeyword,clowdComment
syntax match clowdUse /^use\>/ contained

" <kind> <name> [is <traits>]
" The kind and verb are matched first, so names that follow them use a
" lookbehind rather than restarting from the line start.
syntax match clowdDecl /^\%(load\s\+balancer\|service\|database\|bucket\|lb\|queue\|project\|network\)\>.*$/
      \ contains=clowdKind,clowdDeclName,clowdTraits,clowdComment
syntax match clowdKind /^\%(load\s\+balancer\|service\|database\|bucket\|lb\|queue\|project\|network\)\>/ contained
syntax match clowdDeclName /\%(\%(balancer\|service\|database\|bucket\|lb\|queue\|project\)\s\+\)\@<=\%(is\>\)\@![a-z][a-z0-9_-]*/ contained
syntax match clowdTraits /\<is\>.*$/ contained contains=clowdKeyword,clowdNumber,clowdPunct,clowdComment

" <name> <verb> <name>. Defined after declarations so it wins at the same
" position, as the parser checks verbs first (`queue connects to db`).
syntax match clowdRelate /^[a-z][a-z0-9_-]*\s\+\%(connects\s\+to\|reads\s\+and\s\+writes\|reads\|writes\|sends\s\+to\)\%(\s\|$\).*$/
      \ contains=clowdRelName,clowdVerb,clowdComment
syntax match clowdRelName /^[a-z][a-z0-9_-]*/ contained
syntax match clowdRelName /\%(\%(to\|reads\|writes\)\s\+\)\@<=[a-z][a-z0-9_-]*\ze\s*\%(#.*\)\?$/ contained
syntax match clowdVerb /\<\%(connects\s\+to\|reads\s\+and\s\+writes\|sends\s\+to\|reads\|writes\)\>/ contained

" Indented properties: a key, then values.
syntax match clowdPropLine /^\s\+\S.*$/ contains=clowdProp,clowdKeyword,clowdNumber,clowdString,clowdPunct,clowdComment
syntax match clowdProp /^\s\+\zs\%(health\s\+check\|expire\s\+after\|keep\s\+messages\s\+for\|[a-z_]\+\)/ contained

" on <cloud>, and everything indented under it. Defined last so it wins over
" the property line it starts on. Ends at the first line that isn't deeper.
syntax region clowdOnBlock matchgroup=clowdOn
      \ start=/^\z(\s*\)on\s\+\%(aws\|gcp\|azure\)\s*$/
      \ end=/^\ze\%(\z1\s\)\@!/
      \ contains=clowdRawKey,clowdNumber,clowdPunct,clowdComment
syntax match clowdRawKey /^\s\+\zs[a-z_][a-z0-9_]*/ contained

highlight default link clowdComment Comment
highlight default link clowdString String
highlight default link clowdNumber Number
highlight default link clowdPunct Delimiter
highlight default link clowdKeyword Keyword
highlight default link clowdUse Keyword
highlight default link clowdOn Keyword
highlight default link clowdCloud Constant
highlight default link clowdUseLine String
highlight default link clowdKind Structure
highlight default link clowdDeclName Identifier
highlight default link clowdTraits Type
highlight default link clowdRelName Identifier
highlight default link clowdVerb Operator
highlight default link clowdPropLine String
highlight default link clowdProp Identifier
highlight default link clowdOnBlock String
highlight default link clowdRawKey PreProc

let b:current_syntax = 'clowd'
