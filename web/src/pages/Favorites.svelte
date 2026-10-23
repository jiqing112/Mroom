<script>
  import { onMount } from 'svelte'
  import { Heart } from 'lucide-svelte'
  import AlbumCard from '../components/AlbumCard.svelte'
  import ArtistCard from '../components/ArtistCard.svelte'
  import SongRow from '../components/SongRow.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'

  let songs = $state([])
  let albums = $state([])
  let artists = $state([])
  let loading = $state(true)

  const total = $derived(songs.length + albums.length + artists.length)

  async function load() {
    try {
      const data = await api.get('/api/stars')
      songs = data.songs || []
      albums = data.albums || []
      artists = data.artists || []
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    const onChanged = () => load()
    window.addEventListener('mrome:stars', onChanged)
    window.addEventListener('mrome:refresh', onChanged)
    return () => {
      window.removeEventListener('mrome:stars', onChanged)
      window.removeEventListener('mrome:refresh', onChanged)
    }
  })
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">Favorites</div>
      <h1 class="page-title">我的收藏</h1>
      <div class="page-sub">{total} 项收藏</div>
    </div>
  </header>

  {#if loading && !total}
    <div class="loading">正在载入…</div>
  {:else if !total}
    <EmptyState
      icon={Heart}
      title="还没有收藏"
      desc="在歌曲、专辑或艺术家上点击星标，收藏会汇总到这里。"
    />
  {:else}
    {#if songs.length}
      <section class="section">
        <div class="section-head">
          <h2>歌曲</h2>
          <span class="faint">{songs.length} 首</span>
        </div>
        <div class="list">
          {#each songs as song, i (song.id)}
            <SongRow {song} index={i} queue={songs} />
          {/each}
        </div>
      </section>
    {/if}

    {#if albums.length}
      <section class="section">
        <div class="section-head">
          <h2>专辑</h2>
          <span class="faint">{albums.length} 张</span>
        </div>
        <div class="grid-albums">
          {#each albums as album (album.id)}
            <AlbumCard {album} />
          {/each}
        </div>
      </section>
    {/if}

    {#if artists.length}
      <section class="section">
        <div class="section-head">
          <h2>艺术家</h2>
          <span class="faint">{artists.length} 位</span>
        </div>
        <div class="grid-artists">
          {#each artists as artist (artist.id)}
            <ArtistCard {artist} />
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>

<style>
  .page {
    padding-bottom: 24px;
  }
  .section {
    margin-top: 30px;
  }
  .section:first-of-type {
    margin-top: 4px;
  }
  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 14px;
  }
  .section-head h2 {
    font-size: 18px;
    font-weight: 500;
    letter-spacing: -0.02em;
    margin: 0;
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
