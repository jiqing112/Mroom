const TIME_TAG = /\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]/g
const META_TAG = /^\[[a-zA-Z#]+:.*\]$/

export function parseLyrics(raw) {
  if (!raw || !raw.trim()) return []
  const out = []
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const matches = [...trimmed.matchAll(TIME_TAG)]
    const text = trimmed.replace(TIME_TAG, '').trim()
    if (!matches.length) {
      if (text && !META_TAG.test(trimmed)) out.push({ time: null, text })
      continue
    }
    if (!text) continue
    for (const m of matches) {
      const min = parseInt(m[1], 10)
      const sec = parseInt(m[2], 10)
      let ms = 0
      if (m[3]) {
        const frac = m[3].length === 1 ? m[3] + '00' : m[3].length === 2 ? m[3] + '0' : m[3]
        ms = parseInt(frac, 10)
      }
      out.push({ time: min * 60000 + sec * 1000 + ms, text })
    }
  }
  const timed = out.filter((l) => l.time !== null)
  if (timed.length) return timed.sort((a, b) => a.time - b.time)
  return out
}

export function activeLyricIndex(lines, timeMs) {
  if (!lines.length) return -1
  if (lines[0].time === null) return -1
  let lo = 0
  let hi = lines.length - 1
  let idx = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (lines[mid].time <= timeMs) {
      idx = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return idx
}
