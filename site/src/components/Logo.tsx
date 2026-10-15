/** The wordmark: the name as you'd type it, with the cursor still blinking. */
export function Logo({ className = '' }: { className?: string }) {
  return (
    <span className={`inline-flex items-baseline font-mono font-bold tracking-tight ${className}`}>
      <span className="text-tok-kind">c</span>
      <span className="text-tok-verb">l</span>
      <span className="text-tok-trait">o</span>
      <span className="text-tok-cloud">w</span>
      <span className="text-tok-kw">d</span>
      <span aria-hidden className="ml-0.5 inline-block h-[0.95em] w-[0.5em] translate-y-[0.12em] animate-blink bg-paper" />
    </span>
  )
}
