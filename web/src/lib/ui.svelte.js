import { route, navigate } from './router.svelte.js'

export const ui = $state({
  settingsOpen: false,
  searchOpen: false,
  stageOpen: false,
  queueOpen: false,
  lyricsMode: false,
  mobileNav: false
})

// Global "add to playlist" dialog, opened from any song row.
export const playlistPicker = $state({ open: false, track: null })

export function openPlaylistPicker(track) {
  if (!track) return
  playlistPicker.track = track
  playlistPicker.open = true
}

export function closePlaylistPicker() {
  playlistPicker.open = false
  playlistPicker.track = null
}

export function openSettings() {
  ui.settingsOpen = true
}
export function closeSettings() {
  ui.settingsOpen = false
  // /settings only exists as a deep-link into this modal — drop it from the
  // address bar on close so back/forward and refreshes stay honest.
  if (route.path === '/settings') navigate('/', { replace: true })
}
export function openSearch() {
  ui.searchOpen = true
}
export function closeSearch() {
  ui.searchOpen = false
}
export function openStage(lyrics = false) {
  ui.stageOpen = true
  ui.lyricsMode = lyrics
  ui.queueOpen = false
}
// The PlayerBar's queue button opens the stage directly on its queue tab.
export function openStageQueue() {
  ui.stageOpen = true
  ui.lyricsMode = false
  ui.queueOpen = true
}
export function closeStage() {
  ui.stageOpen = false
}
