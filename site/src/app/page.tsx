import Link from 'next/link'
import { ArrowRight } from 'lucide-react'
import { DidYouMean } from '@/components/DidYouMean'
import { Matrix } from '@/components/Matrix'
import { Code } from '@/components/Code'
import { CopyCommand } from '@/components/CopyCommand'
import { highlight, tokenClass } from '@/lib/clowd'
import { CHECK_MS, FULL_SOURCE, JSON_LINES, SOURCE_LINES, STEPS, TOTAL_RESOURCES } from '@/lib/demo'

export default function Home() {
  return (
    <main>
      {/* Hero */}
      <section className="mx-auto max-w-6xl px-4 pb-20 pt-20 sm:px-6 sm:pt-28">
        <h1 className="max-w-3xl text-[clamp(2.4rem,6vw,4.25rem)] font-medium leading-[1.02] tracking-[-0.04em]">
          Describe your cloud once.
          <br />
          <span className="text-mute">Deploy it to any of them.</span>
        </h1>
        <p className="mt-7 max-w-xl text-pretty text-[17px] leading-relaxed text-mute">
          Clowd is a small, English-like language for infrastructure. Write what you want in sentences, check it against
          every cloud you target, and deploy with OpenTofu.
        </p>
        <div className="mt-9 flex flex-col gap-4 sm:flex-row sm:items-center">
          <CopyCommand className="sm:w-auto" />
          <Link href="/docs/" className="group inline-flex items-center gap-1.5 text-sm text-mute transition hover:text-paper">
            Read the docs <ArrowRight className="size-3.5 transition group-hover:translate-x-0.5" aria-hidden />
          </Link>
        </div>

        <HeroPanel />
      </section>

      <Section
        label="Resources"
        title="Nouns, not products."
        subtitle="Clowd picks the service on each cloud."
        body="You write service, database or bucket. Clowd chooses the product, then fills in the subnets, roles, log groups and health checks nobody wants to write by hand."
      >
        <Grid>
          {KINDS.map((k) => (
            <Card key={k.title} code={k.code} title={k.title} text={k.text} />
          ))}
        </Grid>
      </Section>

      <Section
        label="Relationships"
        title="One line of wiring."
        subtitle="Instead of a page of IAM and security groups."
        body="A relationship generates the network path, the permissions and the environment variables your code reads. It never sees an ARN, and the variables are the same on every cloud."
      >
        <div className="overflow-hidden rounded-lg border border-line">
          {RELATIONS.map((r, i) => (
            <div key={r.src} className={`grid gap-6 p-6 md:grid-cols-[1fr_1.4fr] ${i ? 'border-t border-line' : ''}`}>
              <Line src={r.src} className="text-[15px]" />
              <div className="grid gap-5 text-sm sm:grid-cols-2">
                <div>
                  <p className="text-paper">On AWS</p>
                  <ul className="mt-2 space-y-1 text-mute">
                    {r.aws.map((a) => (
                      <li key={a}>{a}</li>
                    ))}
                  </ul>
                </div>
                <div>
                  <p className="text-paper">In the service</p>
                  <ul className="mt-2 space-y-1 font-mono text-[12.5px] text-mute">
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
        <div className="grid grid-cols-2 overflow-hidden rounded-lg border border-line md:grid-cols-4">
          {[
            [String(SOURCE_LINES), 'lines of Clowd'],
            [String(TOTAL_RESOURCES), 'AWS resources'],
            [String(JSON_LINES), 'lines of OpenTofu JSON'],
            [`${CHECK_MS} ms`, 'to check it'],
          ].map(([n, label], i) => (
            <div key={label} className={`p-6 ${i % 2 ? 'border-l border-line' : ''} ${i > 1 ? 'border-t border-line md:border-t-0' : ''} ${i === 2 ? 'md:border-l' : ''}`}>
              <p className="text-3xl font-medium tracking-tight">{n}</p>
              <p className="mt-1 text-sm text-mute">{label}</p>
            </div>
          ))}
        </div>
        <p className="mt-4 text-sm text-faint">examples/app.clowd, built for AWS. Timed on an Apple M2.</p>
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
      <section className="border-t border-line">
        <div className="mx-auto flex max-w-6xl flex-col items-center px-4 py-28 text-center sm:px-6">
          <h2 className="text-[clamp(2rem,4.5vw,3rem)] font-medium leading-[1.05] tracking-[-0.035em]">
            From sentences to infrastructure.
            <br />
            <span className="text-mute">Install in one line.</span>
          </h2>
          <CopyCommand className="mt-10" />
          <p className="mt-4 text-sm text-faint">Needs Go 1.26 or newer. Deploying also needs OpenTofu or Terraform.</p>
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
    <section className="border-t border-line">
      <div className="mx-auto max-w-6xl px-4 py-24 sm:px-6">
        <div className="grid gap-6 md:grid-cols-2">
          <h2 className="text-lg text-paper">{label}</h2>
          <div>
            <p className="text-[clamp(1.5rem,3vw,2rem)] font-medium leading-tight tracking-[-0.03em]">
              {title}
              <br />
              <span className="text-mute">{subtitle}</span>
            </p>
            <p className="mt-5 max-w-md text-pretty text-[15px] leading-relaxed text-mute">{body}</p>
          </div>
        </div>
        <div className="mt-14">{children}</div>
      </div>
    </section>
  )
}

function Grid({ children }: { children: React.ReactNode }) {
  // gap-px over a line-coloured background draws hairlines between cells.
  return <div className="grid gap-px overflow-hidden rounded-lg border border-line bg-line sm:grid-cols-2 lg:grid-cols-3">{children}</div>
}

function Card({ code, title, text }: { code: string; title: string; text: string }) {
  return (
    <div className="bg-ink p-6">
      <div className="flex h-28 items-center justify-center rounded-md border border-line bg-ink-2">
        <Line src={code} className="text-[14px]" />
      </div>
      <p className="mt-5 text-[15px] text-paper">{title}</p>
      <p className="mt-1 text-sm leading-relaxed text-mute">{text}</p>
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
    <div className="mt-20 grid overflow-hidden rounded-lg border border-line bg-ink-2 grid-cols-[minmax(0,1fr)] lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
      <div className="border-b border-line lg:border-b-0 lg:border-r">
        <div className="border-b border-line px-4 py-2.5 font-mono text-xs text-mute">app.clowd</div>
        <Code src={FULL_SOURCE} lineNumbers bare />
      </div>
      <div className="flex flex-col">
        <div className="border-b border-line px-4 py-2.5 font-mono text-xs text-mute">clowd build --target aws</div>
        <div className="px-5 pt-5">
          <p className="text-4xl font-medium tracking-tight">{TOTAL_RESOURCES}</p>
          <p className="mt-1 text-sm text-mute">AWS resources from {SOURCE_LINES} lines</p>
        </div>
        <ul className="mt-4 h-72 overflow-y-auto border-t border-line px-5 py-3 font-mono text-[12px] leading-[1.9] lg:h-auto lg:flex-1 lg:basis-0">
          {resources.map((r) => {
            const [type, name] = r.split('.')
            return (
              <li key={r} className="whitespace-nowrap">
                <span className="text-faint">{type}.</span>
                <span className="text-paper">{name}</span>
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
    <div className="overflow-hidden rounded-lg border border-line bg-ink-2 font-mono text-[13px] leading-[1.75]">
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
        <div className="absolute left-4 right-4 top-[8.6rem] max-w-md rounded-md border border-line-2 bg-ink-3 p-3.5 font-sans text-[13px] leading-relaxed shadow-2xl shadow-black sm:left-24 sm:right-auto">
          <p className="font-mono text-paper">service api</p>
          <div className="mt-2 grid grid-cols-[3rem_1fr] gap-y-1 text-mute">
            <span className="font-mono text-paper">aws</span>
            <span>
              ECS Fargate <span className="text-bad">✗</span> minimum must be at least 1
            </span>
            <span className="font-mono text-paper">gcp</span>
            <span>
              Cloud Run <span className="text-ok">✓</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
