<script>
  import { onMount } from 'svelte'
  import { Play, Shuffle, Music2, Search } from 'lucide-svelte'
  import SongRow from '../components/SongRow.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { playQueue, shuffleAll } from '../lib/player.svelte.js'
  import { fmtDuration, fmtCount } from '../lib/format.js'
  import { notify } from '../lib/toast.svelte.js'

  let songs = $state([])
  let total = $state(0)
  let sort = $state('title')
  let loading = $state(true)
  let q = $state('')
  let genre = $state('')
  let genres = $state([])

  const sorts = [
    { key: 'title', label: '标题' },
    { key: 'artist', label: '艺术家' },
    { key: 'album', label: '专辑' },
    { key: 'created', label: '最近添加' }
  ]

  let timer = null
  let debounceReady = false
  let seq = 0
  const PAGE = 500

  async function load(more = false) {
    const my = ++seq
    loading = true
    try {
      const offset = more ? songs.length : 0
      const params = new URLSearchParams({ sort, limit: String(PAGE), offset: String(offset) })
      if (q.trim()) params.set('q', q.trim())
      if (genre) params.set('genre', genre)
      const data = await api.get(`/api/songs?${params}`)
      if (my !== seq) return
      const page = data.songs || []
      songs = more ? [...songs, ...page] : page
      total = data.total || songs.length
    } catch (e) {
      if (my === seq) notify.err(e.message)
    } finally {
      if (my === seq) loading = false
    }
  }

  onMount(() => {
    load()
    api
      .get('/api/genres')
      .then((d) => (genres = d.genres || []))
      .catch(() => {})
    const onRefresh = () => load()
    window.addEventListener('mrome:refresh', onRefresh)
    return () => window.removeEventListener('mrome:refresh', onRefresh)
  })

  function setSort(key) {
    if (sort === key) return
    sort = key
    load()
  }

  $effect(() => {
    q
    if (!debounceReady) {
      // onMount already loaded the initial list; only debounce later typing.
      debounceReady = true
      return
    }
    if (timer) clearTimeout(timer)
    timer = setTimeout(load, 220)
    return () => clearTimeout(timer)
  })

  const duration = $derived(songs.reduce((a, s) => a + (s.duration || 0), 0))
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">Library</div>
      <h1 class="page-title">全部歌曲</h1>
      <div class="page-sub">{fmtCount(total)} · {fmtDuration(duration)}</div>
    </div>
    <div class="toolbar">
      <div class="search-mini">
        <Search size={14} strokeWidth={1.7} />
        <input bind:value={q} placeholder="筛选…" />
      </div>
      <button class="btn" onclick={() => shuffleAll(songs)} disabled={!songs.length}>
        <Shuffle size={14} /> 随机
      </button>
      <button class="btn btn-primary" onclick={() => playQueue(songs, 0)} disabled={!songs.length}>
        <Play size={14} fill="currentColor" /> 播放全部
      </button>
    </div>
  </header>

  <div class="sortbar">
    {#each sorts as s}
      <button class="pill" class:active={sort === s.key} onclick={() => setSort(s.key)}>{s.label}</button>
    {/each}
    {#if genres.length}
      <select class="genre-select" bind:value={genre} onchange={() => load()} aria-label="按流派筛选">
        <option value="">全部流派</option>
        {#each genres as g (g.name)}
          <option value={g.name}>{g.name}</option>
        {/each}
      </select>
    {/if}
    <span class="spacer"></span>
    <span class="count faint">{songs.length} 首</span>
  </div>

  {#if loading && !songs.length}
    <div class="loading">正在载入…</div>
  {:else if !songs.length}
    <EmptyState icon={Music2} title={q ? '没有匹配的歌曲' : '还没有歌曲'} desc={q ? '换个关键词试试。' : '把音乐上传到库里，它们会自动出现在这里。'} />
  {:else}
    <div class="list">
      {#each songs as song, i (song.id)}
        <SongRow {song} index={i} queue={songs} />
      {/each}
    </div>
    {#if songs.length < total}
      <div class="more-row">
        <button class="btn" onclick={() => load(true)} disabled={loading}>
          {#if loading}正在载入…{:else}加载更多（还有 {total - songs.length} 首）{/if}
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .page {
    padding-bottom: 20px;
  }
  .search-mini {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 40px;
    padding: 0 13px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: var(--glass);
    color: var(--text-3);
  }
  .search-mini input {
    width: 120px;
    border: none;
    background: none;
    outline: none;
    color: var(--text);
    font-size: 12.5px;
    font-family: var(--font-sans);
  }
  .sortbar {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 14px;
  }
  .spacer {
    flex: 1;
  }
  .genre-select {
    height: 30px;
    margin-left: 4px;
    padding: 0 28px 0 12px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background-color: var(--glass);
    color: var(--text-2);
    font-size: 12px;
    font-family: var(--font-sans);
    cursor: pointer;
    outline: none;
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='10' viewBox='0 0 24 24' fill='none' stroke='%23999' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 10px center;
  }
  .genre-select:hover,
  .genre-select:focus-visible {
    border-color: var(--line-strong);
    color: var(--text);
  }
  .count {
    font-size: 11px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .loading {
    padding: 60px 0;
    text-align: center;
    color: var(--text-3);
    font-size: 12.5px;
  }
  .more-row {
    display: flex;
    justify-content: center;
    padding: 24px 0 8px;
  }
  @media (max-width: 860px) {
    .search-mini {
      display: none;
    }
    .page-head {
      margin-bottom: 14px;
    }
  }
</style>
