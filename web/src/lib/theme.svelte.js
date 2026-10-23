export const theme = $state({
  mode: typeof localStorage !== 'undefined' ? localStorage.getItem('mrome.theme') || 'dark' : 'dark'
})

export function applyTheme(mode) {
  theme.mode = mode
  document.documentElement.dataset.theme = mode
  localStorage.setItem('mrome.theme', mode)
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', mode === 'dark' ? '#07080b' : '#f1eee8')
}

export function toggleTheme() {
  applyTheme(theme.mode === 'dark' ? 'light' : 'dark')
}

export function initTheme() {
  applyTheme(theme.mode)
}
