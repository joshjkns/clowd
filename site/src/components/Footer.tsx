import Link from 'next/link'
import { Logo } from './Logo'
import { REPO } from './Nav'

export function Footer() {
  return (
    <footer className="border-t border-line/70">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-10 text-sm text-mute sm:flex-row sm:items-center sm:px-6">
        <div className="flex items-center gap-3">
          <Logo className="text-base" />
          <span className="font-mono text-faint"># compiles to OpenTofu JSON</span>
        </div>
        <div className="flex flex-wrap gap-x-5 gap-y-2 sm:ml-auto">
          <Link href="/docs/" className="hover:text-paper">Docs</Link>
          <Link href="/docs/resources/" className="hover:text-paper">Resources</Link>
          <Link href="/docs/editor/" className="hover:text-paper">Editor</Link>
          <a href={REPO} className="hover:text-paper">GitHub</a>
          <a href={`${REPO}/blob/main/docs/decisions.md`} className="hover:text-paper">Decisions</a>
        </div>
      </div>
    </footer>
  )
}
