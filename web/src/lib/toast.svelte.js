export const toasts = $state({ items: [] })
let seq = 0

export function toast(message, kind = 'info', ttl = 3200) {
  const id = ++seq
  toasts.items.push({ id, message, kind })
  setTimeout(() => dismiss(id), ttl)
  return id
}

export function dismiss(id) {
  const i = toasts.items.findIndex((t) => t.id === id)
  if (i >= 0) toasts.items.splice(i, 1)
}

export const notify = {
  ok: (m) => toast(m, 'ok'),
  err: (m) => toast(m, 'err', 4200),
  info: (m) => toast(m, 'info')
}
