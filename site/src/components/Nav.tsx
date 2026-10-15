import Link from 'next/link'
import { Logo } from './Logo'

export const REPO = 'https://github.com/joshjkns/clowd'

export function Nav() {
  return (
    <header className="sticky top-0 z-40 border-b border-line bg-ink/85 backdrop-blur-md">
      <nav className="mx-auto flex h-14 max-w-6xl items-center gap-8 px-4 sm:px-6">
        <Link href="/" aria-label="Clowd home" className="flex items-center">
          <Logo />
        </Link>
        <div className="hidden items-center gap-6 text-sm text-mute sm:flex">
          <Link href="/docs/" className="transition hover:text-paper">
            Docs
          </Link>
          <Link href="/docs/language/" className="transition hover:text-paper">
            Language
          </Link>
          <Link href="/docs/cli/" className="transition hover:text-paper">
            CLI
          </Link>
          <a href={REPO} className="transition hover:text-paper">
            GitHub
          </a>
        </div>
        <div className="ml-auto flex items-center gap-4 text-sm">
          <Link href="/docs/" className="text-mute transition hover:text-paper sm:hidden">
            Docs
          </Link>
          <Link
            href="/docs/#install"
            className="rounded-full bg-paper px-4 py-1.5 font-medium text-ink transition hover:bg-white"
          >
            Install
          </Link>
        </div>
      </nav>
    </header>
  )
}
