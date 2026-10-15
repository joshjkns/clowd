import { highlight, tokenClass } from '@/lib/clowd'

type Props = {
  src: string
  lang?: string
  title?: string
  lineNumbers?: boolean
  className?: string
}

/** A static code block. Clowd is highlighted; shell gets a dimmed prompt. */
export function Code({ src, lang = 'clowd', title, lineNumbers = false, className = '' }: Props) {
  const text = src.replace(/\n$/, '')
  return (
    <div className={`overflow-hidden rounded-xl border border-line bg-ink-2 ${className}`}>
      {title && (
        <div className="flex items-center gap-2 border-b border-line px-4 py-2 font-mono text-xs text-mute">
          <span className="size-2 rounded-full bg-tok-verb/70" />
          {title}
        </div>
      )}
      <pre className="overflow-x-auto px-4 py-3.5 font-mono text-[13px] leading-[1.7]">
        <code>{lang === 'clowd' ? <Clowd text={text} lineNumbers={lineNumbers} /> : lang === 'sh' || lang === 'shell' ? <Shell text={text} /> : text}</code>
      </pre>
    </div>
  )
}

function Clowd({ text, lineNumbers }: { text: string; lineNumbers: boolean }) {
  return highlight(text).map((line, i) => (
    <span key={i} className="block min-h-[1.7em]">
      {lineNumbers && <span className="mr-4 inline-block w-5 select-none text-right text-faint">{i + 1}</span>}
      {line.map((t, j) => (
        <span key={j} className={tokenClass[t.kind]}>
          {t.text}
        </span>
      ))}
    </span>
  ))
}

function Shell({ text }: { text: string }) {
  return text.split('\n').map((line, i) => {
    if (line.startsWith('$ ')) {
      return (
        <span key={i} className="block">
          <span className="select-none text-tok-verb">$ </span>
          <span className="text-paper">{line.slice(2)}</span>
        </span>
      )
    }
    if (line.trimStart().startsWith('#')) {
      return (
        <span key={i} className="block italic text-tok-comment">
          {line}
        </span>
      )
    }
    // Colour diagnostics and summaries the way the CLI does on a terminal.
    const tone = /^error/.test(line)
      ? 'text-bad'
      : /^warning/.test(line)
        ? 'text-warn'
        : /^\s*= help/.test(line)
          ? 'text-tok-prop'
          : /^✓/.test(line)
            ? 'text-ok'
            : 'text-mute'
    return (
      <span key={i} className={`block ${tone}`}>
        {line || ' '}
      </span>
    )
  })
}
