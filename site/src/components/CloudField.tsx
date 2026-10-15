'use client'

import { useEffect, useRef } from 'react'

/**
 * A full-bleed field of characters with clouds drifting through it: every
 * cell holds a random character, and its brightness is the cloud density at
 * that point. Puffs move slowly sideways and wrap, and a few characters
 * change each frame. Drawn once and left still for reduced motion.
 */
const CHARS = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
const FONT = 13
const CELL_W = 8
const CELL_H = 18
const FPS = 24

// [x, y, radius] in fractions of the field's width and height, plus a speed
// in widths per second. Grouped so they read as three or four clouds.
const PUFFS: [number, number, number, number][] = [
  [0.08, 0.28, 0.13, 0.004],
  [0.18, 0.22, 0.17, 0.004],
  [0.28, 0.3, 0.12, 0.004],
  [0.55, 0.12, 0.11, 0.006],
  [0.66, 0.08, 0.16, 0.006],
  [0.78, 0.14, 0.12, 0.006],
  [0.62, 0.55, 0.14, 0.003],
  [0.74, 0.5, 0.19, 0.003],
  [0.88, 0.58, 0.13, 0.003],
  [0.15, 0.72, 0.12, 0.005],
  [0.27, 0.68, 0.15, 0.005],
  [0.38, 0.75, 0.1, 0.005],
]

const BASE = 0
const PEAK = 0.22

export function CloudField({ className = '' }: { className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = ref.current
    const ctx = canvas?.getContext('2d')
    if (!canvas || !ctx) return

    const still = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let cols = 0
    let rows = 0
    let grid = new Uint8Array(0)
    let w = 0
    let h = 0

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      w = canvas.clientWidth
      h = canvas.clientHeight
      canvas.width = Math.round(w * dpr)
      canvas.height = Math.round(h * dpr)
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
      cols = Math.ceil(w / CELL_W)
      rows = Math.ceil(h / CELL_H)
      grid = new Uint8Array(cols * rows)
      for (let i = 0; i < grid.length; i++) grid[i] = (Math.random() * CHARS.length) | 0
    }

    const draw = (t: number) => {
      ctx.clearRect(0, 0, w, h)
      ctx.font = `${FONT}px ${getComputedStyle(canvas).fontFamily}`
      ctx.textBaseline = 'top'
      ctx.fillStyle = '#1e40af'

      // Puffs wrap with a margin so they enter and leave off screen.
      const puffs = PUFFS.map(([x, y, r, v]) => {
        const px = ((((x + v * t) % 1.4) + 1.4) % 1.4) - 0.2
        return [px * w, y * h, Math.max(90, r * w * 1.1)] as const
      })

      for (let row = 0; row < rows; row++) {
        const cy = row * CELL_H + CELL_H / 2
        for (let col = 0; col < cols; col++) {
          const cx = col * CELL_W + CELL_W / 2
          let d = 0
          for (const [px, py, r] of puffs) {
            const dx = (cx - px) / r
            const dy = (cy - py) / (r * 0.62)
            const q = 1 - dx * dx - dy * dy
            if (q > d) d = q
          }
          const a = BASE + (PEAK - BASE) * d * d * (3 - 2 * d)
          if (a < 0.03) continue
          ctx.globalAlpha = a
          ctx.fillText(CHARS[grid[row * cols + col]], col * CELL_W, row * CELL_H)
        }
      }
      ctx.globalAlpha = 1
    }

    resize()
    const ro = new ResizeObserver(() => {
      resize()
      draw(performance.now() / 1000)
    })
    ro.observe(canvas)

    if (still) {
      draw(0)
      return () => ro.disconnect()
    }

    let frame = 0
    let last = 0
    let visible = true
    const io = new IntersectionObserver(([e]) => (visible = e.isIntersecting))
    io.observe(canvas)

    const tick = (now: number) => {
      frame = requestAnimationFrame(tick)
      if (!visible || now - last < 1000 / FPS) return
      last = now
      for (let i = 0; i < grid.length / 60; i++) {
        grid[(Math.random() * grid.length) | 0] = (Math.random() * CHARS.length) | 0
      }
      draw(now / 1000)
    }
    frame = requestAnimationFrame(tick)

    return () => {
      cancelAnimationFrame(frame)
      ro.disconnect()
      io.disconnect()
    }
  }, [])

  return <canvas ref={ref} aria-hidden className={`pointer-events-none absolute inset-x-0 top-0 -z-10 w-full font-mono ${className}`} />
}
