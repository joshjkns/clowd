'use client'

import { useState } from 'react'
import { Check, Copy } from 'lucide-react'

export const INSTALL = 'go install github.com/joshjkns/clowd/cmd/clowd@latest'

export function CopyCommand({ command = INSTALL, className = '' }: { command?: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      type="button"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(command)
          setCopied(true)
          setTimeout(() => setCopied(false), 1400)
        } catch {
          // Clipboard can be blocked; the command is still selectable.
        }
      }}
      className={`group flex max-w-full items-center gap-3 overflow-hidden rounded-lg border border-edge bg-page px-4 py-3 text-left font-mono text-[13px] transition hover:bg-page-2 ${className}`}
      aria-label={`Copy: ${command}`}
    >
      <span className="select-none text-faint">$</span>
      <span className="truncate text-fg">{command}</span>
      <span className="ml-auto shrink-0 text-mute transition group-hover:text-fg">
        {copied ? <Check className="size-4 text-fg" /> : <Copy className="size-4" />}
      </span>
    </button>
  )
}
