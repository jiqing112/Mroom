import { api } from './api.js'
import { paletteFor, applyAccent, fallbackPalette } from './colors.svelte.js'
import { notify } from './toast.svelte.js'

const KEY = 'mrome.player.v1'

export const player = $state({
  queue: [],
  index: -1,
  playing: false,
  time: 0,
  duration: 0,
  volume: 0.8,
  muted: false,
  shuffle: false,
  repeat: 'off',
  loading: false,
  error: '',
  ready: false
})

let audio = null
let pendingSeek = 0
let scrobbled = false
let lastPersist = 0
// Whether the user wants sound right now. Kept separately from player.playing
// because the browser pauses media while a page is unloading; that transient
// pause must not be persisted as "the user paused".
let shouldPlay = false
let resumeWanted = false
let resumeArmed = false
// Indices not yet played in the current shuffle cycle, plus a back-history so
// "previous" can return to tracks that were skipped to out of order.
let shuffleRemaining = new Set()
let shuffleHistory = []

function resetShuffle() {
	shuffleHistory = []
	shuffleRemaining = new Set()
	if (player.shuffle && player.queue.length > 1 && player.index >= 0) {
		for (let i = 0; i < player.queue.length; i++) {
			if (i !== player.index) shuffleRemaining.add(i)
		}
	}
}

export function currentTrack() {
  if (player.index < 0 || player.index >= player.queue.length) return null
  return player.queue[player.index]
}

export function initPlayer() {
  if (audio || typeof window === 'undefined') return
  audio = new Audio()
  audio.preload = 'auto'
  audio.volume = player.volume

  audio.addEventListener('timeupdate', () => {
    player.time = audio.currentTime
    if (!scrobbled && audio.duration > 0 && audio.currentTime > Math.min(30, audio.duration * 0.5)) {
      scrobbled = true
      const track = currentTrack()
      if (track) api.post('/api/scrobble', { id: track.id, time: Date.now() }).catch(() => {})
    }
    const now = Date.now()
    if (now - lastPersist > 4000) {
      lastPersist = now
      persist()
    }
  })
  audio.addEventListener('loadedmetadata', () => {
    player.duration = Number.isFinite(audio.duration) ? audio.duration : 0
    if (pendingSeek > 0 && pendingSeek < player.duration - 1) {
      audio.currentTime = pendingSeek
      player.time = pendingSeek
    }
    pendingSeek = 0
    player.loading = false
    if (resumeWanted) {
      resumeWanted = false
      if (shouldPlay) attemptResume()
    }
  })
  audio.addEventListener('play', () => {
    player.playing = true
  })
  audio.addEventListener('pause', () => {
    player.playing = false
  })
  audio.addEventListener('waiting', () => (player.loading = true))
  audio.addEventListener('canplay', () => (player.loading = false))
  audio.addEventListener('ended', () => next(true))
  audio.addEventListener('error', () => {
    player.loading = false
    player.playing = false
    shouldPlay = false
    player.error = '音频无法播放，请检查文件是否完整'
    notify.err(player.error)
    persist()
  })
  audio.addEventListener('volumechange', () => {
    player.muted = audio.muted
    player.volume = audio.volume
  })

  // Persist the exact position/intent when the page goes away. This makes a
  // refresh resume where it left off instead of losing the last few seconds.
  const persistNow = () => persist()
  window.addEventListener('pagehide', persistNow)
  window.addEventListener('beforeunload', persistNow)

  restore()
  setupMediaSession()
  player.ready = true
}

// If autoplay is blocked, ask for a gesture. We deliberately ignore clicks on
// interactive controls so the player's own play button toggles exactly once.
function armResumeOnGesture() {
  if (resumeArmed || typeof window === 'undefined') return
  resumeArmed = true
  const resume = (e) => {
    const el = e.target
    if (el instanceof Element && el.closest('button, a, input, select, textarea, [role="button"], [role="link"]')) {
      return
    }
    window.removeEventListener('pointerdown', resume, true)
    resumeArmed = false
    attemptResume()
  }
  window.addEventListener('pointerdown', resume, true)
  notify.info('浏览器阻止了自动播放，点击页面空白处或播放键继续')
}

function attemptResume() {
  if (!audio || !shouldPlay) return
  const p = audio.play()
  if (p && p.catch) {
    p.catch((err) => {
      player.playing = false
      player.loading = false
      if (err && err.name === 'NotAllowedError') armResumeOnGesture()
    })
  }
}

function load(track, autoplay) {
  if (!audio) initPlayer()
  if (!track) return
  player.loading = true
  player.error = ''
  player.time = 0
  player.duration = track.duration || 0
  scrobbled = false
  pendingSeek = 0
  shouldPlay = !!autoplay
  resumeWanted = false
  audio.src = `/api/stream/${track.id}`
  audio.load()
  if (autoplay) {
    const p = audio.play()
    if (p && p.catch) p.catch((err) => {
      player.playing = false
      if (err && err.name === 'NotAllowedError') armResumeOnGesture()
    })
  }
  updateMediaSession(track)
  paletteFor(track.albumId || track.coverArt || track.id).then(applyAccent)
  persist()
}

export function playTrack(track, list, index) {
  if (!audio) initPlayer()
  if (!track) return
  const cur = currentTrack()
  if (cur && cur.id === track.id) {
    toggle()
    return
  }
  if (list && list.length) {
    player.queue = list.slice()
    player.index =
      typeof index === 'number' && index >= 0 ? index : Math.max(0, list.findIndex((s) => s.id === track.id))
  } else if (cur) {
    // No surrounding list was supplied: keep the queue consistent by replacing
    // the entry that is currently playing instead of leaving it out of sync.
    player.queue.splice(player.index, 1, track)
  } else {
    player.queue = [track]
    player.index = 0
  }
  resetShuffle()
  load(track, true)
}

export function toggle() {
  if (!audio) initPlayer()
  const track = currentTrack()
  if (!track) return
  if (!audio.src) {
    load(track, true)
    return
  }
  if (audio.paused) {
    shouldPlay = true
    const p = audio.play()
    if (p && p.catch) p.catch(() => (player.playing = false))
    persist()
  } else {
    shouldPlay = false
    audio.pause()
    persist()
  }
}

export function next(auto = false) {
  if (!player.queue.length) return
  if (auto && player.repeat === 'one') {
    seek(0)
    audio.play()
    return
  }
  let i = player.index
  if (player.shuffle && player.queue.length > 1) {
    if (!shuffleRemaining.size) {
      // The whole queue has been played in this cycle.
      if (auto && player.repeat !== 'all') {
        player.playing = false
        shouldPlay = false
        audio?.pause()
        persist()
        return
      }
      for (let k = 0; k < player.queue.length; k++) {
        if (k !== player.index) shuffleRemaining.add(k)
      }
    }
    const candidates = [...shuffleRemaining]
    i = candidates[Math.floor(Math.random() * candidates.length)]
    shuffleRemaining.delete(i)
    shuffleHistory.push(player.index)
  } else {
    i += 1
    if (i >= player.queue.length) {
      if (player.repeat === 'all' || !auto) i = 0
      else {
        player.playing = false
        shouldPlay = false
        persist()
        return
      }
    }
  }
  player.index = i
  load(player.queue[i], true)
}

export function prev() {
  if (!player.queue.length) return
  if (audio && audio.currentTime > 4) {
    seek(0)
    return
  }
  if (player.shuffle && shuffleHistory.length) {
    const target = shuffleHistory.pop()
    if (player.index >= 0) shuffleRemaining.add(player.index)
    player.index = target
    load(player.queue[target], true)
    return
  }
  let i = player.index - 1
  if (i < 0) i = player.repeat === 'all' ? player.queue.length - 1 : 0
  player.index = i
  load(player.queue[i], true)
}

export function seek(t) {
  if (!audio) return
  const d = player.duration || audio.duration || 0
  const target = Math.max(0, Math.min(t, d || t))
  audio.currentTime = target
  player.time = target
  persist()
}

export function setVolume(v) {
  if (!audio) initPlayer()
  const val = Math.max(0, Math.min(1, v))
  audio.volume = val
  audio.muted = val === 0
  player.volume = val
  persist()
}

export function toggleMute() {
  if (!audio) return
  audio.muted = !audio.muted
}

// The three playback modes the UI exposes are mutually exclusive, so they are
// derived from the two low-level flags instead of being stored separately:
//   order   -> play the queue front to back, stop at the end
//   one     -> repeat the current track forever
//   shuffle -> jump around the queue and keep going after a full cycle
export const PLAY_MODES = [
  { id: 'order', label: '顺序播放' },
  { id: 'one', label: '单曲循环' },
  { id: 'shuffle', label: '随机播放' }
]

export function playMode() {
  if (player.shuffle) return 'shuffle'
  if (player.repeat === 'one') return 'one'
  return 'order'
}

export function setPlayMode(mode) {
  if (mode === 'shuffle') {
    player.shuffle = true
    player.repeat = 'all'
  } else if (mode === 'one') {
    player.shuffle = false
    player.repeat = 'one'
  } else {
    player.shuffle = false
    player.repeat = 'off'
  }
  resetShuffle()
  persist()
}

export function toggleShuffle() {
  setPlayMode(player.shuffle ? 'order' : 'shuffle')
}

export function cycleRepeat() {
  setPlayMode(player.repeat === 'one' ? 'order' : 'one')
}

export function addToQueue(track, playNext = false) {
  if (!track) return
  if (!player.queue.length) {
    playTrack(track)
    return
  }
  const at = playNext ? player.index + 1 : player.queue.length
  player.queue.splice(at, 0, track)
  resetShuffle()
  persist()
}

export function playQueue(list, startIndex = 0) {
  if (!list || !list.length) return
  player.queue = list.slice()
  player.index = Math.max(0, Math.min(startIndex, list.length - 1))
  resetShuffle()
  load(player.queue[player.index], true)
}

export function removeFromQueue(i) {
  if (i < 0 || i >= player.queue.length) return
  const wasCurrent = i === player.index
  player.queue.splice(i, 1)
  if (!player.queue.length) {
    player.index = -1
    shouldPlay = false
    audio?.pause()
    if (audio) audio.removeAttribute('src')
    player.playing = false
    player.loading = false
    player.time = 0
    player.duration = 0
  } else if (wasCurrent) {
    player.index = Math.min(i, player.queue.length - 1)
    load(player.queue[player.index], player.playing)
  } else if (i < player.index) {
    player.index -= 1
  }
  resetShuffle()
  persist()
}

export function clearQueue() {
  player.queue = []
  player.index = -1
  shouldPlay = false
  audio?.pause()
  if (audio) audio.removeAttribute('src')
  player.playing = false
  player.loading = false
  player.error = ''
  player.time = 0
  player.duration = 0
  resetShuffle()
  persist()
}

// Remove every queue entry for a deleted song without touching the rest of
// the queue. If the current track is removed, continue with its neighbour.
export function dropSongFromQueue(id) {
  if (!id || !player.queue.length) return
  const current = currentTrack()
  const wasCurrent = current?.id === id
  const oldIndex = player.index
  const filtered = player.queue.filter((s) => s.id !== id)
  if (!filtered.length) {
    clearQueue()
    return
  }
  let newIndex = oldIndex
  if (wasCurrent) {
    newIndex = Math.min(oldIndex, filtered.length - 1)
  } else if (oldIndex > 0) {
    let removedBefore = 0
    for (let i = 0; i < oldIndex; i++) {
      if (player.queue[i].id === id) removedBefore++
    }
    newIndex = Math.max(0, oldIndex - removedBefore)
  }
  player.queue = filtered
  player.index = Math.max(0, Math.min(newIndex, filtered.length - 1))
  resetShuffle()
  if (wasCurrent) {
    load(player.queue[player.index], player.playing)
  }
  persist()
}

// Pause playback without discarding the queue (used when the session ends).
export function stop() {
  if (!audio) return
  shouldPlay = false
  audio.pause()
  audio.removeAttribute('src')
  player.playing = false
  player.loading = false
  persist()
}

export function shuffleAll(list) {
  if (!list || !list.length) return
  const copy = list.slice()
  for (let i = copy.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[copy[i], copy[j]] = [copy[j], copy[i]]
  }
  player.shuffle = true
  player.repeat = 'all'
  playQueue(copy, 0)
}

function persist() {
  try {
    const track = currentTrack()
    const MAX = 300
    let queue = player.queue
    let index = player.index
    // Keep the last-played track inside the stored window even when the queue
    // is longer than the localStorage budget.
    if (queue.length > MAX) {
      const start = Math.max(0, Math.min(index - Math.floor(MAX / 2), queue.length - MAX))
      queue = queue.slice(start, start + MAX)
      index = index >= 0 ? Math.max(0, index - start) : -1
    }
    localStorage.setItem(
      KEY,
      JSON.stringify({
        queue,
        index,
        time: player.time,
        volume: player.volume,
        shuffle: player.shuffle,
        repeat: player.repeat,
        playing: shouldPlay,
        id: track?.id || ''
      })
    )
  } catch {
    /* storage may be full or blocked */
  }
}

function restore() {
  let saved = null
  try {
    saved = JSON.parse(localStorage.getItem(KEY) || 'null')
  } catch {
    saved = null
  }
  if (!saved) {
    applyAccent(['#8b7cff', '#4fd1c5'])
    return
  }
  player.queue = Array.isArray(saved.queue) ? saved.queue : []
  player.index = typeof saved.index === 'number' ? Math.min(saved.index, player.queue.length - 1) : -1
  player.volume = typeof saved.volume === 'number' ? saved.volume : 0.8
  player.shuffle = !!saved.shuffle
  player.repeat = saved.repeat || 'off'
  // Older saves could store shuffle without repeat=all; the shuffle mode is
  // defined as "keep shuffling", so fold those states into it.
  if (player.shuffle) player.repeat = 'all'
  audio.volume = player.volume
  const track = currentTrack()
  if (track) {
    audio.src = `/api/stream/${track.id}`
    player.duration = track.duration || 0
    pendingSeek = saved.time || 0
    player.time = pendingSeek
    // Old saved states have no "playing" flag. If they were past the very
    // beginning, assume the user was listening and resume; otherwise stay
    // paused. Newer saves carry the exact intent.
    const savedPlaying =
      saved.playing === undefined ? !!(saved.time > 0) : !!saved.playing
    shouldPlay = savedPlaying
    resumeWanted = savedPlaying
    const threshold = track.duration > 0 ? Math.min(30, track.duration * 0.5) : 30
    scrobbled = pendingSeek > threshold
    updateMediaSession(track)
    paletteFor(track.albumId || track.coverArt || track.id).then(applyAccent)
  } else {
    shouldPlay = false
    applyAccent(['#8b7cff', '#4fd1c5'])
  }
  resetShuffle()
}

function updateMediaSession(track) {
  if (!('mediaSession' in navigator) || !track) return
  try {
    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title,
      artist: track.artist,
      album: track.album,
      artwork: [{ src: `/api/cover/${track.albumId || track.id}/512`, sizes: '512x512', type: 'image/jpeg' }]
    })
  } catch {
    /* ignore */
  }
}

function setupMediaSession() {
  if (!('mediaSession' in navigator)) return
  const handlers = {
    play: () => toggle(),
    pause: () => toggle(),
    previoustrack: () => prev(),
    nexttrack: () => next(),
    seekbackward: (d) => seek(player.time - (d.seekOffset || 10)),
    seekforward: (d) => seek(player.time + (d.seekOffset || 10)),
    seekto: (d) => seek(d.seekTime)
  }
  for (const [name, fn] of Object.entries(handlers)) {
    try {
      navigator.mediaSession.setActionHandler(name, fn)
    } catch {
      /* unsupported action */
    }
  }
}
