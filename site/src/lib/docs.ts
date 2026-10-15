export type DocLink = { href: string; title: string }

export const DOCS: { section: string; pages: DocLink[] }[] = [
  {
    section: 'Start',
    pages: [
      { href: '/docs/', title: 'Introduction' },
      { href: '/docs/tour/', title: 'A tour of app.clowd' },
    ],
  },
  {
    section: 'Language',
    pages: [
      { href: '/docs/language/', title: 'Syntax' },
      { href: '/docs/resources/', title: 'Resources' },
      { href: '/docs/relationships/', title: 'Relationships' },
      { href: '/docs/projects/', title: 'Projects & networks' },
      { href: '/docs/overrides/', title: 'Cloud overrides' },
    ],
  },
  {
    section: 'Tools',
    pages: [
      { href: '/docs/check/', title: 'clowd check' },
      { href: '/docs/cli/', title: 'CLI reference' },
      { href: '/docs/editor/', title: 'Editor & formatter' },
    ],
  },
]

export const FLAT = DOCS.flatMap((s) => s.pages)
