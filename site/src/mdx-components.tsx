import type { MDXComponents } from 'mdx/types'
import { isValidElement } from 'react'
import Link from 'next/link'
import { Code } from '@/components/Code'

type CodeProps = { className?: string; children?: React.ReactNode }

const components: MDXComponents = {
  // Fenced code arrives as <pre><code className="language-x">; render it with
  // the site's highlighter instead. A `title` goes in the fence's meta line,
  // which MDX doesn't pass on, so titles use the <Code> component directly.
  pre: ({ children }) => {
    if (isValidElement<CodeProps>(children)) {
      const lang = children.props.className?.replace('language-', '') ?? 'text'
      const src = String(children.props.children ?? '')
      return <Code src={src} lang={lang} className="my-6" />
    }
    return <pre>{children}</pre>
  },
  a: ({ href = '', children }) =>
    href.startsWith('/') || href.startsWith('#') ? (
      <Link href={href}>{children}</Link>
    ) : (
      <a href={href}>{children}</a>
    ),
  Code,
}

export function useMDXComponents(): MDXComponents {
  return components
}
