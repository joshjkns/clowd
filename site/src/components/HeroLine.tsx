'use client'

import { useEffect, useState } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { highlight, tokenClass } from '@/lib/clowd'

// Each line and what it really generates (checked against the AWS build).
const LINES: { src: string; says: React.ReactNode }[] = [
  {
    src: 'api connects to db',
    says: (
      <>
        A security-group rule, an IAM grant for a password the cloud generates, and{' '}
        <Env>DB_HOST</Env> <Env>DB_PORT</Env> <Env>DB_NAME</Env> <Env>DB_USER</Env> <Env>DB_PASSWORD</Env> in the
        service.
      </>
    ),
  },
  {
    src: 'api reads and writes uploads',
    says: (
      <>
        An IAM policy scoped to one bucket, and <Env>UPLOADS_BUCKET</Env> in the service.
      </>
    ),
  },
  {
    src: 'database db is postgres 16',
    says: <>RDS on AWS. Cloud SQL on GCP. You don’t pick the product; you describe the thing.</>,
  },
  {
    src: 'api sends to jobs',
    says: (
      <>
        Publish rights on the queue, and <Env>JOBS_QUEUE_URL</Env> in the service.
      </>
    ),
  },
]

function Env({ children }: { children: React.ReactNode }) {
  return <code className="rounded bg-ink-3 px-1 py-0.5 font-mono text-[0.85em] text-tok-value">{children}</code>
}

export function HeroLine() {
  const reduce = useReducedMotion()
  const [index, setIndex] = useState(0)
  const [shown, setShown] = useState(LINES[0].src.length)
  const [phase, setPhase] = useState<'typing' | 'holding' | 'deleting'>('holding')

  useEffect(() => {
    if (reduce) return
    const src = LINES[index].src
    let t: ReturnType<typeof setTimeout>
    if (phase === 'holding') {
      t = setTimeout(() => setPhase('deleting'), 3600)
    } else if (phase === 'deleting') {
      if (shown === 0) {
        setIndex((i) => (i + 1) % LINES.length)
        setPhase('typing')
      } else t = setTimeout(() => setShown((n) => n - 1), 14)
    } else if (shown < src.length) {
      t = setTimeout(() => setShown((n) => n + 1), 38 + Math.random() * 40)
    } else {
      setPhase('holding')
    }
    return () => clearTimeout(t)
  }, [phase, shown, index, reduce])

  const line = LINES[index]
  const tokens = highlight(line.src.slice(0, shown))[0]

  return (
    <div>
      <h1 className="sr-only">Clowd: write cloud infrastructure in sentences</h1>
      <div
        aria-hidden
        className="min-h-[2.3em] font-mono text-[clamp(1.9rem,5.2vw,3.8rem)] lg:min-h-[1.15em] font-bold leading-[1.1] tracking-tight"
      >
        {tokens.map((t, i) => (
          <span key={i} className={tokenClass[t.kind]}>
            {t.text}
          </span>
        ))}
        <span className="ml-1 inline-block h-[0.9em] w-[0.45em] translate-y-[0.1em] animate-blink bg-tok-verb" />
      </div>
      <div className="mt-5 min-h-[5.5rem] max-w-2xl text-lg leading-relaxed text-mute sm:min-h-[4rem]">
        <AnimatePresence mode="wait">
          {phase !== 'typing' && (
            <motion.p
              key={index}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4, transition: { duration: 0.12 } }}
              transition={{ duration: 0.3 }}
            >
              <span className="mr-2 font-mono text-sm text-faint">#</span>
              {line.says}
            </motion.p>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}
