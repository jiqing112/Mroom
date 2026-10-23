<script>
  import {
    Play,
    Star,
    Trash2,
    MoreHorizontal,
    CornerDownRight,
    ListPlus,
    ListMusic
  } from 'lucide-svelte'
  import Cover from './Cover.svelte'
  import Rating from './Rating.svelte'
  import {
    player,
    playTrack,
    toggle,
    addToQueue,
    currentTrack,
    dropSongFromQueue
  } from '../lib/player.svelte.js'
  import { openPlaylistPicker } from '../lib/ui.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { fmtTime } from '../lib/format.js'

  let {
    song,
    index = 0,
    queue = [],
    showAlbum = true,
    showCover = true,
    onstar = null,
    onremove = null,
    ondelete = null
  } = $props()

  const current = $derived(currentTrack()?.id === song.id)
  const playing = $derived(current && player.playing)
  const isAdmin = $derived(!!auth.user?.isAdmin)

  let menuOpen = $state(false)
  let menuEl = $state(null)

  $effect(() => {
    if (!menuOpen) return
    const onDoc = (e) => {
      if (menuEl && !menuEl.contains(e.target)) menuOpen = false
    }
    const onKey = (e) => {
      if (e.key === 'Escape') menuOpen = false
    }
    document.addEventListener('click', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('click', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  })

  function play() {
    if (current) toggle()
    else playTrack(song, queue, index)
  }

  async function star(e) {
    e.stopPropagation()
    const next = !song.starred
    song.starred = next
    try {
      await api.post('/api/star', { id: song.id, type: 'song', starred: next })
      notify.ok(next ? '已收藏' : '已取消收藏')
      onstar?.(song, next)
      window.dispatchEvent(new CustomEvent('mrome:stars'))
    } catch (err) {
      song.starred = !next
      notify.err(err.message)
    }
  }

  function toggleMenu(e) {
    e.stopPropagation()
    menuOpen = !menuOpen
  }

  async function rate(n) {
    const prev = song.rating || 0
    if (n === prev) return
    song.rating = n
    try {
      await api.post('/api/rating', { id: song.id, rating: n })
      notify.ok(n ? `已评分 ${n} 星` : '已清除评分')
    } catch (err) {
      song.rating = prev
      notify.err(err.message)
    }
  }

  function enqueue() {
    addToQueue(song)
    notify.ok('已加入播放队列')
  }

  function playNext() {
    addToQueue(song, true)
    notify.ok('将在下一首播放')
  }

  function addToPlaylist() {
    openPlaylistPicker(song)
  }

  async function removeFromLibrary() {
    menuOpen = false
    if (!confirm(`确定删除「${song.title}」？文件会移入回收站。`)) return
    try {
      await api.del(`/api/songs/${song.id}`)
      notify.ok('歌曲已移入回收站')
      dropSongFromQueue(song.id)
      if (ondelete) {
        ondelete(song)
      } else {
        const i = queue.findIndex((s) => s.id === song.id)
        if (i >= 0) queue.splice(i, 1)
      }
      window.dispatchEvent(new CustomEvent('mrome:refresh'))
      window.dispatchEvent(new CustomEvent('mrome:song-deleted', { detail: song.id }))
    } catch (err) {
      notify.err(err.message)
    }
  }
</script>

<div
  class="song-row"
  class:no-cover={!showCover}
  class:no-album={!showAlbum}
  class:current
  class:menu-open={menuOpen}
  role="button"
  tabindex="0"
  onclick={play}
  onkeydown={(e) => {
    if (e.target !== e.currentTarget) return
    if (e.key === 'Enter') play()
  }}
>
  <div class="idx">
    {#if current}
      <span class="playing-bars" class:paused={!playing}><i></i><i></i><i></i></span>
    {:else}
      <span class="num">{String(index + 1).padStart(2, '0')}</span>
      <Play size={11} fill="currentColor" class="hover-play" />
    {/if}
  </div>

  {#if showCover}
    <Cover id={song.albumId} alt={song.album} rounded="rounded-lg" class="row-cover" size={160} />
  {/if}

  <div class="main">
    <div class="title clamp-1">{song.title}</div>
    <div class="sub clamp-1">{song.artist}</div>
  </div>

  {#if showAlbum}
    <div class="col-album faint clamp-1">{song.album}</div>
  {/if}

  <div class="col-dur faint">{fmtTime(song.duration)}</div>

  <div class="row-actions">
    <button
      class="mini"
      class:starred={song.starred}
      onclick={star}
      aria-label="收藏"
      title="收藏"
    >
      <Star size={14} strokeWidth={1.6} fill={song.starred ? 'currentColor' : 'none'} />
    </button>
    <button
      class="mini"
      onclick={(e) => {
        e.stopPropagation()
        enqueue()
      }}
      aria-label="加入播放队列"
      title="加入播放队列"
    >
      <ListMusic size={14} strokeWidth={1.6} />
    </button>
    <div class="menu-wrap" bind:this={menuEl}>
      <button
        class="mini"
        onclick={toggleMenu}
        aria-label="更多操作"
        title="更多操作"
        aria-haspopup="menu"
        aria-expanded={menuOpen}
      >
        <MoreHorizontal size={16} strokeWidth={1.7} />
      </button>
      {#if menuOpen}
        <div
          class="menu glass-deep"
          role="menu"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => {
            if (e.key === 'Escape') menuOpen = false
            e.stopPropagation()
          }}
        >
          <div class="menu-rating">
            <span>评分</span>
            <Rating value={song.rating || 0} onrate={rate} size={13} />
          </div>
          <button class="menu-item" role="menuitem" onclick={(e) => { e.stopPropagation(); menuOpen = false; playNext() }}>
            <CornerDownRight size={14} strokeWidth={1.6} /> 下一首播放
          </button>
          <button class="menu-item" role="menuitem" onclick={(e) => { e.stopPropagation(); menuOpen = false; addToPlaylist() }}>
            <ListPlus size={14} strokeWidth={1.6} /> 加入歌单
          </button>
          {#if onremove}
            <button class="menu-item" role="menuitem" onclick={(e) => { e.stopPropagation(); menuOpen = false; onremove(song) }}>
              <Trash2 size={14} strokeWidth={1.6} /> 从歌单移除
            </button>
          {/if}
          {#if isAdmin}
            <button class="menu-item danger" role="menuitem" onclick={(e) => { e.stopPropagation(); removeFromLibrary() }}>
              <Trash2 size={14} strokeWidth={1.6} /> 删除歌曲
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .song-row {
    user-select: none;
  }
  .idx {
    position: relative;
    height: 16px;
    display: grid;
    place-items: center;
  }
  .num {
    font-size: 11.5px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  :global(.hover-play) {
    position: absolute;
    opacity: 0;
    color: var(--text);
    transition: opacity 0.2s var(--ease);
  }
  .song-row:hover .num {
    opacity: 0;
  }
  .song-row:hover :global(.hover-play) {
    opacity: 1;
  }
  :global(.row-cover) {
    width: 40px;
    height: 40px;
  }
  .song-row.no-cover {
    grid-template-columns: 34px minmax(0, 2.4fr) minmax(0, 1.4fr) 62px 84px;
  }
  .song-row.no-album {
    grid-template-columns: 34px 44px minmax(0, 2.4fr) 62px 84px;
  }
  .song-row.no-cover.no-album {
    grid-template-columns: 34px minmax(0, 2.4fr) 62px 84px;
  }
  .main {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .title {
    font-size: 13.5px;
    font-weight: 450;
    letter-spacing: -0.01em;
  }
  .current .title {
    color: var(--accent);
    font-weight: 500;
  }
  .sub {
    font-size: 11.5px;
    color: var(--text-2);
  }
  .col-album {
    font-size: 11.5px;
    min-width: 0;
  }
  .col-dur {
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
    text-align: right;
  }
  .row-actions {
    display: flex;
    align-items: center;
    gap: 2px;
    justify-content: flex-end;
    position: relative;
  }
  .menu-wrap {
    position: relative;
    display: flex;
  }
  .menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    z-index: 45;
    min-width: 190px;
    padding: 6px;
    border-radius: 14px;
    display: flex;
    flex-direction: column;
    gap: 1px;
    animation: rise 0.18s var(--ease) both;
  }
  .menu-rating {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 4px 8px 8px;
    margin-bottom: 3px;
    border-bottom: 1px solid var(--line);
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    padding: 8px 9px;
    border: none;
    border-radius: 8px;
    background: none;
    color: var(--text-2);
    font: inherit;
    font-size: 12.5px;
    text-align: left;
    cursor: pointer;
    white-space: nowrap;
  }
  .menu-item:hover {
    background: var(--glass-2);
    color: var(--text);
  }
  .menu-item.danger {
    color: #fb7185;
  }
  .menu-item.danger:hover {
    background: rgba(251, 113, 133, 0.1);
  }
  @media (max-width: 860px) {
    .song-row {
      grid-template-columns: 26px 40px minmax(0, 1fr) 80px;
    }
    .song-row.no-cover,
    .song-row.no-cover.no-album {
      grid-template-columns: 26px minmax(0, 1fr) 80px;
    }
  }
  .mini {
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-2);
    cursor: pointer;
    transition: all 0.2s var(--ease);
  }
  .mini:hover {
    color: var(--text);
    background: var(--glass-2);
  }
  .mini.starred {
    color: var(--accent);
  }
</style>
