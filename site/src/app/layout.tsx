import type { Metadata, Viewport } from 'next'
import { Bricolage_Grotesque, Geist, JetBrains_Mono } from 'next/font/google'
import { Nav } from '@/components/Nav'
import { Footer } from '@/components/Footer'
import './globals.css'

const geist = Geist({ subsets: ['latin'], variable: '--font-geist' })
const bricolage = Bricolage_Grotesque({ subsets: ['latin'], variable: '--font-bricolage' })
const jetbrains = JetBrains_Mono({ subsets: ['latin'], variable: '--font-jetbrains' })

export const metadata: Metadata = {
  metadataBase: new URL('https://clowd.jooo.sh'),
  title: {
    default: 'Clowd — write the cloud in sentences',
    template: '%s · Clowd',
  },
  description:
    'Clowd is a tiny, English-like language for cloud infrastructure. Describe it once, check it against every cloud, deploy with OpenTofu.',
  openGraph: {
    title: 'Clowd',
    description: 'api connects to db. Infrastructure in plain, structured English.',
    url: 'https://clowd.jooo.sh',
    siteName: 'Clowd',
    type: 'website',
  },
}

export const viewport: Viewport = { themeColor: '#0b0c10' }

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geist.variable} ${bricolage.variable} ${jetbrains.variable}`}>
      <body className="min-h-dvh overflow-x-hidden">
        <Nav />
        {children}
        <Footer />
      </body>
    </html>
  )
}
