<script>
  import { onMount } from 'svelte'
  import { Play, Shuffle, ChevronLeft, Trash2, ListMusic, Pencil } from 'lucide-svelte'
  import Cover from '../components/Cover.svelte'
  import SongRow from '../components/SongRow.svelte'
  import Link from '../components/Link.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { playQueue, shuffleAll } from '../lib/player.svelte.js'
  import { fmtDuration } from '../lib/format.js'
  import { notify } from '../lib/toast.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import { navigate } from '../lib/router.svelte.js'

  let { id } = $props()
  let playlist = $state(null)
  let loading = $state(true)

  const canEdit = $derived(
    playlist && (playlist.ownerId === auth.user?.id || auth.user?.isAdmin)
  )

  async function load() {
    loading = true
    try {
      const data = await api.get(`/api/playlists/${id}`)
      playlist = data.playlist
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
    const onChanged = (e) => {
      if (!e.detail || e.detail === id) load()
    }
    window.addEventListener('mrome:playlist-changed', onChanged)
    window.addEventListener('mrome:refresh', onChanged)
    return () => {
      window.removeEventListener('mrome:playlist-changed', onChanged)
      window.removeEventListener('mrome:refresh', onChanged)
    }
  })

  async function removeSong(song) {
    try {
      const data = await api.del(`/api/playlists/${id}/songs/${song.id}`)
      playlist = data.playlist
      notify.ok('已从歌单移除')
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function removePlaylist() {
    if (!confirm(`删除歌单「${playlist.name}」？`)) return
    try {
      await api.del(`/api/playlists/${id}`)
      notify.ok('歌单已删除')
      navigate('/playlists')
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function rename() {
    const name = prompt('歌单名称', playlist.name)
    if (!name || name === playlist.name) return
    try {
      const data = await api.patch(`/api/playlists/${id}`, { name })
      playlist = data.playlist
    } catch (e) {
      notify.err(e.message)
    }
  }
</script>

<div class="page rise">
  <Link href="/playlists" class="back"><ChevronLeft size={13} /> 返回歌单</Link>

  {#if loading && !playlist}
    <div class="loading">正在载入…</div>
  {:else if playlist}
    <header class="head">
      <div class="art">
        <Cover id={playlist.coverArt} alt={playlist.name} rounded="rounded-[18px]" class="cover" eager sizes="(max-width: 760px) 52vw, 260px" />
      </div>
      <div class="info">
        <div class="eyebrow">Playlist · {playlist.owner}</div>
        <div class="title-row">
          <h1 class="title-tight" ondblclick={canEdit ? rename : null}>{playlist.name}</h1>
          {#if canEdit}
            <button class="icon-btn rename" onclick={rename} title="重命名歌单" aria-label="重命名歌单">
              <Pencil size={15} strokeWidth={1.6} />
            </button>
          {/if}
        </div>
        <div class="meta">
          <span>{playlist.songCount} 首</span>
          <span class="sep">·</span>
          <span>{fmtDuration(playlist.duration)}</span>
          {#if !playlist.public}<span class="sep">·</span><span>私密</span>{/if}
        </div>
        <div class="actions">
          <button class="btn btn-primary" onclick={() => playQueue(playlist.entries, 0)} disabled={!playlist.entries?.length}>
            <Play size={15} fill="currentColor" /> 播放
          </button>
          <button class="btn" onclick={() => shuffleAll(playlist.entries)} disabled={!playlist.entries?.length}>
            <Shuffle size={14} /> 随机
          </button>
          {#if canEdit}
            <button class="icon-btn" onclick={removePlaylist} title="删除歌单">
              <Trash2 size={15} />
            </button>
          {/if}
        </div>
      </div>
    </header>

    {#if playlist.entries?.length}
      <div class="list">
        {#each playlist.entries as song, i (song.id)}
          <SongRow
            {song}
            index={i}
            queue={playlist.entries}
            onremove={canEdit ? removeSong : null}
            ondelete={() => load()}
          />
        {/each}
      </div>
    {:else}
      <EmptyState
        icon={ListMusic}
        title="歌单还是空的"
        desc="在歌曲列表的 ⋯ 菜单里选择「加入歌单」，或从 Subsonic 客户端添加；歌曲会同步到这里。"
      />
    {/if}
  {:else}
    <div class="loading">歌单不存在</div>
  {/if}
</div>

<style>
  .page {
    padding-bottom: 24px;
  }
  .head {
    display: grid;
    grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
    gap: clamp(22px, 4vw, 44px);
    align-items: end;
    margin-bottom: 40px;
  }
  .art :global(.cover) {
    width: 100%;
    aspect-ratio: 1;
    box-shadow: 0 40px 80px -40px rgba(0, 0, 0, 0.9), 0 0 0 1px var(--line);
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .title-tight {
    font-size: clamp(1.9rem, 4vw, 3.2rem);
    font-weight: 500;
    margin: 12px 0 12px;
    overflow-wrap: anywhere;
    cursor: default;
  }
  .rename {
    flex: none;
    width: 32px;
    height: 32px;
    margin-top: 4px;
  }
  .meta {
    display: flex;
    gap: 8px;
    font-size: 12.5px;
    color: var(--text-2);
  }
  .sep {
    color: var(--text-3);
  }
  .actions {
    display: flex;
    gap: 10px;
    margin-top: 24px;
    align-items: center;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .loading {
    padding: 80px 0;
    text-align: center;
    color: var(--text-3);
  }
  @media (max-width: 760px) {
    .head {
      grid-template-columns: 1fr;
      gap: 18px;
      align-items: start;
    }
    .art {
      width: 52vw;
      max-width: 190px;
    }
  }
</style>
