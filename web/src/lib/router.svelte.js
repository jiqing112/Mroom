const routes = [
  { re: /^\/$/, name: 'home' },
  { re: /^\/songs$/, name: 'songs' },
  { re: /^\/favorites$/, name: 'favorites' },
  { re: /^\/recent$/, name: 'recent' },
  { re: /^\/artists$/, name: 'artists' },
  { re: /^\/artists\/([^/]+)$/, name: 'artist', keys: ['id'] },
  { re: /^\/albums$/, name: 'albums' },
  { re: /^\/albums\/([^/]+)$/, name: 'album', keys: ['id'] },
  { re: /^\/playlists$/, name: 'playlists' },
  { re: /^\/playlists\/([^/]+)$/, name: 'playlist', keys: ['id'] },
  { re: /^\/upload$/, name: 'upload' },
  { re: /^\/settings$/, name: 'settings' },
  { re: /^\/login$/, name: 'login' }
]

export function resolve(path) {
  for (const r of routes) {
    const m = path.match(r.re)
    if (m) {
      const params = {}
      ;(r.keys || []).forEach((k, i) => (params[k] = m[i + 1]))
      return { name: r.name, params }
    }
  }
  return { name: 'notfound', params: {} }
}

export const route = $state({
  path: typeof location !== 'undefined' ? location.pathname : '/',
  name: 'home',
  params: {}
})

const initial = resolve(route.path)
route.name = initial.name
route.params = initial.params

export function navigate(to, { replace = false } = {}) {
  if (typeof to !== 'string') return
  if (to === route.path) return
  if (replace) history.replaceState({}, '', to)
  else history.pushState({}, '', to)
  const r = resolve(to)
  route.path = to
  route.name = r.name
  route.params = r.params
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('mrome:navigate', { detail: to }))
  }
}

export function initRouter() {
  window.addEventListener('popstate', () => {
    const r = resolve(location.pathname)
    route.path = location.pathname
    route.name = r.name
    route.params = r.params
  })
}
