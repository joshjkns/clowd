'use client'

import { useEffect, useRef, useState } from 'react'
import { useInView, useReducedMotion } from 'motion/react'
import { RotateCcw } from 'lucide-react'
import built from '@/data/app.aws.tf.json'
import { CHECK_MS, JSON_LINES } from '@/lib/demo'
import { Counter } from './BuildDemo'

const JSON_TEXT = JSON.stringify(built, null, 2).split('\n')
const CHECK_CMD = 'clowd check app.clowd'
const BUILD_CMD = 'clowd build --target aws'

type Stage = 'idle' | 'check-cmd' | 'check-out' | 'build-cmd' | 'build-out' | 'done'

export function SpeedTerminal() {
  const reduce = useReducedMotion()
  const root = useRef<HTMLDivElement>(null)
  const inView = useInView(root, { once: true, amount: 0.4 })
  const [stage, setStage] = useState<Stage>('idle')
  const [chars, setChars] = useState(0)
  const [jsonLines, setJsonLines] = useState(0)
  const [run, setRun] = useState(0)

  useEffect(() => {
    if (!inView) return
    if (reduce) {
      setStage('done')
      setJsonLines(JSON_TEXT.length)
      return
    }
    setStage('check-cmd')
    setChars(0)
    setJsonLines(0)
  }, [inView, reduce, run])

  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | undefined
    let raf = 0
    if (stage === 'check-cmd' || stage === 'build-cmd') {
      const cmd = stage === 'check-cmd' ? CHECK_CMD : BUILD_CMD
      if (chars < cmd.length) t = setTimeout(() => setChars((c) => c + 1), 28)
      else
        t = setTimeout(() => {
          setChars(0)
          setStage(stage === 'check-cmd' ? 'check-out' : 'build-out')
        }, 260)
    } else if (stage === 'check-out') {
      t = setTimeout(() => setStage('build-cmd'), 1500)
    } else if (stage === 'build-out') {
      // About 25 lines a frame: the whole file lands in well under a second.
      let n = 0
      const step = () => {
        n = Math.min(JSON_TEXT.length, n + 25)
        setJsonLines(n)
        if (n < JSON_TEXT.length) raf = requestAnimationFrame(step)
        else t = setTimeout(() => setStage('done'), 120)
      }
      raf = requestAnimationFrame(step)
    }
    return () => {
      if (t) clearTimeout(t)
      cancelAnimationFrame(raf)
    }
  }, [stage, chars])

  const at = (s: Stage) => ['idle', 'check-cmd', 'check-out', 'build-cmd', 'build-out', 'done'].indexOf(stage) >= ['idle', 'check-cmd', 'check-out', 'build-cmd', 'build-out', 'done'].indexOf(s)

  const jsonBox = useRef<HTMLPreElement>(null)
  useEffect(() => {
    const el = jsonBox.current
    if (el && stage === 'build-out') el.scrollTop = el.scrollHeight
  }, [jsonLines, stage])

  return (
    <div ref={root} className="overflow-hidden rounded-2xl border border-line bg-[#08090c] font-mono text-[12.5px] leading-[1.7]">
      <div className="flex items-center gap-3 border-b border-line px-4 py-2.5 text-xs text-mute">
        <span className="text-faint">~/shop</span>
        <button
          type="button"
          onClick={() => setRun((n) => n + 1)}
          disabled={!at('done')}
          className="ml-auto flex items-center gap-1.5 rounded-md border border-line px-2.5 py-1 transition enabled:hover:border-mute enabled:hover:text-paper disabled:opacity-40"
        >
          <RotateCcw className="size-3.5" aria-hidden /> run again
        </button>
      </div>
      <div className="min-h-[34rem] px-4 py-4">
        <Prompt text={stage === 'check-cmd' ? CHECK_CMD.slice(0, chars) : at('check-out') ? CHECK_CMD : ''} typing={stage === 'check-cmd' || stage === 'idle'} />
        {at('check-out') && (
          <div className="mb-4">
            <CheckOutput />
            <p className="mt-1 text-faint">
              <span className="text-tok-num">{CHECK_MS} ms</span> wall clock, parse to matrix
            </p>
          </div>
        )}
        {at('build-cmd') && <Prompt text={stage === 'build-cmd' ? BUILD_CMD.slice(0, chars) : BUILD_CMD} typing={stage === 'build-cmd'} />}
        {at('build-out') && (
          <>
            <pre
              ref={jsonBox}
              className="scrollbar-none my-2 h-40 overflow-y-auto rounded-lg border border-line/70 bg-ink-2/60 px-3 py-2 text-[11.5px] leading-[1.6]"
            >
              {JSON_TEXT.slice(0, jsonLines).map((l, i) => (
                <JsonLine key={i} line={l} />
              ))}
            </pre>
            <p className="text-tok-cloud">
              ✓ <span className="text-paper">built aws</span> <span className="text-faint">→ .clowd/build/aws/main.tf.json</span>
              <span className="float-right text-faint">
                <Counter value={stage === 'done' ? JSON_LINES : Math.min(jsonLines, JSON_LINES)} className="text-tok-num" /> lines
              </span>
            </p>
          </>
        )}
        {at('done') && <Prompt text="" typing />}
      </div>
    </div>
  )
}

function Prompt({ text, typing }: { text: string; typing: boolean }) {
  return (
    <p>
      <span className="select-none text-tok-verb">❯ </span>
      <span className="text-paper">{text}</span>
      {typing && <span className="ml-px inline-block h-[1.1em] w-[0.55em] translate-y-[0.2em] animate-blink bg-paper/80" />}
    </p>
  )
}

const MARK = { '✓': 'text-ok', '–': 'text-faint' } as const

// Real output of `clowd check examples/app.clowd`, coloured as the CLI does.
function CheckOutput() {
  const rows: [string, string, string, string][] = [
    ['Resources', '', '', 'h'],
    ['  database db', '✓', '✓', ''],
    ['  bucket uploads', '✓', '✓', ''],
    ['  service api', '✓', '✓', ''],
    ['  load balancer web', '✓', '✓', ''],
    ['Relationships', '', '', 'h'],
    ['  api connects to db', '✓', '✓', ''],
    ['  api reads and writes uploads', '✓', '✓', ''],
    ['Overrides', '', '', 'h'],
    ['  database db on aws', '✓', '–', ''],
  ]
  return (
    <div className="text-mute">
      <p className="text-paper">
        app.clowd <span className="text-faint">→</span> <span className="text-tok-cloud">aws</span> (eu-west-2),{' '}
        <span className="text-tok-cloud">gcp</span> (europe-west2)
      </p>
      <div className="mt-2 grid grid-cols-[minmax(0,17rem)_2.5rem_2.5rem] whitespace-pre">
        {rows.map(([label, a, g, h], i) => (
          <div key={i} className={`contents ${h ? 'font-semibold text-paper' : ''}`}>
            <span className={h && i > 0 ? 'mt-2' : ''}>{label}</span>
            <span className={`${h && i > 0 ? 'mt-2' : ''} ${MARK[a as keyof typeof MARK] ?? ''}`}>{h && i === 0 ? 'aws' : a}</span>
            <span className={`${h && i > 0 ? 'mt-2' : ''} ${MARK[g as keyof typeof MARK] ?? ''}`}>{h && i === 0 ? 'gcp' : g}</span>
          </div>
        ))}
      </div>
      <p className="mt-2 text-tok-cloud">
        ✓ <span className="text-paper">deployable to aws, gcp</span>
      </p>
    </div>
  )
}

function JsonLine({ line }: { line: string }) {
  const m = line.match(/^(\s*)(?:("(?:[^"\\]|\\.)*")(:\s?))?(.*)$/)
  if (!m) return <span className="block">{line}</span>
  const [, ws, key, colon, rest] = m
  return (
    <span className="block whitespace-pre">
      {ws}
      {key && <span className="text-tok-prop">{key}</span>}
      {colon && <span className="text-faint">{colon}</span>}
      <JsonValue text={rest} />
    </span>
  )
}

function JsonValue({ text }: { text: string }) {
  if (/^"/.test(text)) {
    // Interpolations are where Clowd wired resources together; make them pop.
    const parts = text.split(/(\$\{[^}]*\})/)
    return (
      <span className="text-tok-value">
        {parts.map((p, i) =>
          p.startsWith('${') ? (
            <span key={i} className="text-tok-verb">
              {p}
            </span>
          ) : (
            p
          ),
        )}
      </span>
    )
  }
  if (/^-?\d/.test(text)) return <span className="text-tok-num">{text}</span>
  if (/^(true|false|null)/.test(text)) return <span className="text-tok-kw">{text}</span>
  return <span className="text-faint">{text}</span>
}
