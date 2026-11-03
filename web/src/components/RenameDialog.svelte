<script>
  import { X, Pencil, Loader2 } from 'lucide-svelte'
  import Cover from './Cover.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { player } from '../lib/player.svelte.js'
  import { renameDialog, closeRename } from '../lib/ui.svelte.js'

  let newName = $state('')
  let saving = $state(false)
  let inputEl = $state(null)

  $effect(() => {
    if (!renameDialog.open) return
    newName = renameDialog.track?.title || ''
    saving = false
    const t = setTimeout(() => {
      inputEl?.focus()
      inputEl?.select()
    }, 40)
    return () => clearTimeout(t)
  })

  async function save() {
    const track = renameDialog.track
    const name = newName.trim()
    if (!track || !name || saving) return
    if (name === track.title) {
      closeRename()
      return
    }
    saving = true
    try {
      // The endpoint rewrites the whole metadata record, so send the existing
      // values along — empty artist/album would be re-labelled "未知" server-side.
      await api.patch(`/api/songs/${track.id}`, {
        title: name,
        artist: track.artist,
        albumArtist: track.albumArtist,
        album: track.album,
        genre: track.genre,
        year: track.year,
        track: track.track
      })
      // Same object the list pages hold: rename in place for instant feedback.
      track.title = name
      // Keep the player queue and the now-playing bar in sync too.
      for (const s of player.queue) {
        if (s.id === track.id) s.title = name
      }
      notify.ok(`已重命名为「${name}」`)
      window.dispatchEvent(new CustomEvent('mrome:refresh'))
      closeRename()
    } catch (e) {
      notify.err(e.message)
    } finally {
      saving = false
    }
  }

  function onkey(e) {
    if (renameDialog.open && e.key === 'Escape') closeRename()
  }
</script>

<svelte:window onkeydown={onkey} />

{#if renameDialog.open && renameDialog.track}
  <div class="overlay" role="presentation" onclick={closeRename}>
    <div
      class="panel glass-deep"
      role="dialog"
      aria-modal="true"
      aria-label="重命名歌曲"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => {
        if (e.key === 'Escape') closeRename()
        e.stopPropagation()
      }}
    >
      <header class="head">
        <div class="track">
          <Cover
            id={renameDialog.track.albumId}
            alt={renameDialog.track.album}
            rounded="rounded-md"
            class="thumb"
            size={160}
          />
          <div class="meta">
            <div class="title clamp-1">{renameDialog.track.title}</div>
            <div class="sub clamp-1">{renameDialog.track.artist}</div>
          </div>
        </div>
        <button class="icon-btn" onclick={closeRename} aria-label="关闭">
          <X size={16} strokeWidth={1.6} />
        </button>
      </header>

      <div class="body">
        <label class="field-label" for="rename-input">歌曲名称</label>
        <input
          id="rename-input"
          bind:this={inputEl}
          bind:value={newName}
          placeholder="输入新的歌曲名称…"
          autocomplete="off"
          spellcheck="false"
          maxlength={200}
          onkeydown={(e) => {
            if (e.key === 'Enter') save()
          }}
        />
      </div>

      <div class="foot">
        <button class="btn" onclick={closeRename} disabled={saving}>取消</button>
        <button class="btn btn-primary" onclick={save} disabled={saving || !newName.trim()}>
          {#if saving}
            <Loader2 size={14} class="spin" />
          {:else}
            <Pencil size={13} strokeWidth={1.8} />
          {/if}
          保存
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
    width: min(400px, 100%);
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
  .body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 18px 16px 4px;
  }
  .field-label {
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .body input {
    width: 100%;
    height: 40px;
    padding: 0 12px;
    border-radius: 10px;
    border: 1px solid var(--line);
    background: var(--bg);
    color: var(--text);
    font-size: 13px;
    font-family: var(--font-sans);
    outline: none;
  }
  .body input:focus {
    border-color: rgba(var(--accent-rgb), 0.6);
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 16px;
  }
  .foot .btn {
    height: 36px;
  }
</style>
