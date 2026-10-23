export class ApiError extends Error {
  constructor(status, message) {
    super(message || '请求失败')
    this.status = status
  }
}

async function request(path, { method = 'GET', body, headers = {} } = {}) {
  const opts = {
    method,
    credentials: 'same-origin',
    headers: { ...headers }
  }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { raw: text }
    }
  }
  if (!res.ok) {
    // A 401 on anything but the login call means the session expired.
    if (res.status === 401 && typeof window !== 'undefined' && !path.startsWith('/api/auth/login')) {
      window.dispatchEvent(new CustomEvent('mrome:unauthorized'))
    }
    throw new ApiError(res.status, data?.error || `HTTP ${res.status}`)
  }
  return data
}

export const api = {
  get: (p) => request(p),
  post: (p, body) => request(p, { method: 'POST', body }),
  patch: (p, body) => request(p, { method: 'PATCH', body }),
  del: (p) => request(p, { method: 'DELETE' })
}

export function qs(params) {
  const out = new URLSearchParams()
  for (const [k, v] of Object.entries(params || {})) {
    if (v !== undefined && v !== null && v !== '') out.set(k, String(v))
  }
  const s = out.toString()
  return s ? `?${s}` : ''
}
