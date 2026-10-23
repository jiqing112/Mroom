import { api } from './api.js'

export const auth = $state({
  user: null,
  ready: false,
  app: null,
  guestMode: false,
  canLogin: false
})

export async function loadMe() {
  try {
    const data = await api.get('/api/auth/me')
    auth.user = data.user
    auth.app = data.app
    auth.guestMode = !!data.guestMode
    auth.canLogin = !!data.canLogin
  } catch {
    auth.user = null
    auth.guestMode = false
    auth.canLogin = false
  } finally {
    auth.ready = true
  }
}

export async function login(username, password) {
  const data = await api.post('/api/auth/login', { username, password })
  auth.user = data.user
  auth.ready = true
  return data.user
}

export async function logout() {
  try {
    await api.post('/api/auth/logout')
  } catch {
    /* ignore */
  }
  auth.user = null
}
