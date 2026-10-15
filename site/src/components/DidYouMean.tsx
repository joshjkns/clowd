'use client'

import { useState } from 'react'
import { CornerDownLeft } from 'lucide-react'
import { check, TOY_FILE, type Diag } from '@/lib/toy'
import { highlight, tokenClass } from '@/lib/clowd'

const TRIES = ['api talks to db', 'api writes jobs', 'db connects to api', 'api reads upload', 'api reads and writes uploads']

const LINE_NO = TOY_FILE.split('\n').length

export function DidYouMean() {
  const [line, setLine] = useState(TRIES[0])
  const result = check(line)

  return (
    <div className="overflow-hidden border border-edge bg-page">
      <div className="border-b border-line px-4 py-2.5 font-mono text-xs text-mute">toy.clowd</div>
      <pre className="px-0 pt-3 font-mono text-[13px] leading-[1.75]">
        {/* Keeps the trailing empty line: fmt puts a blank line before relationships. */}
        {highlight(TOY_FILE).map((l, i) => (
          <span key={i} className="relative block min-h-[1.75em] pl-12 pr-4 opacity-60">
            <span className="absolute left-3 w-6 select-none text-right text-faint">{i + 1}</span>
            {l.map((t, j) => (
              <span key={j} className={tokenClass[t.kind]}>
                {t.text}
              </span>
            ))}
          </span>
        ))}
      </pre>
      <label className="relative flex items-center bg-page-2 py-1.5 pl-12 pr-4 font-mono text-[13px] leading-[1.75] ring-1 ring-inset ring-line-2 focus-within:ring-edge">
        <span className="absolute left-3 w-6 select-none text-right text-faint">{LINE_NO}</span>
        <span className="sr-only">Type a relationship</span>
        {/* The input is transparent over a highlighted copy, so it colours as you type. */}
        <span aria-hidden className="pointer-events-none absolute inset-y-0 left-12 right-4 flex items-center whitespace-pre">
          {highlight(line)[0].map((t, j) => (
            <span key={j} className={tokenClass[t.kind]}>
              {t.text}
            </span>
          ))}
        </span>
        <input
          value={line}
          onChange={(e) => setLine(e.target.value)}
          spellCheck={false}
          autoCapitalize="off"
          autoComplete="off"
          className="w-full bg-transparent text-transparent caret-fg outline-none"
        />
      </label>

      <div className="min-h-[9.5rem] border-t border-line px-4 py-4 font-mono text-[12.5px] leading-[1.7]" aria-live="polite">
        <div>
            {result.kind === 'empty' && <p className="italic text-faint"># type a relationship, like api connects to db</p>}
            {result.kind === 'ok' && result.note && (
              <p>
                <span className="font-bold text-tok-trait">warning</span>
                <span className="text-fg">: {result.note}</span>
              </p>
            )}
            {result.kind === 'ok' && !result.note && (
              <div>
                <p className="text-tok-cloud">✓ deployable to aws</p>
                <p className="mt-1 text-mute">
                  <span className="text-faint">gives the service </span>
                  {result.env.map((e) => (
                    <span key={e} className="mr-2 text-tok-value">
                      {e}
                    </span>
                  ))}
                </p>
              </div>
            )}
            {result.kind === 'diags' && result.diags.map((d, i) => <Diagnostic key={i} d={d} line={line} onFix={setLine} />)}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-line px-4 py-3 text-xs">
        <span className="mr-1 text-mute">Try</span>
        {TRIES.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setLine(t)}
            className={`rounded-md border px-2.5 py-1 font-mono transition ${line === t ? 'border-edge bg-fg text-page' : 'border-line-2 text-mute hover:border-edge hover:text-fg'}`}
          >
            {t}
          </button>
        ))}
      </div>
    </div>
  )
}

function Diagnostic({ d, line, onFix }: { d: Diag; line: string; onFix: (s: string) => void }) {
  const colour = d.severity === 'error' ? 'text-bad' : 'text-tok-trait'
  return (
    <div className="mb-3 last:mb-0">
      <p>
        <span className={`font-bold ${colour}`}>{d.severity}</span>
        <span className="font-semibold text-fg">: {d.message}</span>
      </p>
      <p className="text-faint">
        {' '}--&gt; <span className="text-mute">toy.clowd:{LINE_NO}:{d.from + 1}</span>
      </p>
      <p className="whitespace-pre text-faint">
        {'  |\n'}
        {String(LINE_NO).padStart(2)}
        {' | '}
        <span className="text-fg">{line}</span>
        {'\n  | '}
        <span className={colour}>{' '.repeat(d.from) + '^'.repeat(Math.max(1, d.to - d.from))}</span>
      </p>
      {d.help && (
        <p className="mt-0.5 flex flex-wrap items-center gap-x-3">
          <span>
            <span className="text-faint">{'  = '}</span>
            <span className="font-bold text-fg">help</span>
            <span className="text-fg">: {d.help}</span>
          </span>
          {d.fix && (
            <button
              type="button"
              onClick={() => onFix(d.fix!)}
              className="inline-flex items-center gap-1 rounded border border-edge px-1.5 py-0.5 text-[11px] text-fg transition hover:bg-fg hover:text-page"
            >
              <CornerDownLeft className="size-3" aria-hidden /> apply
            </button>
          )}
        </p>
      )}
    </div>
  )
}
