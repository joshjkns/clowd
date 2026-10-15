'use client'

import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion, useInView, useReducedMotion } from 'motion/react'
import { highlight, tokenClass } from '@/lib/clowd'

type Cloud = 'aws' | 'gcp' | 'azure'
const CLOUDS: Cloud[] = ['aws', 'gcp', 'azure']

// Products from internal/capability/matrix.go. Azure has no entries yet.
const ROWS: { src: string; aws: string; gcp: string }[] = [
  { src: 'service api', aws: 'ECS Fargate', gcp: 'Cloud Run' },
  { src: 'database db is postgres 16', aws: 'RDS', gcp: 'Cloud SQL' },
  { src: 'bucket uploads is private', aws: 'S3', gcp: 'Cloud Storage' },
  { src: 'load balancer web', aws: 'Application Load Balancer', gcp: 'Application Load Balancer' },
  { src: 'queue jobs is fifo', aws: 'SQS', gcp: 'Pub/Sub' },
]

export function Kinds() {
  const reduce = useReducedMotion()
  const root = useRef<HTMLDivElement>(null)
  const inView = useInView(root, { amount: 0.5 })
  const [cloud, setCloud] = useState<Cloud>('aws')
  const [touched, setTouched] = useState(false)

  useEffect(() => {
    if (!inView || touched || reduce) return
    const t = setInterval(() => setCloud((c) => CLOUDS[(CLOUDS.indexOf(c) + 1) % CLOUDS.length]), 2200)
    return () => clearInterval(t)
  }, [inView, touched, reduce])

  return (
    <div ref={root} className="overflow-hidden rounded-2xl border border-line bg-ink-2">
      <div className="flex items-center gap-3 border-b border-line px-4 py-2.5">
        <span className="font-mono text-xs text-mute">use</span>
        <div role="tablist" aria-label="Target cloud" className="flex rounded-lg bg-ink p-0.5">
          {CLOUDS.map((c) => (
            <button
              key={c}
              role="tab"
              aria-selected={cloud === c}
              onClick={() => {
                setTouched(true)
                setCloud(c)
              }}
              className={`relative rounded-md px-3 py-1 font-mono text-xs transition ${cloud === c ? 'text-ink' : 'text-mute hover:text-paper'}`}
            >
              {cloud === c && (
                <motion.span layoutId="cloud-pill" className="absolute inset-0 rounded-md bg-tok-cloud" transition={{ type: 'spring', bounce: 0.2, duration: 0.4 }} />
              )}
              <span className="relative font-semibold">{c}</span>
            </button>
          ))}
        </div>
      </div>
      <ul className="divide-y divide-line/60">
        {ROWS.map((r, i) => {
          const product = cloud === 'azure' ? null : r[cloud]
          return (
            <li key={r.src} className="grid grid-cols-1 items-center gap-1 px-4 py-3.5 sm:grid-cols-[1fr_auto] sm:gap-6">
              <code className="font-mono text-[13px]">
                {highlight(r.src)[0].map((t, j) => (
                  <span key={j} className={tokenClass[t.kind]}>
                    {t.text}
                  </span>
                ))}
              </code>
              <div className="relative h-6 overflow-hidden text-sm sm:min-w-[13rem] sm:text-right">
                <AnimatePresence mode="popLayout" initial={false}>
                  <motion.span
                    key={cloud + r.src}
                    initial={{ y: '110%', opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    exit={{ y: '-110%', opacity: 0 }}
                    transition={{ duration: 0.28, delay: i * 0.04, ease: [0.2, 0.7, 0.2, 1] }}
                    className={`block whitespace-nowrap ${product ? 'text-paper' : 'font-mono text-xs leading-6 text-faint'}`}
                  >
                    {product ?? '✗ not implemented yet'}
                  </motion.span>
                </AnimatePresence>
              </div>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
