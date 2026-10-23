export function fmtTime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const s = Math.floor(seconds % 60)
  const m = Math.floor((seconds / 60) % 60)
  const h = Math.floor(seconds / 3600)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

export function fmtDuration(seconds) {
  if (!seconds) return '0 分钟'
  const total = Math.round(seconds / 60)
  const h = Math.floor(total / 60)
  const m = total % 60
  if (h > 0) return `${h} 小时 ${m} 分`
  return `${m} 分钟`
}

export function fmtCount(n, unit = '首') {
  if (n === undefined || n === null) return '—'
  return `${n.toLocaleString('zh-CN')} ${unit}`
}

export function fmtDate(unix) {
  if (!unix) return ''
  const d = new Date(unix * 1000)
  return `${d.getFullYear()} 年 ${d.getMonth() + 1} 月`
}

export function fmtDateShort(unix) {
  if (!unix) return ''
  const d = new Date(unix * 1000)
  return `${d.getFullYear()}.${String(d.getMonth() + 1).padStart(2, '0')}.${String(d.getDate()).padStart(2, '0')}`
}

export function initials(name = '') {
  const t = name.trim()
  if (!t) return '?'
  return t.slice(0, 1).toUpperCase()
}
