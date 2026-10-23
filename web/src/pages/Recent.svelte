<script>
  import { onMount } from 'svelte'
  import { History, Play } from 'lucide-svelte'
  import SongRow from '../components/SongRow.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { playQueue } from '../lib/player.svelte.js'

  let songs = $state([])
  let loading = $state(true)

  async function load() {
    try {
      const data = await api.get('/api/recent?limit=200')
      songs = data.songs || []
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    const onChanged = () => load()
    window.addEventListener('mrome:refresh', onChanged)
    return () => window.removeEventListener('mrome:refresh', onChanged)
  })
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">History</div>
      <h1 class="page-title">最近播放</h1>
      <div class="page-sub">{songs.length} 首 · 网页与客户端共用同一份记录</div>
    </div>
    {#if songs.length}
      <button class="btn btn-primary" onclick={() => playQueue(songs, 0)}>
        <Play size={14} fill="currentColor" /> 播放全部
      </button>
    {/if}
  </header>

  {#if loading && !songs.length}
    <div class="loading">正在载入…</div>
  {:else if !songs.length}
    <EmptyState
      icon={History}
      title="还没有播放记录"
      desc="播放过的歌曲会按时间倒序出现在这里，用网页或客户端播放都算。"
    />
  {:else}
    <div class="list">
      {#each songs as song, i (song.id)}
        <SongRow {song} index={i} queue={songs} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .page {
    padding-bottom: 24px;
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
</style>
