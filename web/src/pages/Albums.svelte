<script>
  import { onMount } from 'svelte'
  import { Disc3, Shuffle } from 'lucide-svelte'
  import AlbumCard from '../components/AlbumCard.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { playQueue } from '../lib/player.svelte.js'
  import { fmtDuration } from '../lib/format.js'

  let albums = $state([])
  let sort = $state('newest')
  let loading = $state(true)
  let hasMore = $state(false)
  let genre = $state('')
  let genres = $state([])
  let seq = 0
  const PAGE = 500

  const sorts = [
    { key: 'newest', label: '最近添加' },
    { key: 'alphabetical', label: '名称' },
    { key: 'alphabeticalByArtist', label: '艺术家' },
    { key: 'byYear', label: '年份' }
  ]

  async function load(more = false) {
    const my = ++seq
    loading = true
    try {
      const offset = more ? albums.length : 0
      const q = genre ? `&genre=${encodeURIComponent(genre)}` : ''
      const data = await api.get(`/api/albums?sort=${sort}&limit=${PAGE}&offset=${offset}${q}`)
      if (my !== seq) return
      const page = data.albums || []
      albums = more ? [...albums, ...page] : page
      hasMore = page.length === PAGE
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

  async function shuffleAll() {
    if (!albums.length) return
    const pool = [...albums].sort(() => Math.random() - 0.5).slice(0, 20)
    const picks = []
    for (const a of pool) {
      try {
        const d = await api.get(`/api/albums/${a.id}`)
        picks.push(...(d.album?.songs || []))
      } catch {
        /* ignore */
      }
    }
    if (picks.length) {
      for (let i = picks.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1))
        ;[picks[i], picks[j]] = [picks[j], picks[i]]
      }
      playQueue(picks, 0)
    }
  }

  const duration = $derived(albums.reduce((a, b) => a + (b.duration || 0), 0))
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">Collection</div>
      <h1 class="page-title">专辑</h1>
      <div class="page-sub">{albums.length} 张专辑 · {fmtDuration(duration)}</div>
    </div>
    <div class="toolbar">
      <button class="btn" onclick={shuffleAll} disabled={!albums.length}>
        <Shuffle size={14} /> 随机播放
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
  </div>

  {#if loading && !albums.length}
    <div class="loading">正在载入…</div>
  {:else if !albums.length}
    <EmptyState icon={Disc3} title="还没有专辑" desc="上传带标签的音乐后，专辑会自动生成。" />
  {:else}
    <div class="grid-albums">
      {#each albums as album (album.id)}
        <AlbumCard {album} />
      {/each}
    </div>
    {#if hasMore}
      <div class="more-row">
        <button class="btn" onclick={() => load(true)} disabled={loading}>
          {#if loading}正在载入…{:else}加载更多专辑{/if}
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .page {
    padding-bottom: 20px;
  }
  .sortbar {
    display: flex;
    gap: 6px;
    margin-bottom: 20px;
    flex-wrap: wrap;
  }
  .loading {
    padding: 60px 0;
    text-align: center;
    color: var(--text-3);
    font-size: 12.5px;
  }
  .genre-select {
    height: 30px;
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
  .more-row {
    display: flex;
    justify-content: center;
    padding: 28px 0 8px;
  }
</style>
