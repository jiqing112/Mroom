<script>
  import { onMount } from 'svelte'
  import { Play, Shuffle, Star, ChevronLeft, Clock, Disc3, Pencil } from 'lucide-svelte'
  import Cover from '../components/Cover.svelte'
  import SongRow from '../components/SongRow.svelte'
  import Link from '../components/Link.svelte'
  import { api } from '../lib/api.js'
  import { playQueue, shuffleAll, currentTrack, player } from '../lib/player.svelte.js'
  import { fmtDuration, fmtCount } from '../lib/format.js'
  import { notify } from '../lib/toast.svelte.js'
  import { auth } from '../lib/auth.svelte.js'

  let { id } = $props()
  let album = $state(null)
  let loading = $state(true)
  let editOpen = $state(false)
  let editSong = $state(null)

  const isCurrentAlbum = $derived(album && currentTrack()?.albumId === album.id)

  async function load() {
    loading = true
    try {
      const data = await api.get(`/api/albums/${id}`)
      album = data.album
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    id
    load()
  })

  onMount(() => {
    const onRefresh = () => load()
    window.addEventListener('mrome:refresh', onRefresh)
    return () => window.removeEventListener('mrome:refresh', onRefresh)
  })

  async function star() {
    if (!album) return
    const next = !album.starred
    album.starred = next
    try {
    await api.post('/api/star', { id: album.id, type: 'album', starred: next })
    window.dispatchEvent(new CustomEvent('mrome:stars'))
  } catch (e) {
      album.starred = !next
      notify.err(e.message)
    }
  }

  function openEdit(song) {
    editSong = { ...song }
    editOpen = true
  }

  async function saveEdit() {
    try {
      const res = await api.patch(`/api/songs/${editSong.id}`, {
        title: editSong.title,
        artist: editSong.artist,
        albumArtist: editSong.albumArtist,
        album: editSong.album,
        genre: editSong.genre,
        year: Number(editSong.year) || 0,
        track: Number(editSong.track) || 0
      })
      notify.ok('元数据已更新')
      editOpen = false
      await load()
    } catch (e) {
      notify.err(e.message)
    }
  }
</script>

<div class="page rise">
  <Link href="/albums" class="back"><ChevronLeft size={13} /> 返回专辑</Link>

  {#if loading && !album}
    <div class="loading">正在载入…</div>
  {:else if album}
    <header class="head">
      <div class="art">
        <Cover id={album.id} alt={album.name} rounded="rounded-[20px]" class="cover" eager sizes="(max-width: 760px) 56vw, 300px" />
      </div>
      <div class="info">
        <div class="eyebrow">Album {#if album.genre}· {album.genre}{/if}</div>
        <h1 class="title-tight">{album.name}</h1>
        <div class="meta">
          <Link href={`/artists/${album.artistId}`} class="al-artist">{album.artist}</Link>
          {#if album.year}<span class="sep">·</span><span>{album.year}</span>{/if}
          <span class="sep">·</span><span>{fmtCount(album.songCount)}</span>
          <span class="sep">·</span><span>{fmtDuration(album.duration)}</span>
        </div>
        <div class="actions">
          <button class="btn btn-primary" onclick={() => playQueue(album.songs, 0)}>
            <Play size={15} fill="currentColor" /> 播放
          </button>
          <button class="btn" onclick={() => shuffleAll(album.songs)}>
            <Shuffle size={14} /> 随机
          </button>
          <button class="icon-btn" class:active={album.starred} onclick={star} title="收藏专辑">
            <Star size={16} strokeWidth={1.6} fill={album.starred ? 'currentColor' : 'none'} />
          </button>
        </div>
      </div>
    </header>

    <section class="tracks">
      <div class="tracks-head">
        <span class="eyebrow">Tracks</span>
        {#if isCurrentAlbum}<span class="now">正在播放此专辑</span>{/if}
      </div>
      {#each album.songs as song, i (song.id)}
        <div class="track-wrap">
          <SongRow
            {song}
            index={i}
            queue={album.songs}
            showAlbum={false}
            showCover={false}
            ondelete={() => load()}
          />
          {#if auth.user?.isAdmin}
            <button class="edit" onclick={() => openEdit(song)} title="编辑元数据">
              <Pencil size={12} />
            </button>
          {/if}
        </div>
      {/each}
    </section>
  {:else}
    <div class="loading">专辑不存在</div>
  {/if}
</div>

{#if editOpen && editSong}
  <div class="modal" role="presentation" onclick={() => (editOpen = false)}>
    <div
      class="dialog glass-deep"
      role="dialog"
      aria-modal="true"
      aria-label="编辑元数据"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => {
        if (e.key === 'Escape') editOpen = false
        e.stopPropagation()
      }}
    >
      <h3>编辑元数据</h3>
      <label><span>标题</span><input bind:value={editSong.title} /></label>
      <label><span>艺术家</span><input bind:value={editSong.artist} /></label>
      <label><span>专辑艺术家</span><input bind:value={editSong.albumArtist} /></label>
      <label><span>专辑</span><input bind:value={editSong.album} /></label>
      <div class="row">
        <label><span>年份</span><input type="number" bind:value={editSong.year} /></label>
        <label><span>音轨</span><input type="number" bind:value={editSong.track} /></label>
      </div>
      <label><span>流派</span><input bind:value={editSong.genre} /></label>
      <div class="dialog-actions">
        <button class="btn" onclick={() => (editOpen = false)}>取消</button>
        <button class="btn btn-primary" onclick={saveEdit}>保存</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .page {
    padding-bottom: 24px;
  }
  .head {
    display: grid;
    grid-template-columns: minmax(200px, 300px) minmax(0, 1fr);
    gap: clamp(24px, 4vw, 48px);
    align-items: end;
    margin-bottom: 44px;
  }
  .art {
    position: relative;
  }
  .art :global(.cover) {
    width: 100%;
    aspect-ratio: 1;
    box-shadow: 0 40px 90px -40px rgba(0, 0, 0, 0.9), 0 0 0 1px var(--line);
  }
  .info {
    padding-bottom: 6px;
    min-width: 0;
  }
  .title-tight {
    font-size: clamp(2rem, 4.4vw, 3.6rem);
    font-weight: 500;
    margin: 12px 0 14px;
    overflow-wrap: anywhere;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    font-size: 12.5px;
    color: var(--text-2);
  }
  :global(.al-artist) {
    color: var(--text);
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: border-color 0.2s var(--ease);
  }
  :global(.al-artist):hover {
    border-color: var(--text-3);
  }
  .sep {
    color: var(--text-3);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 26px;
  }
  .tracks {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .tracks-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px 10px;
    border-bottom: 1px solid var(--line);
    margin-bottom: 8px;
  }
  .now {
    font-size: 10.5px;
    color: var(--accent);
  }
  .track-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }
  .track-wrap > :global(.song-row) {
    flex: 1;
  }
  .edit {
    position: absolute;
    right: 74px;
    width: 24px;
    height: 24px;
    border: none;
    border-radius: 6px;
    background: var(--glass-2);
    color: var(--text-3);
    display: grid;
    place-items: center;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.2s var(--ease);
  }
  .track-wrap:hover .edit {
    opacity: 1;
  }
  .loading {
    padding: 80px 0;
    text-align: center;
    color: var(--text-3);
  }
  .modal {
    position: fixed;
    inset: 0;
    z-index: 88;
    background: rgba(4, 5, 8, 0.5);
    backdrop-filter: blur(10px);
    display: grid;
    place-items: center;
    padding: 24px;
  }
  .dialog {
    width: min(420px, 100%);
    border-radius: 22px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    animation: rise 0.35s var(--ease) both;
  }
  .dialog h3 {
    margin: 0 0 6px;
    font-size: 17px;
    font-weight: 500;
  }
  .dialog label {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .dialog label span {
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .dialog input {
    height: 38px;
    border-radius: 10px;
    border: 1px solid var(--line);
    background: var(--bg);
    color: var(--text);
    padding: 0 12px;
    font-size: 13px;
    font-family: var(--font-sans);
    outline: none;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .dialog-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
  }
  @media (max-width: 760px) {
    .head {
      grid-template-columns: 1fr;
      align-items: start;
      gap: 20px;
    }
    .art {
      width: 56vw;
      max-width: 220px;
    }
    .actions {
      margin-top: 18px;
    }
    .edit {
      display: none;
    }
  }
</style>
