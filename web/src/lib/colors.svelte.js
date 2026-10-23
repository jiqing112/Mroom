export const palette = $state({
  colors: ['#8b7cff', '#4fd1c5', '#1a1030'],
  source: ''
})

const cache = new Map()

export function hashHue(seed = '') {
  let h = 2166136261
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return Math.abs(h) % 360
}

export function hslToHex(h, s, l) {
  s /= 100
  l /= 100
  const k = (n) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  const f = (n) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))
  const toHex = (x) =>
    Math.round(255 * x)
      .toString(16)
      .padStart(2, '0')
  return `#${toHex(f(0))}${toHex(f(8))}${toHex(f(4))}`
}

export function fallbackPalette(seed = '') {
  const h = hashHue(seed || 'mrome')
  return [hslToHex(h, 62, 62), hslToHex((h + 58) % 360, 58, 50)]
}

function rgbToHsl(r, g, b) {
  r /= 255
  g /= 255
  b /= 255
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2
  let h = 0
  let s = 0
  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r:
        h = (g - b) / d + (g < b ? 6 : 0)
        break
      case g:
        h = (b - r) / d + 2
        break
      default:
        h = (r - g) / d + 4
    }
    h *= 60
  }
  return [h, s, l]
}

function extractFromImage(img) {
  const size = 48
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d', { willReadFrequently: true })
  ctx.drawImage(img, 0, 0, size, size)
  let data
  try {
    data = ctx.getImageData(0, 0, size, size).data
  } catch {
    return null
  }

  const buckets = new Map()
  for (let i = 0; i < data.length; i += 4) {
    const r = data[i]
    const g = data[i + 1]
    const b = data[i + 2]
    const [h, s, l] = rgbToHsl(r, g, b)
    if (s < 0.16 || l < 0.1 || l > 0.93) continue
    const key = Math.round(h / 12) * 12
    const score = s * (1 - Math.abs(l - 0.5)) * (l > 0.25 ? 1 : 0.4)
    const entry = buckets.get(key) || { score: 0, r: 0, g: 0, b: 0, n: 0 }
    entry.score += score
    entry.r += r
    entry.g += g
    entry.b += b
    entry.n += 1
    buckets.set(key, entry)
  }

  const ranked = [...buckets.entries()].sort((a, b) => b[1].score - a[1].score)
  if (!ranked.length) return null

  const pick = (entry) => {
    const r = entry.r / entry.n / 255
    const g = entry.g / entry.n / 255
    const b = entry.b / entry.n / 255
    const [h, s] = rgbToHsl(r * 255, g * 255, b * 255)
    return { h, s }
  }

  const first = pick(ranked[0][1])
  let second = null
  for (const [, entry] of ranked.slice(1)) {
    const c = pick(entry)
    const dist = Math.min(Math.abs(c.h - first.h), 360 - Math.abs(c.h - first.h))
    if (dist > 28) {
      second = c
      break
    }
  }
  const primary = hslToHex(first.h, Math.min(88, Math.max(46, first.s * 100)), 62)
  const secondary = second
    ? hslToHex(second.h, Math.min(82, Math.max(42, second.s * 100)), 54)
    : hslToHex((first.h + 46) % 360, 62, 52)
  return [primary, secondary]
}

export async function paletteFor(id) {
  if (!id) return fallbackPalette('empty')
  if (cache.has(id)) return cache.get(id)
  const promise = (async () => {
    try {
      const img = new Image()
      img.crossOrigin = 'anonymous'
      img.decoding = 'async'
      img.src = `/api/cover/${id}/160`
      if (!img.complete) {
        await new Promise((resolve, reject) => {
          img.onload = resolve
          img.onerror = reject
          setTimeout(reject, 8000)
        })
      }
      await img.decode?.()
      return extractFromImage(img) || fallbackPalette(id)
    } catch {
      return fallbackPalette(id)
    }
  })()
  cache.set(id, promise)
  return promise
}

function hexToRgb(hex) {
  const h = hex.replace('#', '')
  const n = parseInt(h.length === 3 ? h.split('').map((c) => c + c).join('') : h, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

export function applyAccent(colors) {
  if (!colors || !colors.length) return
  const root = document.documentElement
  const [a, b] = colors
  root.style.setProperty('--accent', a)
  root.style.setProperty('--accent-2', b || a)
  const [r, g, bl] = hexToRgb(a)
  const [r2, g2, b2] = hexToRgb(b || a)
  root.style.setProperty('--accent-rgb', `${r}, ${g}, ${bl}`)
  root.style.setProperty('--accent-2-rgb', `${r2}, ${g2}, ${b2}`)
  palette.colors = [a, b || a, ...colors.slice(2)]
}
