<script>
  import { palette } from '../lib/colors.svelte.js'
  import { perf } from '../lib/perf.svelte.js'

  let canvas = $state(null)
  let target = palette.colors.slice()

  const hexToRgb = (hex) => {
    const h = (hex || '#8b7cff').replace('#', '')
    const n = parseInt(h.length === 3 ? h.split('').map((c) => c + c).join('') : h, 16)
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
  }

  $effect(() => {
    target = palette.colors.map(hexToRgb)
  })

  // The blobs drift on a ~5 minute cycle, so ~24 fps is indistinguishable from
  // 60 fps while cutting the compositor and raster work. In smooth/static modes
  // we do not paint at all — CSS supplies a flat gradient instead.
  $effect(() => {
    if (!canvas || perf.smooth) return
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

    const ctx = canvas.getContext('2d')
    let w = 0
    let h = 0
    let raf = 0
    let t = 0
    let last = 0

    const blobs = [
      { bx: 0.22, by: 0.28, r: 0.62, ax: 0.16, ay: 0.12, sx: 0.0029, sy: 0.0021, ph: 0.0, c: 0, a: 0.85 },
      { bx: 0.78, by: 0.3, r: 0.52, ax: 0.14, ay: 0.16, sx: 0.0021, sy: 0.0033, ph: 2.1, c: 1, a: 0.7 },
      { bx: 0.6, by: 0.82, r: 0.66, ax: 0.18, ay: 0.1, sx: 0.0017, sy: 0.0027, ph: 4.2, c: 0, a: 0.5 },
      { bx: 0.3, by: 0.75, r: 0.44, ax: 0.12, ay: 0.14, sx: 0.0037, sy: 0.0019, ph: 1.2, c: 1, a: 0.42 }
    ]

    let cur = target.map((c) => c.slice())

    function resize() {
      const dpr = Math.min(window.devicePixelRatio || 1, 1.5)
      w = Math.max(1, Math.floor((window.innerWidth * dpr) / 2))
      h = Math.max(1, Math.floor((window.innerHeight * dpr) / 2))
      canvas.width = w
      canvas.height = h
      paint()
    }

    function paint() {
      ctx.clearRect(0, 0, w, h)
      ctx.globalCompositeOperation = 'lighter'
      const base = Math.max(w, h)
      for (const b of blobs) {
        const cx = (b.bx + Math.sin(t * b.sx + b.ph) * b.ax) * w
        const cy = (b.by + Math.cos(t * b.sy + b.ph * 1.4) * b.ay) * h
        const r = b.r * base * (0.92 + 0.08 * Math.sin(t * 0.004 + b.ph))
        const col = cur[b.c] || cur[0]
        const g = ctx.createRadialGradient(cx, cy, 0, cx, cy, r)
        g.addColorStop(0, `rgba(${col[0]},${col[1]},${col[2]},${0.5 * b.a})`)
        g.addColorStop(0.45, `rgba(${col[0]},${col[1]},${col[2]},${0.2 * b.a})`)
        g.addColorStop(1, `rgba(${col[0]},${col[1]},${col[2]},0)`)
        ctx.fillStyle = g
        ctx.beginPath()
        ctx.arc(cx, cy, r, 0, Math.PI * 2)
        ctx.fill()
      }
      ctx.globalCompositeOperation = 'source-over'
    }

    function draw(now) {
      if (now - last >= 41) {
        last = now
        t += 1
        for (let i = 0; i < cur.length; i++) {
          const tgt = target[i] || target[0] || [139, 124, 255]
          for (let k = 0; k < 3; k++) cur[i][k] += (tgt[k] - cur[i][k]) * 0.04
        }
        paint()
      }
      if (!reduce && !document.hidden) raf = requestAnimationFrame(draw)
    }

    function onVisibility() {
      cancelAnimationFrame(raf)
      if (!document.hidden && !reduce) {
        last = 0
        raf = requestAnimationFrame(draw)
      }
    }

    resize()
    window.addEventListener('resize', resize)
    document.addEventListener('visibilitychange', onVisibility)
    if (!reduce) raf = requestAnimationFrame(draw)

    return () => {
      cancelAnimationFrame(raf)
      window.removeEventListener('resize', resize)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  })
</script>

<canvas bind:this={canvas} class="aurora" aria-hidden="true"></canvas>

<style>
  .aurora {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
    z-index: 0;
    pointer-events: none;
    opacity: var(--aurora-opacity);
    transform: scale(1.18);
    transition: opacity 0.8s var(--ease);
  }
  /* smooth mode paints the wash with a flat gradient in app.css instead */
  :global([data-perf='smooth']) .aurora {
    display: none;
  }
</style>
