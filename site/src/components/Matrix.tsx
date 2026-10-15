import { Code } from './Code'

export const MATRIX_SRC = `use aws
use gcp

service api
  image ghcr.io/me/api:latest
  scale 0 to 5

queue jobs is fifo

api sends to jobs
`

const MARK: Record<string, string> = { '✓': 'text-ok', '⚠': 'text-warn', '✗': 'text-bad' }

// Real `clowd check` output for MATRIX_SRC.
const ROWS: { label: string; aws: string; gcp: string; note?: [string, string] }[] = [
  { label: 'service api', aws: '✗', gcp: '✓', note: ['aws', "ECS Fargate can't start a task for an incoming request, so the minimum must be at least 1"] },
  { label: 'queue jobs', aws: '✓', gcp: '⚠', note: ['gcp', 'Pub/Sub has no FIFO queues; this uses an ordered, exactly-once subscription, which limits throughput'] },
]

export function Matrix() {
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
      <Code src={MATRIX_SRC} title="app.clowd" lineNumbers />
      <div className="overflow-x-auto border border-edge bg-page px-4 py-3.5 font-mono text-[12.5px] leading-[1.75]">
        <p>
          <span className="select-none text-faint">$ </span>
          <span className="text-fg">clowd check</span>
        </p>
        <p className="text-fg">
          app.clowd <span className="text-faint">→</span> <span className="text-fg">aws</span>,{' '}
          <span className="text-fg">gcp</span>
        </p>
        <div className="mt-2 grid grid-cols-[minmax(0,1fr)_2.25rem_2.25rem] gap-x-1 sm:grid-cols-[9rem_2.25rem_2.25rem_minmax(0,1fr)] text-mute">
          <span className="font-semibold text-fg">Resources</span>
          <span className="font-semibold text-fg">aws</span>
          <span className="font-semibold text-fg">gcp</span>
          <span className="hidden sm:block" />
          {ROWS.map((r) => (
            <div key={r.label} className="contents">
              <span className="pl-2">{r.label}</span>
              <span className={MARK[r.aws]}>{r.aws}</span>
              <span className={MARK[r.gcp]}>{r.gcp}</span>
              <span className="col-span-3 pb-2 pl-2 text-[11.5px] leading-snug sm:col-span-1 sm:pl-0">
                <span className="text-fg">{r.note![0]}:</span> {r.note![1]}
              </span>
            </div>
          ))}
          <span className="col-span-3 mt-2 font-semibold text-fg sm:col-span-4">Relationships</span>
          <span className="pl-2">api sends to jobs</span>
          <span className="text-ok">✓</span>
          <span className="text-ok">✓</span>
          <span className="hidden sm:block" />
        </div>
        <p className="mt-3 text-faint">
          <span className="text-ok">✓</span> supported <span className="text-warn">⚠</span> emulated{' '}
          <span className="text-bad">✗</span> unavailable – not applicable
        </p>
        <p>
          <span className="text-ok">✓</span> <span className="text-fg">deployable to gcp</span>
          <span className="text-faint"> · </span>
          <span className="text-bad">✗</span> <span className="text-fg">not deployable to aws</span>
        </p>
      </div>
    </div>
  )
}
