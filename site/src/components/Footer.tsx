import Link from 'next/link'
import { Logo } from './Logo'
import { REPO } from './Nav'

const COLUMNS: { title: string; links: [string, string][] }[] = [
  {
    title: 'Learn',
    links: [
      ['Introduction', '/docs/'],
      ['A tour of app.clowd', '/docs/tour/'],
      ['Syntax', '/docs/language/'],
      ['Resources', '/docs/resources/'],
    ],
  },
  {
    title: 'Tools',
    links: [
      ['clowd check', '/docs/check/'],
      ['CLI reference', '/docs/cli/'],
      ['Editor & formatter', '/docs/editor/'],
    ],
  },
  {
    title: 'Project',
    links: [
      ['GitHub', REPO],
      ['Plans', `${REPO}/tree/main/docs/plans`],
      ['Decisions', `${REPO}/blob/main/docs/decisions.md`],
      ['Issues', `${REPO}/issues`],
    ],
  },
]

export function Footer() {
  return (
    <footer className="border-t border-line">
      <div className="mx-auto grid max-w-6xl gap-10 px-4 py-14 sm:px-6 md:grid-cols-[1fr_repeat(3,10rem)]">
        <div>
          <Logo />
          <p className="mt-4 max-w-xs text-sm text-mute">Infrastructure in plain English. Compiles to OpenTofu.</p>
        </div>
        {COLUMNS.map((c) => (
          <div key={c.title}>
            <p className="text-sm text-paper">{c.title}</p>
            <ul className="mt-4 space-y-2.5 text-sm text-mute">
              {c.links.map(([label, href]) => (
                <li key={label}>
                  {href.startsWith('/') ? (
                    <Link href={href} className="transition hover:text-paper">
                      {label}
                    </Link>
                  ) : (
                    <a href={href} className="transition hover:text-paper">
                      {label}
                    </a>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </footer>
  )
}
