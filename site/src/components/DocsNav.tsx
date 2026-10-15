'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useState } from 'react'
import { ArrowLeft, ArrowRight, Menu, X } from 'lucide-react'
import { DOCS, FLAT } from '@/lib/docs'

function normalise(p: string) {
  return p.endsWith('/') ? p : p + '/'
}

export function DocsSidebar() {
  const path = normalise(usePathname())
  const [open, setOpen] = useState(false)
  useEffect(() => setOpen(false), [path])

  const list = (
    <nav className="space-y-7 text-sm">
      {DOCS.map((s) => (
        <div key={s.section}>
          <p className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-faint">{s.section}</p>
          <ul className="space-y-0.5 border-l border-line">
            {s.pages.map((p) => {
              const active = path === p.href
              return (
                <li key={p.href}>
                  <Link
                    href={p.href}
                    className={`-ml-px block border-l py-1.5 pl-4 transition ${active ? 'border-fg text-fg' : 'border-transparent text-mute hover:border-mute hover:text-fg'}`}
                  >
                    {p.title}
                  </Link>
                </li>
              )
            })}
          </ul>
        </div>
      ))}
    </nav>
  )

  return (
    <>
      <div className="lg:hidden">
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="flex w-full items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm text-mute"
          aria-expanded={open}
        >
          {open ? <X className="size-4" /> : <Menu className="size-4" />}
          {FLAT.find((p) => p.href === path)?.title ?? 'Docs'}
        </button>
        {open && <div className="mt-4 border border-edge bg-page p-4">{list}</div>}
      </div>
      <aside className="hidden lg:block">
        <div className="sticky top-28 max-h-[calc(100dvh-8rem)] overflow-y-auto pb-10">{list}</div>
      </aside>
    </>
  )
}

export function DocsPager() {
  const path = normalise(usePathname())
  const i = FLAT.findIndex((p) => p.href === path)
  const prev = i > 0 ? FLAT[i - 1] : undefined
  const next = i >= 0 && i < FLAT.length - 1 ? FLAT[i + 1] : undefined
  return (
    <div className="mt-20 grid gap-3 border-t border-line pt-8 sm:grid-cols-2">
      {prev ? (
        <Link href={prev.href} className="group border border-line-2 px-4 py-3 transition hover:border-edge">
          <span className="flex items-center gap-1.5 text-xs text-faint">
            <ArrowLeft className="size-3.5" /> Previous
          </span>
          <span className="mt-1 block font-display text-lg text-fg">{prev.title}</span>
        </Link>
      ) : (
        <span />
      )}
      {next && (
        <Link href={next.href} className="group border border-line-2 px-4 py-3 text-right transition hover:border-edge">
          <span className="flex items-center justify-end gap-1.5 text-xs text-faint">
            Next <ArrowRight className="size-3.5" />
          </span>
          <span className="mt-1 block font-display text-lg text-fg">{next.title}</span>
        </Link>
      )}
    </div>
  )
}
