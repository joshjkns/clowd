import Link from 'next/link'
import { ArrowRight } from 'lucide-react'
import { HeroLine } from '@/components/HeroLine'
import { BuildDemo } from '@/components/BuildDemo'
import { SpeedTerminal } from '@/components/SpeedTerminal'
import { Kinds } from '@/components/Kinds'
import { DidYouMean } from '@/components/DidYouMean'
import { Matrix } from '@/components/Matrix'
import { Code } from '@/components/Code'
import { CopyCommand } from '@/components/CopyCommand'
import { REPO } from '@/components/Nav'
import { CHECK_MS, SOURCE_LINES, TOTAL_RESOURCES } from '@/lib/demo'

export default function Home() {
  return (
    <main>
      {/* Hero */}
      <section className="grid-bg relative">
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-gradient-to-b from-transparent to-ink" />
        <div className="relative mx-auto max-w-6xl px-4 pb-16 pt-16 sm:px-6 sm:pt-24">
          <Link
            href="/docs/#status"
            className="mb-10 inline-flex items-center gap-2 rounded-full border border-line bg-ink/80 px-3 py-1 font-mono text-xs text-mute transition hover:border-mute hover:text-paper"
          >
            <span className="size-1.5 rounded-full bg-tok-cloud" />
            early · deploys to aws today, gcp next
          </Link>
          <HeroLine />
          <p className="mt-6 max-w-2xl text-pretty text-lg leading-relaxed text-paper/85">
            Clowd is a small, English-like language for cloud infrastructure. Write what you want in sentences; Clowd
            checks it against every cloud you target and compiles it to{' '}
            <a href="https://opentofu.org" className="underline decoration-line underline-offset-4 hover:decoration-mute">
              OpenTofu
            </a>
            .
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:items-center">
            <CopyCommand className="sm:w-auto" />
            <Link
              href="/docs/"
              className="group inline-flex items-center justify-center gap-2 rounded-xl bg-paper px-5 py-3 text-sm font-semibold text-ink transition hover:bg-white"
            >
              Read the docs
              <ArrowRight className="size-4 transition group-hover:translate-x-0.5" aria-hidden />
            </Link>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 sm:px-6">
        <BuildDemo />
        <p className="mt-4 text-center font-mono text-xs text-faint">
          # examples/app.clowd, built for real. {SOURCE_LINES} lines in, {TOTAL_RESOURCES} AWS resources out.
        </p>
      </section>

      {/* Speed */}
      <Section
        id="fast"
        tag="fast"
        title={
          <>
            <span className="text-tok-num">{CHECK_MS} ms</span> from sentence to verdict.
          </>
        }
        intro={
          <>
            <code>clowd check</code> parses, resolves and evaluates your file against every target cloud, locally, with no
            credentials and no network. That’s fast enough to run on every keystroke, which is what the editor does.
            Then <code>clowd build</code> writes plain OpenTofu JSON that you can read, diff, or take with you.
          </>
        }
      >
        <SpeedTerminal />
      </Section>

      {/* Nouns */}
      <Section
        id="nouns"
        tag="nouns, not products"
        title="Say what it is. Not what AWS calls it."
        intro={
          <>
            You write <K>service</K>, <K>database</K>, <K>bucket</K>, <K>queue</K>. Clowd picks the product per cloud and
            fills in the hundred settings nobody wants to think about: subnets, roles, log groups, health checks.
          </>
        }
        aside
      >
        <Kinds />
      </Section>

      {/* One phrasing */}
      <Section
        id="phrasing"
        tag="one way to say it"
        title="Guess wrong, get the right words back."
        intro={
          <>
            Each relationship has exactly one phrasing: <V>connects to</V>, never <em>links to</em>, <em>talks to</em> or{' '}
            <em>uses</em>. A wrong guess is an error that names the fix, and the rest of the file keeps checking as if
            you’d written it. Go on, break it.
          </>
        }
      >
        <DidYouMean />
      </Section>

      {/* Matrix */}
      <Section
        id="matrix"
        tag="know before you deploy"
        title="Find out which cloud says no before it does."
        intro={
          <>
            <span className="text-ok">✓</span> maps straight across. <span className="text-warn">⚠</span> works, with a
            caveat worth reading. <span className="text-bad">✗</span> won’t deploy, and here’s why. All from the same
            capability matrix the compiler uses, so the check and the build never disagree.
          </>
        }
      >
        <Matrix />
      </Section>

      {/* Escape hatch */}
      <Section
        id="escape"
        tag="escape hatches"
        title="Abstractions leak. Here’s the bucket."
        intro={
          <>
            When the portable words aren’t enough, an <code>on aws</code> block passes raw provider arguments through.
            Keys are checked against the real resource, so a typo fails in <code>check</code>, not halfway through an
            apply.
          </>
        }
        aside
      >
        <div className="grid gap-4">
          <Code
            title="app.clowd"
            src={`service api
  image ghcr.io/me/api:latest
  scale 2 to 10
  on aws
    capacity_provider_strategy
      capacity_provider FARGATE_SPOT
      weight 1`}
          />
          <ul className="grid gap-2 text-sm text-mute sm:grid-cols-3">
            <Fact>Raw keys look different on purpose</Fact>
            <Fact>Only applied when lowering to that cloud</Fact>
            <Fact>Wiring Clowd owns (subnets, roles) stays locked</Fact>
          </ul>
        </div>
      </Section>

      {/* Editor */}
      <Section
        id="editor"
        tag="editor"
        title="Your editor already speaks it."
        intro={
          <>
            <code>clowd lsp</code> ships in the same binary. The VS Code extension gives you live diagnostics, completion
            that only offers what the checker accepts, formatting, and hovers that say what each line becomes.
          </>
        }
      >
        <HoverMock />
      </Section>

      {/* Close */}
      <section className="mx-auto max-w-6xl px-4 py-28 sm:px-6">
        <div className="relative overflow-hidden rounded-3xl border border-line bg-ink-2 px-6 py-14 sm:px-12">
          <div className="grid-bg pointer-events-none absolute inset-0 opacity-60" />
          <div className="relative">
            <p className="font-mono text-[clamp(1.6rem,4.5vw,3.2rem)] font-bold leading-tight tracking-tight">
              <span className="text-tok-name">you</span> <span className="text-tok-verb">reads</span>{' '}
              <span className="text-tok-name">docs</span>
              <span className="ml-1 inline-block h-[0.9em] w-[0.45em] translate-y-[0.1em] animate-blink bg-tok-verb" />
            </p>
            <p className="mt-3 font-mono text-sm text-faint"># error: `you` is a person, not a service. we’ll allow it.</p>
            <div className="mt-8 flex flex-col gap-3 sm:flex-row">
              <CopyCommand />
              <Link
                href="/docs/"
                className="inline-flex items-center justify-center gap-2 rounded-xl bg-paper px-5 py-3 text-sm font-semibold text-ink transition hover:bg-white"
              >
                Get started <ArrowRight className="size-4" aria-hidden />
              </Link>
              <a
                href={REPO}
                className="inline-flex items-center justify-center rounded-xl border border-line px-5 py-3 text-sm text-mute transition hover:border-mute hover:text-paper"
              >
                Star on GitHub
              </a>
            </div>
          </div>
        </div>
      </section>
    </main>
  )
}

function Section({
  id,
  tag,
  title,
  intro,
  aside = false,
  children,
}: {
  id: string
  tag: string
  title: React.ReactNode
  intro: React.ReactNode
  aside?: boolean
  children: React.ReactNode
}) {
  const text = (
    <div className={aside ? 'lg:sticky lg:top-24' : 'max-w-2xl'}>
      <p className="font-mono text-sm italic text-tok-comment"># {tag}</p>
      <h2 className="mt-3 text-balance font-display text-[clamp(1.9rem,4vw,2.9rem)] font-bold leading-[1.05] tracking-tight">
        {title}
      </h2>
      <p className="mt-5 text-pretty leading-relaxed text-mute [&_code]:font-mono [&_code]:text-[0.9em] [&_code]:text-tok-value [&_em]:not-italic [&_em]:text-paper/80 [&_em]:line-through [&_em]:decoration-bad/70">
        {intro}
      </p>
    </div>
  )
  return (
    <section id={id} className="mx-auto max-w-6xl scroll-mt-20 px-4 pt-28 sm:px-6 sm:pt-36">
      {aside ? (
        <div className="grid items-start gap-10 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:gap-14">
          {text}
          {children}
        </div>
      ) : (
        <>
          {text}
          <div className="mt-10">{children}</div>
        </>
      )}
    </section>
  )
}

function K({ children }: { children: React.ReactNode }) {
  return <span className="font-mono text-[0.92em] font-semibold text-tok-kind">{children}</span>
}

function V({ children }: { children: React.ReactNode }) {
  return <span className="font-mono text-[0.92em] font-semibold text-tok-verb">{children}</span>
}

function Fact({ children }: { children: React.ReactNode }) {
  return (
    <li className="rounded-lg border border-line/70 bg-ink-2/60 px-3 py-2.5 leading-snug">
      <span className="mr-1.5 text-tok-prop">›</span>
      {children}
    </li>
  )
}

function HoverMock() {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-ink-2 font-mono text-[13px] leading-[1.75]">
      <div className="border-b border-line px-4 py-2.5 text-xs text-mute">app.clowd — Visual Studio Code</div>
      <div className="relative px-4 py-4 pb-44 sm:pb-36">
        <p>
          <span className="text-tok-kw">use</span> <span className="text-tok-cloud">aws</span>{' '}
          <span className="text-tok-kw">in</span> <span className="text-tok-value">eu-west-2</span>
        </p>
        <p>
          <span className="text-tok-kw">use</span> <span className="text-tok-cloud">gcp</span>{' '}
          <span className="text-tok-kw">in</span> <span className="text-tok-value">europe-west2</span>
        </p>
        <p>&nbsp;</p>
        <p>
          <span className="font-semibold text-tok-kind">service</span>{' '}
          <span className="rounded-sm bg-paper/10 font-semibold text-tok-name">api</span>
        </p>
        <p className="pl-[2ch]">
          <span className="text-tok-prop">scale</span> <span className="text-tok-num">0</span>{' '}
          <span className="text-tok-kw">to</span>{' '}
          <span className="text-tok-num underline decoration-tok-trait decoration-wavy underline-offset-4">5</span>
        </p>
        <div className="absolute left-4 right-4 top-[8.6rem] max-w-md rounded-lg border border-line bg-ink-3 p-3.5 font-sans text-[13px] leading-relaxed shadow-2xl shadow-black/60 sm:left-24 sm:right-auto">
          <p className="font-mono text-paper">
            <span className="text-tok-kind">service</span> api
          </p>
          <div className="mt-2 grid grid-cols-[3rem_1fr] gap-y-1 text-mute">
            <span className="font-mono text-tok-cloud">aws</span>
            <span>
              ECS Fargate <span className="text-bad">✗</span> minimum must be at least 1
            </span>
            <span className="font-mono text-tok-cloud">gcp</span>
            <span>
              Cloud Run <span className="text-ok">✓</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
