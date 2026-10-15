// A small, forgiving highlighter for Clowd source. It mirrors the parser's
// rules closely enough for colour (keywords are positional, verbs are checked
// before kinds, raw `on <cloud>` settings are left alone) without trying to
// validate anything. It must cope with half-typed lines, since the hero
// animation highlights the file one character at a time.

export type TokenKind =
  | 'kw'
  | 'kind'
  | 'name'
  | 'verb'
  | 'cloud'
  | 'trait'
  | 'prop'
  | 'value'
  | 'num'
  | 'raw'
  | 'comment'
  | 'punct'
  | 'err'
  | 'plain'

export type Token = { text: string; kind: TokenKind }

export const VERBS = [
  ['reads', 'and', 'writes'],
  ['connects', 'to'],
  ['sends', 'to'],
  ['reads'],
  ['writes'],
]

// Wrong guesses the parser accepts with a "did you mean" error
// (internal/syntax/parser/words.go).
export const VERB_GUESSES: [string[], string][] = [
  [['connects', 'with'], 'connects to'],
  [['connect', 'to'], 'connects to'],
  [['links', 'to'], 'connects to'],
  [['talks', 'to'], 'connects to'],
  [['depends', 'on'], 'connects to'],
  [['reads', 'from'], 'reads'],
  [['writes', 'to'], 'writes'],
  [['reads', 'writes'], 'reads and writes'],
  [['read', 'and', 'write'], 'reads and writes'],
  [['writes', 'and', 'reads'], 'reads and writes'],
  [['send', 'to'], 'sends to'],
  [['publishes', 'to'], 'sends to'],
  [['reads/writes'], 'reads and writes'],
  [['connects'], 'connects to'],
  [['links'], 'connects to'],
  [['uses'], 'connects to'],
  [['calls'], 'connects to'],
  [['read'], 'reads'],
  [['write'], 'writes'],
  [['sends'], 'sends to'],
]

const KINDS = [['load', 'balancer'], ['service'], ['database'], ['bucket'], ['lb'], ['queue'], ['project'], ['network']]
const CLOUDS = new Set(['aws', 'gcp', 'azure'])
const VALUE_KEYWORDS = new Set(['to', 'on', 'with', 'tls', 'for', 'days', 'times', 'off', 'and'])
const MULTI_WORD_PROPS = [['health', 'check'], ['expire', 'after'], ['keep', 'messages', 'for']]
const NUMBER = /^\d+(\.\d+)?(s|m|h|d|mb|gb)?$/i

type Piece = { text: string; ws: boolean }

// split keeps whitespace so tokens concatenate back to the exact line.
function split(line: string): { pieces: Piece[]; comment: string } {
  const pieces: Piece[] = []
  let comment = ''
  const re = /(\s+)|("(?:[^"\\]|\\.)*"?)|([^\s]+)/g
  let m: RegExpExecArray | null
  while ((m = re.exec(line))) {
    if (m[1]) {
      pieces.push({ text: m[1], ws: true })
      continue
    }
    const word = m[0]
    if (word.startsWith('#')) {
      comment = line.slice(m.index)
      break
    }
    // A trailing comma is punctuation; `a,b` stays one value.
    if (word.length > 1 && word.endsWith(',') && !m[2]) {
      pieces.push({ text: word.slice(0, -1), ws: false })
      pieces.push({ text: ',', ws: false })
    } else {
      pieces.push({ text: word, ws: false })
    }
  }
  return { pieces, comment }
}

function matchAt(words: string[], at: number, phrase: string[]): boolean {
  return phrase.every((p, i) => words[at + i]?.toLowerCase() === p)
}

function longest(words: string[], at: number, phrases: string[][]): number {
  let best = 0
  for (const p of phrases) if (p.length > best && matchAt(words, at, p)) best = p.length
  return best
}

function classifyValue(w: string): TokenKind {
  if (w === ',') return 'punct'
  if (VALUE_KEYWORDS.has(w.toLowerCase())) return 'kw'
  if (NUMBER.test(w)) return 'num'
  return 'value'
}

function classifyLine(words: string[], indent: number, inRaw: boolean): TokenKind[] {
  const kinds: TokenKind[] = new Array(words.length).fill('plain')
  const lower = words.map((w) => w.toLowerCase())
  const set = (from: number, to: number, k: TokenKind) => {
    for (let i = from; i < to && i < words.length; i++) kinds[i] = k
  }

  if (inRaw) {
    kinds[0] = 'raw'
    for (let i = 1; i < words.length; i++) kinds[i] = words[i] === ',' ? 'punct' : NUMBER.test(words[i]) ? 'num' : 'value'
    return kinds
  }

  if (lower[0] === 'on') {
    kinds[0] = 'kw'
    if (words[1]) kinds[1] = CLOUDS.has(lower[1]) ? 'cloud' : 'plain'
    return kinds
  }

  if (indent > 0) {
    const n = longest(words, 0, MULTI_WORD_PROPS) || 1
    set(0, n, 'prop')
    for (let i = n; i < words.length; i++) kinds[i] = classifyValue(words[i])
    return kinds
  }

  if (lower[0] === 'use') {
    kinds[0] = 'kw'
    if (words[1]) kinds[1] = CLOUDS.has(lower[1]) ? 'cloud' : 'plain'
    if (lower[2] === 'in') kinds[2] = 'kw'
    set(3, words.length, 'value')
    return kinds
  }

  // Verbs are checked before kinds, as in the parser, so `queue connects to db` works.
  const verb = longest(words, 1, VERBS)
  const guess = verb ? 0 : longest(words, 1, VERB_GUESSES.map(([w]) => w))
  if (words.length > 1 && (verb || guess)) {
    const n = verb || guess
    kinds[0] = 'name'
    set(1, 1 + n, verb ? 'verb' : 'err')
    set(1 + n, words.length, 'name')
    return kinds
  }

  const kind = longest(words, 0, KINDS)
  if (kind) {
    set(0, kind, 'kind')
    let i = kind
    if (lower[0] !== 'network' && words[i] && lower[i] !== 'is') kinds[i++] = 'name'
    if (lower[i] === 'is') {
      kinds[i++] = 'kw'
      for (; i < words.length; i++) {
        kinds[i] = words[i] === ',' ? 'punct' : lower[i] === 'and' ? 'kw' : NUMBER.test(words[i]) ? 'num' : 'trait'
      }
    }
    return kinds
  }

  if (words.length > 1) kinds[0] = 'name'
  return kinds
}

/** Highlight Clowd source into one token list per line. */
export function highlight(src: string): Token[][] {
  const out: Token[][] = []
  // Indents of the `on <cloud>` lines we are inside; anything deeper is raw.
  let rawAt: number[] = []
  for (const line of src.split('\n')) {
    const { pieces, comment } = split(line)
    const indent = line.length - line.trimStart().length
    const tokens: Token[] = []
    const words = pieces.filter((p) => !p.ws).map((p) => p.text)
    if (words.length) {
      rawAt = rawAt.filter((at) => at < indent)
      const kinds = classifyLine(words, indent, rawAt.length > 0)
      let w = 0
      for (const p of pieces) tokens.push({ text: p.text, kind: p.ws ? 'plain' : kinds[w++] })
      if (words[0].toLowerCase() === 'on' && rawAt.length === 0) rawAt.push(indent)
    } else {
      for (const p of pieces) tokens.push({ text: p.text, kind: 'plain' })
    }
    if (comment) tokens.push({ text: comment, kind: 'comment' })
    out.push(tokens)
  }
  return out
}

/** Tailwind classes for each token kind. Colours live in globals.css. */
export const tokenClass: Record<TokenKind, string> = {
  kw: 'text-tok-kw',
  kind: 'text-tok-kind font-semibold',
  name: 'text-tok-name font-semibold',
  verb: 'text-tok-verb font-semibold',
  cloud: 'text-tok-cloud',
  trait: 'text-tok-trait',
  prop: 'text-tok-prop',
  value: 'text-tok-value',
  num: 'text-tok-num',
  raw: 'text-tok-raw italic',
  comment: 'text-tok-comment italic',
  punct: 'text-tok-comment',
  err: 'text-tok-verb underline decoration-wavy decoration-[#ff5d5d] underline-offset-4',
  plain: '',
}
