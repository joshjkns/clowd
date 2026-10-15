import type { Metadata, Viewport } from 'next'
import { Funnel_Display, IBM_Plex_Mono, Inter } from 'next/font/google'
import { Nav } from '@/components/Nav'
import { Footer } from '@/components/Footer'
import './globals.css'

const sans = Inter({ subsets: ['latin'], variable: '--font-inter' })
const display = Funnel_Display({ subsets: ['latin'], variable: '--font-funnel' })
const mono = IBM_Plex_Mono({ subsets: ['latin'], weight: ['400', '500', '600', '700'], variable: '--font-plex-mono' })

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

export const viewport: Viewport = { themeColor: '#ffffff' }

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${sans.variable} ${display.variable} ${mono.variable}`}>
      <body className="min-h-dvh overflow-x-hidden">
        <Nav />
        {children}
        <Footer />
      </body>
    </html>
  )
}
