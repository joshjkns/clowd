import createMDX from '@next/mdx'

/** @type {import('next').NextConfig} */
const nextConfig = {
  // GitHub Pages serves static files only.
  output: 'export',
  // Emit /docs/cli/index.html so Pages resolves /docs/cli without a rewrite.
  trailingSlash: true,
  images: { unoptimized: true },
  pageExtensions: ['ts', 'tsx', 'mdx'],
}

const withMDX = createMDX({
  options: {
    remarkPlugins: ['remark-gfm'],
    rehypePlugins: ['rehype-slug'],
  },
})

export default withMDX(nextConfig)
