import type { Metadata, Viewport } from 'next'
import Link from 'next/link'
import { Geist, Geist_Mono } from 'next/font/google'
import { Nav } from '@/components/Nav'
import { Footer } from '@/components/Footer'
import './globals.css'

const geist = Geist({ subsets: ['latin'], variable: '--font-geist' })
const geistMono = Geist_Mono({ subsets: ['latin'], variable: '--font-geist-mono' })

export const metadata: Metadata = {
  metadataBase: new URL('https://clowd.jooo.sh'),
  title: {
    default: 'Clowd — infrastructure in plain English',
    template: '%s · Clowd',
  },
  description:
    'Clowd is a small, English-like language for cloud infrastructure. Describe it once, check it against every cloud, deploy with OpenTofu.',
  openGraph: {
    title: 'Clowd',
    description: 'Infrastructure in plain English. Describe it once, deploy it anywhere.',
    url: 'https://clowd.jooo.sh',
    siteName: 'Clowd',
    type: 'website',
  },
}

export const viewport: Viewport = { themeColor: '#000000' }

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geist.variable} ${geistMono.variable}`}>
      <body className="min-h-dvh overflow-x-hidden">
        <div className="border-b border-line bg-ink-2 px-4 py-2 text-center text-[13px] text-mute">
          <span className="text-paper">Beta.</span> Clowd is a work in progress
          <span className="hidden sm:inline">, and the language may change before 1.0</span>.{' '}
          <Link href="/docs/#status" className="text-paper underline decoration-faint underline-offset-2 hover:decoration-paper">
            What works today
          </Link>
        </div>
        <Nav />
        {children}
        <Footer />
      </body>
    </html>
  )
}
