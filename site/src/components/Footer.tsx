import Link from 'next/link'
import { Glyph } from './Logo'
import { REPO } from './Nav'

const LINKS: [string, string][] = [
  ['Docs', '/docs/'],
  ['Tour', '/docs/tour/'],
  ['Syntax', '/docs/language/'],
  ['Resources', '/docs/resources/'],
  ['clowd check', '/docs/check/'],
  ['CLI', '/docs/cli/'],
  ['Editor', '/docs/editor/'],
  ['GitHub', REPO],
  ['Plans', `${REPO}/tree/main/docs/plans`],
  ['Decisions', `${REPO}/blob/main/docs/decisions.md`],
  ['Issues', `${REPO}/issues`],
]

export function Footer() {
  return (
    <footer className="border-t border-line">
      <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
        <p className="flex items-center gap-2.5 font-display text-lg text-fg">
          <Glyph className="size-5 shrink-0 text-fg" />
          Infrastructure in plain English. Compiles to OpenTofu.
        </p>
        <ul className="mt-6 flex flex-wrap gap-x-5 gap-y-2 font-mono text-[13px] text-mute">
          {LINKS.map(([label, href]) => (
            <li key={label}>
              {href.startsWith('/') ? (
                <Link href={href} className="transition hover:text-fg">
                  {label}
                </Link>
              ) : (
                <a href={href} className="transition hover:text-fg">
                  {label}
                </a>
              )}
            </li>
          ))}
        </ul>
      </div>
    </footer>
  )
}
