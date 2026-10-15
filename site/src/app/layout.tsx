import type { Metadata, Viewport } from 'next'
import { IBM_Plex_Mono, IBM_Plex_Sans, IBM_Plex_Serif } from 'next/font/google'
import { Nav } from '@/components/Nav'
import { Footer } from '@/components/Footer'
import './globals.css'

const sans = IBM_Plex_Sans({ subsets: ['latin'], weight: ['400', '500', '600'], variable: '--font-plex-sans' })
const serif = IBM_Plex_Serif({
  subsets: ['latin'],
  weight: ['400', '500'],
  style: ['normal', 'italic'],
  variable: '--font-plex-serif',
})
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

export const viewport: Viewport = { themeColor: '#12110f' }

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${sans.variable} ${serif.variable} ${mono.variable}`}>
      <body className="min-h-dvh overflow-x-hidden">
        <Nav />
        {children}
        <Footer />
      </body>
    </html>
  )
}
