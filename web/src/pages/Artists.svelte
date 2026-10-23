<script>
  import { onMount } from 'svelte'
  import { Users, Search } from 'lucide-svelte'
  import ArtistCard from '../components/ArtistCard.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'

  let artists = $state([])
  let loading = $state(true)
  let q = $state('')

  const filtered = $derived(
    q.trim() ? artists.filter((a) => a.name.toLowerCase().includes(q.trim().toLowerCase())) : artists
  )

  async function load() {
    try {
      const data = await api.get('/api/artists')
      artists = data.artists || []
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    const onRefresh = () => load()
    window.addEventListener('mrome:refresh', onRefresh)
    return () => window.removeEventListener('mrome:refresh', onRefresh)
  })
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">People</div>
      <h1 class="page-title">艺术家</h1>
      <div class="page-sub">{artists.length} 位艺术家</div>
    </div>
    <div class="search-mini">
      <Search size={14} strokeWidth={1.7} />
      <input bind:value={q} placeholder="搜索艺术家…" />
    </div>
  </header>

  {#if loading && !artists.length}
    <div class="loading">正在载入…</div>
  {:else if !filtered.length}
    <EmptyState icon={Users} title={q ? '没有匹配的艺术家' : '还没有艺术家'} desc={q ? '换个关键词试试。' : '上传音乐后，艺术家会自动整理到这里。'} />
  {:else}
    <div class="grid-artists">
      {#each filtered as artist (artist.id)}
        <ArtistCard {artist} />
      {/each}
    </div>
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
    padding: 0 14px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: var(--glass);
    color: var(--text-3);
  }
  .search-mini input {
    width: 150px;
    border: none;
    background: none;
    outline: none;
    color: var(--text);
    font-size: 12.5px;
    font-family: var(--font-sans);
  }
  .loading {
    padding: 60px 0;
    text-align: center;
    color: var(--text-3);
    font-size: 12.5px;
  }
</style>
