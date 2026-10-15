import { VERBS, VERB_GUESSES } from './clowd'

// A toy version of the checker for one relationship line, against a fixed
// file. Its messages are copied from what `clowd check` prints for the same
// input, so the page never promises a message the compiler doesn't give.

export type Kind = 'service' | 'database' | 'bucket' | 'queue'

export const DECLARED: Record<string, Kind> = {
  api: 'service',
  worker: 'service',
  db: 'database',
  uploads: 'bucket',
  jobs: 'queue',
}

export const TOY_FILE = `use aws in eu-west-2

service api
  image ghcr.io/me/api:latest

service worker
  image ghcr.io/me/worker:latest

database db is postgres 16
bucket uploads
queue jobs
`

export type Diag = {
  severity: 'error' | 'warning'
  message: string
  // Character range in the input to underline.
  from: number
  to: number
  help?: string
  // The whole line with the fix applied, when there is one obvious fix.
  fix?: string
}

export type Result =
  | { kind: 'empty' }
  | { kind: 'diags'; diags: Diag[] }
  | { kind: 'ok'; env: string[]; note?: string }

function distance(a: string, b: string): number {
  const d = Array.from({ length: a.length + 1 }, (_, i) => [i, ...Array(b.length).fill(0)])
  for (let j = 1; j <= b.length; j++) d[0][j] = j
  for (let i = 1; i <= a.length; i++)
    for (let j = 1; j <= b.length; j++)
      d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1))
  return d[a.length][b.length]
}

function closest(name: string): string | undefined {
  let best: string | undefined
  let bestD = 3
  for (const n of Object.keys(DECLARED)) {
    const d = distance(name.toLowerCase(), n)
    if (d < bestD) [best, bestD] = [n, d]
  }
  return best
}

const ARTICLE: Record<Kind, string> = { service: 'a service', database: 'a database', bucket: 'a bucket', queue: 'a queue' }

function envName(n: string) {
  return n.toUpperCase().replace(/-/g, '_')
}

/** Check a single relationship line. */
export function check(line: string): Result {
  const words: { w: string; at: number }[] = []
  for (const m of line.matchAll(/\S+/g)) words.push({ w: m[0], at: m.index! })
  if (words.length === 0) return { kind: 'empty' }
  const lower = words.map((x) => x.w.toLowerCase())
  const span = (i: number, j: number) => ({ from: words[i].at, to: words[j - 1].at + words[j - 1].w.length })
  const diags: Diag[] = []

  // Verb: canonical, a known wrong guess, or an unknown word.
  const match = (phrases: string[][]) => {
    let best = 0
    for (const p of phrases) if (p.length > best && p.every((w, k) => lower[1 + k] === w)) best = p.length
    return best
  }
  let verb = ''
  let n = match(VERBS)
  if (n) verb = lower.slice(1, 1 + n).join(' ')
  else {
    n = match(VERB_GUESSES.map(([w]) => w))
    if (n) {
      const said = lower.slice(1, 1 + n)
      verb = VERB_GUESSES.find(([w]) => w.join(' ') === said.join(' '))![1]
      const fix = [words[0].w, verb, ...words.slice(1 + n).map((x) => x.w)].join(' ')
      diags.push({ severity: 'error', message: `unknown relationship \`${said.join(' ')}\``, ...span(1, 1 + n), help: `did you mean \`${verb}\`?`, fix })
    } else if (words.length >= 3) {
      n = words.length - 2
      diags.push({
        severity: 'error',
        message: `unknown relationship \`${words.slice(1, 1 + n).map((x) => x.w).join(' ')}\``,
        ...span(1, 1 + n),
        help: 'relationships are `connects to`, `reads`, `writes`, `reads and writes`, `sends to`',
      })
    }
  }

  if (!n) {
    return {
      kind: 'diags',
      diags: [
        {
          severity: 'error',
          message: `unknown statement \`${words[0].w}\``,
          ...span(0, words.length),
          help: 'relate two names: `api connects to db`',
        },
      ],
    }
  }

  const target = words[1 + n]
  if (!target) {
    diags.push({ severity: 'error', message: `expected a name after \`${verb || lower[n]}\``, ...span(n, 1 + n) })
    return { kind: 'diags', diags }
  }
  if (words.length > 2 + n) {
    diags.push({ severity: 'error', message: 'one target per relationship', ...span(2 + n, words.length) })
  }

  // Names.
  const names = [
    { i: 0, w: words[0].w },
    { i: 1 + n, w: target.w },
  ]
  let resolved = true
  for (const { i, w } of names) {
    const exact = DECLARED[w.toLowerCase()]
    if (!exact) {
      resolved = false
      const near = closest(w)
      const fixed = words.map((x, k) => (k === i ? near : x.w)).join(' ')
      diags.push({
        severity: 'error',
        message: `no resource named \`${w}\``,
        ...span(i, i + 1),
        help: near ? `did you mean \`${near}\`?` : `declare it, for example \`${i === 0 ? 'service' : 'database'} ${w}\``,
        fix: near ? fixed : undefined,
      })
    } else if (w !== w.toLowerCase()) {
      diags.push({ severity: 'warning', message: `\`${w}\` is declared as \`${w.toLowerCase()}\``, ...span(i, i + 1), help: `write \`${w.toLowerCase()}\`` })
    }
  }
  if (diags.some((d) => d.severity === 'error') || !resolved || !verb) return { kind: 'diags', diags }

  const from = lower[0]
  const to = target.w.toLowerCase()
  const fromKind = DECLARED[from]
  const toKind = DECLARED[to]
  const whole = span(0, words.length)

  if (from === to) {
    return { kind: 'diags', diags: [...diags, { severity: 'error', message: `\`${from}\` can't relate to itself`, ...whole }] }
  }
  if (fromKind !== 'service') {
    const flipped = toKind === 'service' ? `${to} ${verb} ${from}` : undefined
    return {
      kind: 'diags',
      diags: [
        ...diags,
        {
          severity: 'error',
          message: `only a service can start a relationship; \`${from}\` is ${ARTICLE[fromKind]}`,
          ...span(0, 1),
          help: flipped ? `relationships go from the service: \`${flipped}\`` : undefined,
          fix: flipped,
        },
      ],
    }
  }

  // Which verbs apply to which kinds, and the checker's suggestion otherwise.
  const ok: Record<string, Kind[]> = {
    'connects to': ['database', 'service'],
    reads: ['bucket', 'queue'],
    writes: ['bucket'],
    'reads and writes': ['bucket'],
    'sends to': ['queue'],
  }
  if (!ok[verb].includes(toKind)) {
    let help: string | undefined
    let fix: string | undefined
    const say = (v: string) => `${from} ${v} ${to}`
    if (toKind === 'database') [help, fix] = [`write \`${say('connects to')}\``, say('connects to')]
    else if (toKind === 'bucket' && verb === 'connects to')
      help = `say what access it needs: \`${say('reads')}\`, \`${say('writes')}\` or \`${say('reads and writes')}\``
    else if (toKind === 'bucket') [help, fix] = [`write \`${say('writes')}\``, say('writes')]
    else if (toKind === 'queue' && verb === 'writes')
      [help, fix] = [`to publish to a queue, write \`${say('sends to')}\``, say('sends to')]
    else if (toKind === 'queue' && verb === 'connects to')
      help = `write \`${say('reads')}\` to consume or \`${say('sends to')}\` to publish`
    else if (toKind === 'queue') help = `write \`${say('reads')}\` and \`${say('sends to')}\` on separate lines`
    return {
      kind: 'diags',
      diags: [...diags, { severity: 'error', message: `\`${verb}\` doesn't apply to ${ARTICLE[toKind]}`, ...whole, help, fix }],
    }
  }

  const N = envName(to)
  if (toKind === 'service') {
    return { kind: 'ok', env: [], note: 'service connects to service is not implemented for aws yet' }
  }
  const env =
    toKind === 'database'
      ? [`${N}_HOST`, `${N}_PORT`, `${N}_NAME`, `${N}_USER`, `${N}_PASSWORD`]
      : toKind === 'bucket'
        ? [`${N}_BUCKET`]
        : [`${N}_QUEUE_URL`]
  return { kind: 'ok', env }
}
