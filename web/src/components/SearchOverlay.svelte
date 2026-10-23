<script>
  import { Search, X, Music2, Disc3, Users, CornerDownLeft } from 'lucide-svelte'
  import { ui, closeSearch } from '../lib/ui.svelte.js'
  import { api } from '../lib/api.js'
  import { navigate } from '../lib/router.svelte.js'
  import { playTrack } from '../lib/player.svelte.js'
  import { fmtTime } from '../lib/format.js'
  import Cover from './Cover.svelte'

  let q = $state('')
  let inputEl = $state(null)
  let results = $state({ songs: [], albums: [], artists: [] })
  let loading = $state(false)
  let cursor = $state(0)
  let timer = null
  let seq = 0

  const flat = $derived([
    ...results.artists.map((a) => ({ type: 'artist', item: a })),
    ...results.albums.map((a) => ({ type: 'album', item: a })),
    ...results.songs.map((s) => ({ type: 'song', item: s }))
  ])

  const total = $derived(flat.length)

  $effect(() => {
    if (ui.searchOpen) {
      setTimeout(() => inputEl?.focus(), 40)
    } else {
      q = ''
      results = { songs: [], albums: [], artists: [] }
      cursor = 0
    }
  })

  $effect(() => {
    if (!ui.searchOpen) return
    const onKey = (e) => e.key === 'Escape' && closeSearch()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  $effect(() => {
    const query = q.trim()
    if (timer) clearTimeout(timer)
    if (!query) {
      results = { songs: [], albums: [], artists: [] }
      loading = false
      return
    }
    loading = true
    timer = setTimeout(async () => {
      const my = ++seq
      try {
        const data = await api.get(`/api/search?q=${encodeURIComponent(query)}`)
        if (my !== seq) return
        results = { songs: data.songs || [], albums: data.albums || [], artists: data.artists || [] }
        cursor = 0
      } catch {
        if (my === seq) results = { songs: [], albums: [], artists: [] }
      } finally {
        if (my === seq) loading = false
      }
    }, 170)
  })

  function choose(entry) {
    if (!entry) return
    if (entry.type === 'song') {
      playTrack(entry.item, results.songs, results.songs.findIndex((s) => s.id === entry.item.id))
    } else if (entry.type === 'album') {
      navigate(`/albums/${entry.item.id}`)
    } else {
      navigate(`/artists/${entry.item.id}`)
    }
    closeSearch()
  }

  function onKey(e) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      cursor = Math.min(total - 1, cursor + 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      cursor = Math.max(0, cursor - 1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      choose(flat[cursor])
    }
  }

  function parts(text) {
    const query = q.trim()
    if (!query) return [{ t: text, hit: false }]
    const i = text.toLowerCase().indexOf(query.toLowerCase())
    if (i < 0) return [{ t: text, hit: false }]
    return [
      { t: text.slice(0, i), hit: false },
      { t: text.slice(i, i + query.length), hit: true },
      { t: text.slice(i + query.length), hit: false }
    ]
  }

  const offset = (type) => {
    if (type === 'artist') return 0
    if (type === 'album') return results.artists.length
    return results.artists.length + results.albums.length
  }
</script>

{#if ui.searchOpen}
  <div class="overlay" role="presentation" onclick={closeSearch}>
    <div
      class="panel glass-deep"
      role="dialog"
      aria-modal="true"
      aria-label="搜索"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => {
        if (e.key === 'Escape') closeSearch()
        e.stopPropagation()
      }}
    >
      <div class="input-row">
        <Search size={18} strokeWidth={1.6} class="icon" />
        <input
          bind:this={inputEl}
          bind:value={q}
          onkeydown={onKey}
          placeholder="搜索歌曲、专辑、艺术家…"
          autocomplete="off"
          spellcheck="false"
        />
        {#if q}
          <button class="icon-btn" onclick={() => (q = '')} aria-label="清空">
            <X size={15} strokeWidth={1.6} />
          </button>
        {:else}
          <kbd>ESC</kbd>
        {/if}
      </div>

      <div class="results scroll">
        {#if loading}
          <div class="hint-row">搜索中…</div>
        {:else if !q.trim()}
          <div class="hint-row">输入关键词开始搜索，支持标题、艺术家与专辑名。</div>
        {:else if total === 0}
          <div class="hint-row">没有找到与 “{q}” 相关的内容</div>
        {:else}
          {#if results.artists.length}
            <div class="group-label">艺术家</div>
            {#each results.artists as a, i}
              <button
                class="r-row"
                class:cursor={cursor === offset('artist') + i}
                onclick={() => choose({ type: 'artist', item: a })}
                onmouseenter={() => (cursor = offset('artist') + i)}
              >
                <Cover id={a.coverArt || a.id} alt={a.name} rounded="rounded-full" class="r-cover" size={96} />
                <div class="r-main">
                  <div class="r-title">
                    {#each parts(a.name) as p}<span class:hit={p.hit}>{p.t}</span>{/each}
                  </div>
                  <div class="r-sub">{a.albumCount} 张专辑</div>
                </div>
                <Users size={14} class="r-type" />
              </button>
            {/each}
          {/if}

          {#if results.albums.length}
            <div class="group-label">专辑</div>
            {#each results.albums as a, i}
              <button
                class="r-row"
                class:cursor={cursor === offset('album') + i}
                onclick={() => choose({ type: 'album', item: a })}
                onmouseenter={() => (cursor = offset('album') + i)}
              >
                <Cover id={a.id} alt={a.name} rounded="rounded-md" class="r-cover" size={96} />
                <div class="r-main">
                  <div class="r-title">
                    {#each parts(a.name) as p}<span class:hit={p.hit}>{p.t}</span>{/each}
                  </div>
                  <div class="r-sub">{a.artist} · {a.songCount} 首</div>
                </div>
                <Disc3 size={14} class="r-type" />
              </button>
            {/each}
          {/if}

          {#if results.songs.length}
            <div class="group-label">歌曲</div>
            {#each results.songs as s, i}
              <button
                class="r-row"
                class:cursor={cursor === offset('song') + i}
                onclick={() => choose({ type: 'song', item: s })}
                onmouseenter={() => (cursor = offset('song') + i)}
              >
                <Cover id={s.albumId} alt={s.album} rounded="rounded-md" class="r-cover" size={96} />
                <div class="r-main">
                  <div class="r-title">
                    {#each parts(s.title) as p}<span class:hit={p.hit}>{p.t}</span>{/each}
                  </div>
                  <div class="r-sub">
                    {#each parts(s.artist) as p}<span class:hit={p.hit}>{p.t}</span>{/each} · {s.album}
                  </div>
                </div>
                <span class="r-dur">{fmtTime(s.duration)}</span>
              </button>
            {/each}
          {/if}
        {/if}
      </div>

      <div class="foot">
        <span><kbd>↑</kbd><kbd>↓</kbd> 选择</span>
        <span><kbd>↵</kbd> 打开</span>
        <span><CornerDownLeft size={11} /> 播放歌曲</span>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 90;
    background: rgba(4, 5, 8, 0.52);
    backdrop-filter: blur(12px);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding: 11vh 20px 20px;
    animation: fade-in 0.22s var(--ease) both;
  }
  .panel {
    width: min(680px, 94vw);
    max-height: min(70vh, 640px);
    border-radius: 24px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: rise 0.4s var(--ease) both;
  }
  .input-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 18px;
    border-bottom: 1px solid var(--line);
  }
  :global(.icon) {
    color: var(--text-3);
    flex: none;
  }
  .input-row input {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    outline: none;
    color: var(--text);
    font-size: 16px;
    font-family: var(--font-sans);
    letter-spacing: -0.01em;
  }
  .input-row input::placeholder {
    color: var(--text-3);
  }
  kbd {
    font-family: var(--font-sans);
    font-size: 9.5px;
    padding: 2px 6px;
    border-radius: 5px;
    border: 1px solid var(--line);
    color: var(--text-3);
    background: var(--glass);
  }
  .results {
    overflow-y: auto;
    padding: 8px 8px 12px;
    min-height: 120px;
  }
  .group-label {
    font-size: 9.5px;
    letter-spacing: 0.18em;
    text-transform: uppercase;
    color: var(--text-3);
    padding: 12px 12px 6px;
  }
  .r-row {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 7px 10px;
    border-radius: 12px;
    border: 1px solid transparent;
    background: none;
    color: var(--text);
    text-align: left;
    cursor: pointer;
  }
  .r-row.cursor {
    background: var(--glass-2);
    border-color: var(--line);
  }
  :global(.r-cover) {
    width: 38px;
    height: 38px;
    flex: none;
  }
  .r-main {
    flex: 1;
    min-width: 0;
  }
  .r-title {
    font-size: 13px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .r-sub {
    font-size: 11px;
    color: var(--text-3);
    margin-top: 2px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .hit {
    color: var(--accent);
    font-weight: 600;
  }
  :global(.r-type) {
    color: var(--text-3);
    flex: none;
  }
  .r-dur {
    font-size: 10.5px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  .hint-row {
    padding: 34px 16px;
    text-align: center;
    color: var(--text-3);
    font-size: 12.5px;
  }
  .foot {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 10px 18px;
    border-top: 1px solid var(--line);
    font-size: 10.5px;
    color: var(--text-3);
  }
  .foot span {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
</style>
