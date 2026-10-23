<script>
  import { X, ListMusic, Plus, Loader2 } from 'lucide-svelte'
  import Cover from './Cover.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { playlistPicker, closePlaylistPicker } from '../lib/ui.svelte.js'

  let playlists = $state([])
  let loading = $state(false)
  let busy = $state('')
  let creating = $state(false)
  let newName = $state('')
  let inputEl = $state(null)

  async function load() {
    loading = true
    try {
      const data = await api.get('/api/playlists')
      playlists = data.playlists || []
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    if (!playlistPicker.open) return
    newName = ''
    load()
    const t = setTimeout(() => inputEl?.focus(), 40)
    return () => clearTimeout(t)
  })

  async function addTo(p) {
    const track = playlistPicker.track
    if (!track || busy) return
    busy = p.id
    try {
      await api.post(`/api/playlists/${p.id}/songs`, { songIds: [track.id] })
      notify.ok(`已加入「${p.name}」`)
      window.dispatchEvent(new CustomEvent('mrome:playlist-changed', { detail: p.id }))
      closePlaylistPicker()
    } catch (e) {
      notify.err(e.message)
    } finally {
      busy = ''
    }
  }

  async function create() {
    const track = playlistPicker.track
    const name = newName.trim()
    if (!track || !name || creating) return
    creating = true
    try {
      const data = await api.post('/api/playlists', { name, public: true, songIds: [track.id] })
      notify.ok(`已创建「${data.playlist.name}」并加入歌曲`)
      window.dispatchEvent(new CustomEvent('mrome:playlist-changed', { detail: data.playlist.id }))
      closePlaylistPicker()
    } catch (e) {
      notify.err(e.message)
    } finally {
      creating = false
    }
  }

  function onkey(e) {
    if (playlistPicker.open && e.key === 'Escape') closePlaylistPicker()
  }
</script>

<svelte:window onkeydown={onkey} />

{#if playlistPicker.open && playlistPicker.track}
  <div class="overlay" role="presentation" onclick={closePlaylistPicker}>
    <div
      class="panel glass-deep"
      role="dialog"
      aria-modal="true"
      aria-label="加入歌单"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => {
        if (e.key === 'Escape') closePlaylistPicker()
        e.stopPropagation()
      }}
    >
      <header class="head">
        <div class="track">
          <Cover
            id={playlistPicker.track.albumId}
            alt={playlistPicker.track.album}
            rounded="rounded-md"
            class="thumb"
            size={160}
          />
          <div class="meta">
            <div class="title clamp-1">{playlistPicker.track.title}</div>
            <div class="sub clamp-1">{playlistPicker.track.artist}</div>
          </div>
        </div>
        <button class="icon-btn" onclick={closePlaylistPicker} aria-label="关闭">
          <X size={16} strokeWidth={1.6} />
        </button>
      </header>

      <div class="list scroll">
        {#if loading}
          <div class="hint">正在载入歌单…</div>
        {:else if !playlists.length}
          <div class="hint">还没有歌单，在下面创建一个。</div>
        {:else}
          {#each playlists as p (p.id)}
            <button class="row" onclick={() => addTo(p)} disabled={!!busy}>
              <ListMusic size={16} strokeWidth={1.6} class="pl-icon" />
              <span class="name clamp-1">{p.name}</span>
              <span class="count">{p.songCount} 首</span>
              {#if busy === p.id}
                <Loader2 size={14} class="spin" />
              {:else}
                <Plus size={14} />
              {/if}
            </button>
          {/each}
        {/if}
      </div>

      <div class="create">
        <input
          bind:this={inputEl}
          bind:value={newName}
          placeholder="新歌单名称…"
          autocomplete="off"
          onkeydown={(e) => {
            if (e.key === 'Enter') create()
          }}
        />
        <button class="btn btn-primary" onclick={create} disabled={creating || !newName.trim()}>
          {#if creating}
            <Loader2 size={14} class="spin" />
          {:else}
            <Plus size={14} />
          {/if}
          创建并加入
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 92;
    background: rgba(4, 5, 8, 0.5);
    backdrop-filter: blur(10px);
    display: grid;
    place-items: center;
    padding: 24px;
    animation: fade-in 0.22s var(--ease) both;
  }
  .panel {
    width: min(460px, 100%);
    max-height: min(74vh, 640px);
    border-radius: 24px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: rise 0.4s var(--ease) both;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 16px 16px 14px;
    border-bottom: 1px solid var(--line);
  }
  .track {
    display: flex;
    align-items: center;
    gap: 11px;
    min-width: 0;
  }
  :global(.thumb) {
    width: 42px;
    height: 42px;
    flex: none;
  }
  .meta {
    min-width: 0;
  }
  .title {
    font-size: 13.5px;
    font-weight: 500;
  }
  .sub {
    font-size: 11px;
    color: var(--text-3);
    margin-top: 2px;
  }
  .list {
    overflow-y: auto;
    padding: 6px 8px 10px;
    min-height: 90px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: 1px solid transparent;
    border-radius: 12px;
    background: none;
    color: var(--text);
    text-align: left;
    cursor: pointer;
    font-size: 12.5px;
  }
  .row:hover:not(:disabled) {
    background: var(--glass-2);
    border-color: var(--line);
  }
  .row:disabled {
    opacity: 0.6;
    cursor: default;
  }
  :global(.pl-icon) {
    color: var(--text-3);
    flex: none;
  }
  .name {
    flex: 1;
    min-width: 0;
  }
  .count {
    font-size: 10.5px;
    color: var(--text-3);
    white-space: nowrap;
  }
  .hint {
    padding: 30px 16px;
    text-align: center;
    font-size: 12.5px;
    color: var(--text-3);
  }
  .create {
    display: flex;
    gap: 8px;
    padding: 12px 16px 16px;
    border-top: 1px solid var(--line);
  }
  .create input {
    flex: 1;
    min-width: 0;
    height: 38px;
    padding: 0 12px;
    border-radius: 10px;
    border: 1px solid var(--line);
    background: var(--bg);
    color: var(--text);
    font-size: 12.5px;
    font-family: var(--font-sans);
    outline: none;
  }
  .create input:focus {
    border-color: rgba(var(--accent-rgb), 0.6);
  }
  .create .btn {
    flex: none;
    height: 38px;
  }
</style>
