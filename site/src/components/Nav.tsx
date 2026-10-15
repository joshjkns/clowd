import Link from 'next/link'
import { Logo } from './Logo'

export const REPO = 'https://github.com/joshjkns/clowd'

const LINKS: [string, string][] = [
  ['Docs', '/docs/'],
  ['Language', '/docs/language/'],
  ['CLI', '/docs/cli/'],
  ['GitHub', REPO],
]

export function Nav() {
  return (
    <header className="sticky top-0 z-40 border-b border-line bg-ink">
      <nav className="mx-auto flex h-14 max-w-6xl items-center gap-8 px-4 sm:px-6">
        <div className="flex items-center gap-3">
          <Link href="/" aria-label="Clowd home" className="flex items-center">
            <Logo />
          </Link>
          {/* The beta notice lives here so it shows on every page without a banner. */}
          <Link
            href="/docs/#status"
            title="Clowd is a work in progress, and the language may change before 1.0"
            className="border border-line-2 px-1.5 py-px font-mono text-[10.5px] uppercase tracking-[0.12em] text-mute transition hover:border-mute hover:text-paper"
          >
            Beta
          </Link>
        </div>
        <div className="hidden items-center gap-6 font-mono text-[13px] text-mute sm:flex">
          {LINKS.map(([label, href]) =>
            href.startsWith('/') ? (
              <Link key={href} href={href} className="transition hover:text-paper">
                {label}
              </Link>
            ) : (
              <a key={href} href={href} className="transition hover:text-paper">
                {label}
              </a>
            ),
          )}
        </div>
        <div className="ml-auto flex items-center gap-4 font-mono text-[13px]">
          <Link href="/docs/" className="text-mute transition hover:text-paper sm:hidden">
            Docs
          </Link>
          <Link href="/docs/#install" className="border border-paper px-3 py-1 text-paper transition hover:bg-paper hover:text-ink">
            Install
          </Link>
        </div>
      </nav>
    </header>
  )
}
