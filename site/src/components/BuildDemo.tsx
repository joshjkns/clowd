'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { animate, motion, useInView, useReducedMotion } from 'motion/react'
import { FastForward, RotateCcw } from 'lucide-react'
import { highlight, tokenClass, type TokenKind } from '@/lib/clowd'
import { FULL_SOURCE, JSON_LINES, SOURCE_LINES, STEPS, TOTAL_RESOURCES } from '@/lib/demo'

const bg: Partial<Record<TokenKind, string>> = {
  cloud: 'bg-tok-cloud',
  kind: 'bg-tok-kind',
  trait: 'bg-tok-trait',
  prop: 'bg-tok-prop',
  num: 'bg-tok-num',
  verb: 'bg-tok-verb',
  kw: 'bg-tok-kw',
}

// Where each step ends in FULL_SOURCE, and which line it starts on.
const ENDS = STEPS.reduce<number[]>((acc, s) => [...acc, (acc.at(-1) ?? 0) + s.src.length], [])
const FIRST_LINE = STEPS.reduce<number[]>(
  (acc, s, i) => [...acc, i === 0 ? 0 : acc[i - 1] + STEPS[i - 1].src.split('\n').length - 1],
  [],
)
const TOTAL_LINES = FULL_SOURCE.split('\n').length

function delayFor(ch: string | undefined, atStepEnd: boolean) {
  if (atStepEnd) return 320
  if (ch === '\n') return 90
  if (ch === ' ') return 22
  return 11 + Math.random() * 16
}

export function BuildDemo() {
  const reduce = useReducedMotion()
  const root = useRef<HTMLDivElement>(null)
  const inView = useInView(root, { once: true, amount: 0.35 })
  const [typed, setTyped] = useState(0)
  const [run, setRun] = useState(0)
  const done = typed >= FULL_SOURCE.length

  useEffect(() => {
    if (reduce) setTyped(FULL_SOURCE.length)
  }, [reduce])

  useEffect(() => {
    if (!inView || done || reduce) return
    const t = setTimeout(() => setTyped((n) => n + 1), delayFor(FULL_SOURCE[typed - 1], ENDS.includes(typed)))
    return () => clearTimeout(t)
  }, [inView, typed, done, reduce, run])

  const finished = ENDS.filter((e) => e <= typed).length
  const active = Math.min(finished, STEPS.length - 1)
  const resources = useMemo(
    () => STEPS.slice(0, finished).flatMap((s) => s.resources.map((r) => ({ r, tint: s.tint }))),
    [finished],
  )

  const lines = highlight(FULL_SOURCE.slice(0, typed))
  const activeFrom = FIRST_LINE[active]
  const activeTo = activeFrom + STEPS[active].src.trimEnd().split('\n').length

  const list = useRef<HTMLOListElement>(null)
  useEffect(() => {
    const el = list.current
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: reduce ? 'auto' : 'smooth' })
  }, [resources.length, reduce])

  return (
    <div ref={root} className="grid overflow-hidden rounded-2xl border border-line bg-ink-2 shadow-[0_30px_120px_-40px_rgba(94,200,255,0.25)] grid-cols-[minmax(0,1fr)] lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
      {/* Editor */}
      <div className="relative border-b border-line lg:border-b-0 lg:border-r">
        <div className="flex items-center gap-3 border-b border-line px-4 py-2.5 font-mono text-xs text-mute">
          <span className="flex gap-1.5">
            <span className="size-2.5 rounded-full bg-[#ff5d5d]/70" />
            <span className="size-2.5 rounded-full bg-tok-trait/70" />
            <span className="size-2.5 rounded-full bg-tok-cloud/70" />
          </span>
          app.clowd
          <span className="ml-auto text-faint">{lines.filter((l) => l.some((t) => t.text.trim())).length} / {SOURCE_LINES} lines</span>
        </div>
        <pre className="overflow-x-auto py-3 font-mono text-[12.5px] leading-[1.75] sm:text-[13px]" style={{ minHeight: `calc(${TOTAL_LINES} * 1.75em + 1.5rem)` }}>
          <code>
            {lines.map((line, i) => {
              const on = !done && i >= activeFrom && i < activeTo
              return (
                <span key={i} className="relative block min-h-[1.75em] pl-12 pr-4">
                  <span
                    className={`absolute left-0 top-0 h-full w-[3px] transition-opacity duration-300 ${bg[STEPS[active].tint]} ${on ? 'opacity-100' : 'opacity-0'}`}
                  />
                  <span className="absolute left-3 w-6 select-none text-right text-faint">{i + 1}</span>
                  {line.map((t, j) => (
                    <span key={j} className={tokenClass[t.kind]}>
                      {t.text}
                    </span>
                  ))}
                  {i === lines.length - 1 && !done && (
                    <span className="inline-block h-[1.1em] w-[0.55em] translate-y-[0.2em] bg-paper/90" />
                  )}
                </span>
              )
            })}
          </code>
        </pre>
      </div>

      {/* Output */}
      <div className="flex min-h-[22rem] flex-col">
        <div className="flex items-center gap-3 border-b border-line px-4 py-2.5 font-mono text-xs text-mute">
          <span className="text-tok-cloud">aws</span>
          <span className="text-faint">→</span>
          .clowd/build/aws/main.tf.json
        </div>
        <div className="flex items-end gap-3 px-5 pb-3 pt-5">
          <Counter value={resources.length} className="font-display text-6xl font-bold leading-none tracking-tight text-paper" />
          <div className="pb-1 text-sm leading-tight text-mute">
            AWS resources
            <br />
            <span className="text-faint">from {lines.filter((l) => l.some((t) => t.text.trim())).length} lines of Clowd</span>
          </div>
        </div>
        <ol ref={list} className="scrollbar-none relative h-64 flex-1 overflow-y-auto px-5 pb-4 font-mono text-[12px] leading-[1.9] lg:h-auto lg:max-h-[27rem]">
          {resources.map(({ r, tint }, i) => {
            const [type, name] = r.split('.')
            return (
              <motion.li
                key={`${run}-${r}`}
                initial={reduce ? false : { opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.25, delay: reduce ? 0 : (i % 12) * 0.025 }}
                className="flex items-center gap-2.5 whitespace-nowrap"
              >
                <span className={`size-1.5 shrink-0 rounded-full ${bg[tint]}`} />
                <span className="text-mute">{type}</span>
                <span className="-ml-2 text-faint">.</span>
                <span className="-ml-2.5 text-paper">{name}</span>
              </motion.li>
            )
          })}
          {resources.length === 0 && <li className="italic text-faint"># nothing yet. start typing.</li>}
        </ol>
        <div className="flex items-center gap-3 border-t border-line px-4 py-2.5 font-mono text-xs">
          {done ? (
            <span className="text-tok-cloud">
              ✓ <span className="text-mute">{TOTAL_RESOURCES} resources · {JSON_LINES} lines of OpenTofu JSON</span>
            </span>
          ) : (
            <span className="text-faint">building…</span>
          )}
          <button
            type="button"
            onClick={() => {
              if (done) {
                setTyped(0)
                setRun((n) => n + 1)
              } else setTyped(FULL_SOURCE.length)
            }}
            className="ml-auto flex items-center gap-1.5 rounded-md border border-line px-2.5 py-1 text-mute transition hover:border-mute hover:text-paper"
          >
            {done ? <RotateCcw className="size-3.5" aria-hidden /> : <FastForward className="size-3.5" aria-hidden />}
            {done ? 'replay' : 'skip'}
          </button>
        </div>
      </div>
    </div>
  )
}

/** A number that rolls to its new value instead of jumping. */
export function Counter({ value, className, decimals = 0 }: { value: number; className?: string; decimals?: number }) {
  const ref = useRef<HTMLSpanElement>(null)
  const from = useRef(value)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const controls = animate(from.current, value, {
      duration: 0.5,
      ease: 'easeOut',
      onUpdate: (v) => (el.textContent = v.toFixed(decimals)),
    })
    from.current = value
    return () => controls.stop()
  }, [value, decimals])
  return (
    <span ref={ref} className={`tabular-nums ${className ?? ''}`}>
      {value.toFixed(decimals)}
    </span>
  )
}
