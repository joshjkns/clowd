import Link from 'next/link'
import { ArrowRight } from 'lucide-react'
import { DidYouMean } from '@/components/DidYouMean'
import { Matrix } from '@/components/Matrix'
import { Code } from '@/components/Code'
import { CopyCommand } from '@/components/CopyCommand'
import { CloudField } from '@/components/CloudField'
import { highlight, tokenClass } from '@/lib/clowd'
import { CHECK_MS, FULL_SOURCE, JSON_LINES, SOURCE_LINES, STEPS, TOTAL_RESOURCES } from '@/lib/demo'

export default function Home() {
  return (
    <main className="relative isolate">
      {/* Hero */}
      <CloudField className="h-[44rem] [mask-image:linear-gradient(to_bottom,black_55%,transparent)]" />
      {/* Keeps the hero copy readable over the field. */}
      <div aria-hidden className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[40rem] bg-[radial-gradient(ellipse_45%_55%_at_50%_42%,var(--color-page)_40%,transparent)]" />
      <section className="mx-auto max-w-6xl px-4 pb-24 pt-20 text-center sm:px-6 sm:pt-28">
        <h1 className="font-display text-[clamp(5rem,17vw,11rem)] font-semibold leading-[0.85] tracking-[-0.065em]">clowd</h1>
        <p className="mx-auto mt-12 max-w-3xl text-balance font-display text-[clamp(1.45rem,3.2vw,2.2rem)] leading-[1.3] tracking-[-0.02em]">
          <span className="box-decoration-clone bg-fg px-2 text-page">Describe your cloud once,</span> deploy it to any of them
        </p>
        <p className="mx-auto mt-5 max-w-xl text-pretty text-[16px] leading-relaxed text-mute">
          A small, English-like language for infrastructure. Write what you want in sentences, check it against every
          cloud you target, and deploy with OpenTofu.
        </p>
        <div className="mt-10 flex flex-col items-center gap-3">
          <CopyCommand />
          <Link href="/docs/#status" className="bg-fg px-2 py-0.5 font-mono text-[12px] text-page transition hover:bg-accent">
            Beta: the language may change before 1.0
          </Link>
        </div>
        <Link href="/docs/" className="group mt-8 inline-flex items-center gap-1.5 font-mono text-[13px] text-fg underline decoration-line-2 underline-offset-4 transition hover:decoration-fg">
          Read the docs <ArrowRight className="size-3.5 transition group-hover:translate-x-0.5" aria-hidden />
        </Link>

        <HeroPanel />
      </section>

      <Section
        label="Resources"
        title="Nouns, not products."
        subtitle="Clowd picks the service on each cloud."
        body="You write service, database or bucket. Clowd chooses the product, then fills in the subnets, roles, log groups and health checks nobody wants to write by hand."
      >
        <div className="grid gap-x-8 gap-y-14 sm:grid-cols-2 lg:grid-cols-3">
          {KINDS.map((k) => (
            <Card key={k.title} code={k.code} title={k.title} text={k.text} />
          ))}
        </div>
      </Section>

      <Section
        label="Relationships"
        title="One line of wiring."
        subtitle="Instead of a page of IAM and security groups."
        body="A relationship generates the network path, the permissions and the environment variables your code reads. It never sees an ARN, and the variables are the same on every cloud."
      >
        <div className="border border-edge">
          {RELATIONS.map((r, i) => (
            <div key={r.src} className={`grid gap-6 p-6 md:grid-cols-[1fr_1.4fr] ${i ? 'border-t border-line' : ''}`}>
              <Line src={r.src} className="text-[15px]" />
              <div className="grid gap-5 text-sm sm:grid-cols-2">
                <div>
                  <p className="font-mono text-[11px] uppercase tracking-[0.12em] text-faint">On AWS</p>
                  <ul className="mt-2 space-y-1 text-fg">
                    {r.aws.map((a) => (
                      <li key={a}>{a}</li>
                    ))}
                  </ul>
                </div>
                <div>
                  <p className="font-mono text-[11px] uppercase tracking-[0.12em] text-faint">In the service</p>
                  <ul className="mt-2 space-y-1 font-mono text-[12.5px] text-tok-value">
                    {r.env.map((e) => (
                      <li key={e}>{e}</li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section
        label="Checking"
        title="Know before you deploy."
        subtitle={`Every cloud, in ${CHECK_MS} milliseconds.`}
        body="clowd check reads the same capability matrix the compiler uses. ✓ maps straight across, ⚠ works with a caveat, ✗ won't deploy and says why. No account, no network, no waiting for a plan."
      >
        <Matrix />
      </Section>

      <Section
        label="Errors"
        title="One way to say it."
        subtitle="And the fix, when you guess wrong."
        body="Each relationship has exactly one phrasing. A wrong guess is an error that names the right words, and the rest of the file keeps checking. Try a few below."
      >
        <DidYouMean />
      </Section>

      <Section
        label="Output"
        title="Plain OpenTofu underneath."
        subtitle="No new engine, no lock-in."
        body="clowd build writes ordinary OpenTofu JSON. OpenTofu plans, applies and keeps state, and you can read the output, diff it, or take it and leave."
      >
        {/* gap-px over an edge-coloured background draws the lines between cells. */}
        <div className="grid grid-cols-2 gap-px border border-edge bg-edge md:grid-cols-4">
          {[
            [String(SOURCE_LINES), 'lines of Clowd'],
            [String(TOTAL_RESOURCES), 'AWS resources'],
            [String(JSON_LINES), 'lines of OpenTofu JSON'],
            [`${CHECK_MS} ms`, 'to check it'],
          ].map(([n, label]) => (
            <div key={label} className="bg-page p-6">
              <p className="font-display text-4xl font-medium tracking-[-0.03em]">{n}</p>
              <p className="mt-1 text-sm text-mute">{label}</p>
            </div>
          ))}
        </div>
        <p className="mt-4 text-center font-mono text-[12px] text-faint">examples/app.clowd, built for AWS. Timed on an Apple M2.</p>
      </Section>

      <Section
        label="Editor"
        title="Your editor speaks it too."
        subtitle="Diagnostics, hovers, completion, formatting."
        body="The language server ships in the same binary as the CLI, so the editor and clowd check never disagree. Hover a resource to see what it becomes on each cloud."
      >
        <HoverMock />
      </Section>

      {/* Close */}
      <section className="relative isolate overflow-hidden">
        <CloudField className="h-full [mask-image:radial-gradient(ellipse_at_center,transparent_20%,black_75%)]" />
        <div className="mx-auto flex max-w-6xl flex-col items-center px-4 py-32 text-center sm:px-6">
          <h2 className="font-display text-[clamp(2.2rem,5vw,3.5rem)] font-medium leading-[1.05] tracking-[-0.04em]">
            From sentences to <span className="box-decoration-clone bg-fg px-2 text-page">infrastructure.</span>
          </h2>
          <p className="mt-3 font-display text-xl text-mute">Install in one line.</p>
          <CopyCommand className="mt-10" />
          <p className="mt-4 font-mono text-[12px] text-faint">Needs Go 1.26 or newer. Deploying also needs OpenTofu or Terraform.</p>
        </div>
      </section>
    </main>
  )
}

const KINDS = [
  { code: 'service api', title: 'Services', text: 'Containers. ECS Fargate on AWS, Cloud Run on GCP.' },
  { code: 'database db is postgres 16', title: 'Databases', text: 'Postgres or MySQL. RDS on AWS, Cloud SQL on GCP.' },
  { code: 'bucket uploads is private', title: 'Buckets', text: 'Object storage. S3 on AWS, Cloud Storage on GCP.' },
  { code: 'load balancer web', title: 'Load balancers', text: 'HTTPS with certificates, routed by path.' },
  { code: 'queue jobs is fifo', title: 'Queues', text: 'SQS on AWS, Pub/Sub on GCP, with dead letters.' },
  { code: 'on aws', title: 'Escape hatches', text: 'Raw provider settings when the abstraction runs out.' },
]

const RELATIONS = [
  {
    src: 'api connects to db',
    aws: ['Security-group ingress rule', 'IAM grant to read the generated password'],
    env: ['DB_HOST', 'DB_PORT', 'DB_NAME', 'DB_USER', 'DB_PASSWORD'],
  },
  {
    src: 'api reads and writes uploads',
    aws: ['IAM policy scoped to the one bucket'],
    env: ['UPLOADS_BUCKET'],
  },
  {
    src: 'api sends to jobs',
    aws: ['IAM permission to publish to the queue'],
    env: ['JOBS_QUEUE_URL'],
  },
]

function Section({
  label,
  title,
  subtitle,
  body,
  children,
}: {
  label: string
  title: string
  subtitle: string
  body: string
  children: React.ReactNode
}) {
  return (
    <section>
      <div className="mx-auto max-w-6xl px-4 py-24 sm:px-6">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="inline-block bg-fg px-2 py-0.5 font-mono text-[12px] uppercase tracking-[0.12em] text-page">{label}</h2>
          <p className="mt-6 text-balance font-display text-[clamp(1.9rem,4vw,2.75rem)] font-medium leading-[1.05] tracking-[-0.035em]">{title}</p>
          <p className="mt-2 text-balance font-display text-[clamp(1.15rem,2.2vw,1.45rem)] tracking-[-0.015em] text-mute">{subtitle}</p>
          <p className="mx-auto mt-5 max-w-lg text-pretty text-[15px] leading-relaxed text-mute">{body}</p>
        </div>
        <div className="mt-14">{children}</div>
      </div>
    </section>
  )
}

function Card({ code, title, text }: { code: string; title: string; text: string }) {
  return (
    <div className="text-center">
      <p className="font-display text-xl font-medium tracking-[-0.02em]">{title}</p>
      <p className="mx-auto mt-1.5 max-w-xs text-sm leading-relaxed text-mute">{text}</p>
      <div className="mt-5 flex h-28 items-center justify-center border border-edge px-4">
        <Line src={code} className="text-[14px]" />
      </div>
    </div>
  )
}

function Line({ src, className = '' }: { src: string; className?: string }) {
  return (
    <code className={`font-mono ${className}`}>
      {highlight(src)[0].map((t, i) => (
        <span key={i} className={tokenClass[t.kind]}>
          {t.text}
        </span>
      ))}
    </code>
  )
}

function HeroPanel() {
  const resources = STEPS.flatMap((s) => s.resources)
  return (
    <div className="mt-20 grid overflow-hidden border border-edge bg-page text-left grid-cols-[minmax(0,1fr)] lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
      <div className="border-b border-line lg:border-b-0 lg:border-r">
        <div className="border-b border-line px-4 py-2.5 font-mono text-xs text-mute">app.clowd</div>
        <Code src={FULL_SOURCE} lineNumbers bare />
      </div>
      <div className="flex flex-col">
        <div className="border-b border-line px-4 py-2.5 font-mono text-xs text-mute">clowd build --target aws</div>
        <div className="px-5 pt-5">
          <p className="font-display text-5xl font-medium tracking-[-0.04em]">{TOTAL_RESOURCES}</p>
          <p className="mt-1 text-sm text-mute">AWS resources from {SOURCE_LINES} lines</p>
        </div>
        <ul className="mt-4 h-72 overflow-y-auto border-t border-line px-5 py-3 font-mono text-[12px] leading-[1.9] lg:h-auto lg:flex-1 lg:basis-0">
          {resources.map((r) => {
            const [type, name] = r.split('.')
            return (
              <li key={r} className="whitespace-nowrap">
                <span className="text-faint">{type}.</span>
                <span className="text-fg">{name}</span>
              </li>
            )
          })}
        </ul>
      </div>
    </div>
  )
}

function HoverMock() {
  return (
    <div className="overflow-hidden border border-edge bg-page font-mono text-[13px] leading-[1.75]">
      <div className="border-b border-line px-4 py-2.5 text-xs text-mute">app.clowd</div>
      <div className="relative px-4 py-4 pb-40 sm:pb-32">
        <Line src="use aws in eu-west-2" />
        <br />
        <Line src="use gcp in europe-west2" />
        <br />
        <br />
        <Line src="service api" />
        <br />
        <Line src="  scale 0 to 5" className="whitespace-pre" />
        <div className="absolute left-4 right-4 top-[8.6rem] max-w-md border border-edge bg-page p-3.5 font-sans text-[13px] leading-relaxed shadow-[4px_4px_0_0_var(--color-edge)] sm:left-24 sm:right-auto">
          <p className="font-mono text-fg">service api</p>
          <div className="mt-2 grid grid-cols-[3rem_1fr] gap-y-1 text-mute">
            <span className="font-mono text-fg">aws</span>
            <span>
              ECS Fargate <span className="text-bad">✗</span> minimum must be at least 1
            </span>
            <span className="font-mono text-fg">gcp</span>
            <span>
              Cloud Run <span className="text-ok">✓</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
