import { DocsPager, DocsSidebar } from '@/components/DocsNav'
import { REPO } from '@/components/Nav'

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto grid max-w-6xl gap-10 px-4 pb-24 pt-10 sm:px-6 lg:grid-cols-[13rem_minmax(0,1fr)] lg:gap-16 lg:pt-14">
      <DocsSidebar />
      <main className="min-w-0 max-w-3xl">
        <article className="prose-clowd">{children}</article>
        <DocsPager />
        <p className="mt-8 text-xs text-faint">
          Something wrong or missing?{' '}
          <a href={`${REPO}/issues/new`} className="underline underline-offset-2 hover:text-mute">
            Open an issue
          </a>
          .
        </p>
      </main>
    </div>
  )
}
