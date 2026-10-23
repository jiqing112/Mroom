<script>
  import { onMount } from 'svelte'
  import { Play, Shuffle, ChevronLeft, Star } from 'lucide-svelte'
  import Cover from '../components/Cover.svelte'
  import AlbumCard from '../components/AlbumCard.svelte'
  import SongRow from '../components/SongRow.svelte'
  import Link from '../components/Link.svelte'
  import { api } from '../lib/api.js'
  import { playQueue, shuffleAll } from '../lib/player.svelte.js'
  import { fmtCount } from '../lib/format.js'
  import { notify } from '../lib/toast.svelte.js'

  let { id } = $props()
  let artist = $state(null)
  let songs = $state([])
  let total = $state(0)
  let loading = $state(true)
  let seq = 0
  const PAGE = 500

  async function load(more = false) {
    const my = ++seq
    loading = true
    try {
      const offset = more ? songs.length : 0
      const data = await api.get(`/api/artists/${id}?limit=${PAGE}&offset=${offset}`)
      if (my !== seq) return
      artist = data.artist
      const page = data.songs || []
      songs = more ? [...songs, ...page] : page
      total = data.total || songs.length
    } catch (e) {
      if (my === seq) notify.err(e.message)
    } finally {
      if (my === seq) loading = false
    }
  }

  $effect(() => {
    id
    songs = []
    total = 0
    load()
  })

  onMount(() => {
    const onRefresh = () => load()
    window.addEventListener('mrome:refresh', onRefresh)
    return () => window.removeEventListener('mrome:refresh', onRefresh)
  })

  async function toggleStar() {
    if (!artist) return
    const next = !artist.starred
    artist.starred = next
    try {
      await api.post('/api/star', { id: artist.id, type: 'artist', starred: next })
      notify.ok(next ? '已收藏艺术家' : '已取消收藏')
      window.dispatchEvent(new CustomEvent('mrome:stars'))
    } catch (e) {
      artist.starred = !next
      notify.err(e.message)
    }
  }
</script>

<div class="page rise">
  <Link href="/artists" class="back"><ChevronLeft size={13} /> 返回艺术家</Link>

  {#if loading && !artist}
    <div class="loading">正在载入…</div>
  {:else if artist}
    <header class="head">
      <div class="avatar">
        <Cover id={artist.coverArt || artist.id} alt={artist.name} rounded="rounded-full" class="cover" eager sizes="(max-width: 760px) 42vw, 210px" />
      </div>
      <div class="info">
        <div class="eyebrow">Artist</div>
        <h1 class="title-tight">{artist.name}</h1>
        <div class="meta">
          <span>{fmtCount(artist.albumCount, '张专辑')}</span>
          <span class="sep">·</span>
          <span>{fmtCount(artist.songCount)}</span>
        </div>
        <div class="actions">
          <button class="btn btn-primary" onclick={() => playQueue(songs, 0)} disabled={!songs.length}>
            <Play size={15} fill="currentColor" /> 播放
          </button>
          <button class="btn" onclick={() => shuffleAll(songs)} disabled={!songs.length}>
            <Shuffle size={14} /> 随机
          </button>
          <button
            class="icon-btn"
            class:active={artist.starred}
            onclick={toggleStar}
            title="收藏艺术家"
            aria-label="收藏艺术家"
          >
            <Star size={16} strokeWidth={1.6} fill={artist.starred ? 'currentColor' : 'none'} />
          </button>
        </div>
      </div>
    </header>

    {#if artist.albums?.length}
      <section class="section">
        <div class="section-head"><h2>专辑</h2></div>
        <div class="grid-albums">
          {#each artist.albums as album (album.id)}
            <AlbumCard {album} />
          {/each}
        </div>
      </section>
    {/if}

    {#if songs.length}
      <section class="section">
        <div class="section-head">
          <h2>歌曲</h2>
          <span class="faint">{total} 首</span>
        </div>
        <div class="list">
          {#each songs as song, i (song.id)}
            <SongRow {song} index={i} queue={songs} ondelete={() => load()} />
          {/each}
        </div>
        {#if songs.length < total}
          <div class="more-row">
            <button class="btn" onclick={() => load(true)} disabled={loading}>
              {#if loading}正在载入…{:else}加载更多（还有 {total - songs.length} 首）{/if}
            </button>
          </div>
        {/if}
      </section>
    {/if}
  {:else}
    <div class="loading">艺术家不存在</div>
  {/if}
</div>

<style>
  .page {
    padding-bottom: 24px;
  }
  .head {
    display: grid;
    grid-template-columns: minmax(150px, 210px) minmax(0, 1fr);
    gap: clamp(22px, 4vw, 44px);
    align-items: center;
    margin-bottom: 46px;
  }
  .avatar {
    position: relative;
    width: 100%;
  }
  .avatar :global(.cover) {
    width: 100%;
    aspect-ratio: 1;
    box-shadow: 0 40px 80px -40px rgba(0, 0, 0, 0.9), 0 0 0 1px var(--line);
  }
  .avatar :global(.cover img) {
    transform: scale(1.45);
  }
  .title-tight {
    font-size: clamp(2rem, 4vw, 3.3rem);
    font-weight: 500;
    margin: 12px 0 12px;
    overflow-wrap: anywhere;
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
  }
  .section {
    margin-top: 34px;
  }
  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-bottom: 16px;
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
    padding: 80px 0;
    text-align: center;
    color: var(--text-3);
  }
  .more-row {
    display: flex;
    justify-content: center;
    padding: 24px 0 8px;
  }
  @media (max-width: 760px) {
    .head {
      grid-template-columns: 1fr;
      gap: 18px;
      margin-bottom: 30px;
    }
    .avatar {
      width: 42vw;
      max-width: 180px;
    }
    .actions {
      margin-top: 18px;
    }
  }
</style>
