/**
 * The glyph: a cloud with two lines of text cut out of it, since a Clowd
 * file is a cloud written in sentences. Paths match src/app/icon.svg.
 */
export function Glyph({ className = '' }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden>
      <path
        fill="currentColor"
        fillRule="evenodd"
        d="M6.5 19A4.5 4.5 0 0 1 5.6 10.1 6.5 6.5 0 0 1 18.2 8.6 5.2 5.2 0 0 1 17.5 19H6.5ZM8.8 12.1h6.4a.8.8 0 0 1 0 1.6H8.8a.8.8 0 0 1 0-1.6Zm0 3h3.6a.8.8 0 0 1 0 1.6H8.8a.8.8 0 0 1 0-1.6Z"
      />
    </svg>
  )
}

export function Logo({ className = '' }: { className?: string }) {
  return (
    <span className={`flex items-center gap-2 leading-none ${className}`}>
      <Glyph className="size-6 shrink-0 text-paper" />
      <span className="font-serif text-[18px] text-paper">Clowd</span>
    </span>
  )
}
