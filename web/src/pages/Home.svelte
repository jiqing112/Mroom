<script>
  import { onMount } from 'svelte'
  import { Play, Pause, Mic2, ArrowRight, Disc3, Sparkles, Loader2 } from 'lucide-svelte'
  import Cover from '../components/Cover.svelte'
  import AlbumCard from '../components/AlbumCard.svelte'
  import SongRow from '../components/SongRow.svelte'
  import Link from '../components/Link.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { player, currentTrack, toggle, playQueue } from '../lib/player.svelte.js'
  import { openStage } from '../lib/ui.svelte.js'
  import { fmtDuration, fmtTime } from '../lib/format.js'
  import { navigate } from '../lib/router.svelte.js'
  import { notify } from '../lib/toast.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import Slider from '../components/Slider.svelte'
  import { seek } from '../lib/player.svelte.js'

  let data = $state(null)
  let loading = $state(true)
  let heroBusy = $state(false)

  const track = $derived(currentTrack())
  const featured = $derived(data?.newest?.[0] || null)
  const hero = $derived(
    track
      ? {
          title: track.title,
          artist: track.artist,
          album: track.album,
          albumId: track.albumId,
          year: track.year,
          duration: track.duration,
          live: true
        }
      : featured
        ? {
            title: featured.name,
            artist: featured.artist,
            album: `${featured.songCount} 首曲目`,
            albumId: featured.id,
            year: featured.year,
            duration: featured.duration,
            live: false
          }
        : null
  )

  const duration = $derived(player.duration || hero?.duration || 0)

  async function load() {
    try {
      data = await api.get('/api/home')
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

  async function heroPlay() {
    if (track) {
      toggle()
      return
    }
    if (!featured) return
    heroBusy = true
    try {
      const res = await api.get(`/api/albums/${featured.id}`)
      if (res.album?.songs?.length) playQueue(res.album.songs, 0)
    } catch (e) {
      notify.err(e.message)
    } finally {
      heroBusy = false
    }
  }
</script>

<div class="page rise">
  {#if loading}
    <div class="hero skeleton"></div>
  {:else if !hero}
    <EmptyState icon={Disc3} title="音乐库还是空的" desc="上传第一首歌，或把音频文件放进音乐目录后执行扫描。">
      {#if auth.user?.isAdmin}
        <button class="btn btn-primary" onclick={() => navigate('/upload')}>上传音乐</button>
      {/if}
    </EmptyState>
  {:else}
    <section class="hero">
      <div class="hero-bg" aria-hidden="true">
        <Cover id={hero.albumId} alt="" rounded="rounded-none" class="hero-bg-img" eager sizes="320px" />
      </div>

      <div class="hero-copy">
        <div class="eyebrow">
          {#if hero.live}<span class="live-dot"></span> Now playing{:else}Start here{/if}
        </div>
        <h1 class="title-tight">{hero.title}</h1>
        <div class="hero-meta">
          <span class="artist">{hero.artist}</span>
          <span class="sep">·</span>
          <span class="faint">{hero.album}</span>
          {#if hero.year}<span class="sep">·</span><span class="faint">{hero.year}</span>{/if}
        </div>

        {#if hero.live}
          <div class="hero-progress">
            <Slider value={player.time} max={duration} onseek={seek} label="播放进度" />
            <div class="times"><span>{fmtTime(player.time)}</span><span>{fmtTime(duration)}</span></div>
          </div>
        {/if}

        <div class="hero-actions">
          <button class="btn btn-primary" onclick={heroPlay} disabled={heroBusy}>
            {#if heroBusy}
              <Loader2 size={15} class="spin" />
            {:else if hero.live && player.playing}
              <Pause size={15} fill="currentColor" />
            {:else}
              <Play size={15} fill="currentColor" />
            {/if}
            {hero.live && player.playing ? '暂停' : '播放'}
          </button>
          {#if hero.albumId}
            <button class="btn" onclick={() => navigate(`/albums/${hero.albumId}`)}>查看专辑</button>
          {/if}
          {#if hero.live}
            <button class="icon-btn" onclick={() => openStage(true)} title="歌词" aria-label="歌词">
              <Mic2 size={16} strokeWidth={1.6} />
            </button>
          {/if}
        </div>

        {#if data?.stats}
          <div class="hero-stats">
            <div><b>{data.stats.songs}</b><span>曲目</span></div>
            <i></i>
            <div><b>{data.stats.albums}</b><span>专辑</span></div>
            <i></i>
            <div><b>{data.stats.artists}</b><span>艺术家</span></div>
            <i></i>
            <div><b>{fmtDuration(data.stats.duration)}</b><span>总时长</span></div>
          </div>
        {/if}
      </div>

      <div class="hero-art">
        <div class="halo" aria-hidden="true"></div>
        <div class="vinyl {player.playing && hero.live ? 'playing' : ''}">
          <div class="disc">
            <div class="label">
              <Cover id={hero.albumId} alt={hero.title} rounded="rounded-full" eager sizes="(max-width: 900px) 52vw, 330px" />
            </div>
            <div class="hole"></div>
          </div>
          <div class="sheen"></div>
        </div>
        <button class="stage-btn" onclick={() => openStage(false)} aria-label="打开播放舞台">
          <Sparkles size={14} strokeWidth={1.6} />
          <span>沉浸模式</span>
        </button>
      </div>
    </section>

    {#if data?.newest?.length}
      <section class="shelf">
        <div class="shelf-head">
          <h2>最近添加</h2>
          <Link href="/albums" class="more">全部专辑 <ArrowRight size={13} /></Link>
        </div>
        <div class="row scroll">
          {#each data.newest as album (album.id)}
            <AlbumCard {album} size="sm" />
          {/each}
        </div>
      </section>
    {/if}

    {#if data?.recent?.length}
      <section class="shelf">
        <div class="shelf-head">
          <h2>最近播放</h2>
        </div>
        <div class="row scroll">
          {#each data.recent as album (album.id)}
            <AlbumCard {album} size="sm" />
          {/each}
        </div>
      </section>
    {/if}

    {#if data?.recentSongs?.length}
      <section class="shelf">
        <div class="shelf-head">
          <h2>全部歌曲</h2>
          <Link href="/songs" class="more">查看全部 <ArrowRight size={13} /></Link>
        </div>
        <div class="list">
          {#each data.recentSongs as song, i (song.id)}
            <SongRow {song} index={i} queue={data.recentSongs} />
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 44px;
    padding-bottom: 20px;
  }
  .hero {
    position: relative;
    border-radius: 32px;
    overflow: hidden;
    border: 1px solid var(--line);
    background: linear-gradient(150deg, var(--glass-2), var(--glass));
    display: grid;
    grid-template-columns: minmax(0, 1.05fr) minmax(280px, 0.95fr);
    align-items: center;
    gap: 30px;
    padding: clamp(26px, 3.6vw, 54px);
    min-height: 430px;
    box-shadow: inset 0 1px 0 var(--inset), var(--shadow);
  }
  .hero-bg {
    position: absolute;
    inset: 0;
    z-index: 0;
    opacity: 0.35;
  }
  :global(.hero-bg-img) {
    position: absolute;
    inset: -20%;
    width: 140%;
    height: 140%;
    filter: blur(80px) saturate(160%);
    transform: scale(1.1);
  }
  .hero::after {
    content: '';
    position: absolute;
    inset: 0;
    z-index: 1;
    background: linear-gradient(100deg, var(--bg) 8%, rgba(0, 0, 0, 0.25) 55%, transparent);
    opacity: 0.82;
  }
  :global([data-theme='light']) .hero::after {
    background: linear-gradient(100deg, rgba(241, 238, 232, 0.94) 8%, rgba(241, 238, 232, 0.6) 55%, transparent);
    opacity: 1;
  }
  .hero-copy {
    position: relative;
    z-index: 2;
    min-width: 0;
  }
  .live-dot {
    display: inline-block;
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 10px var(--accent);
    margin-right: 8px;
    vertical-align: middle;
    animation: breathe 2.4s ease-in-out infinite;
  }
  .hero h1 {
    font-size: clamp(2.1rem, 4.2vw, 3.7rem);
    font-weight: 500;
    margin: 16px 0 12px;
    max-width: 15ch;
    overflow-wrap: anywhere;
  }
  .hero-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    font-size: 13.5px;
    color: var(--text-2);
  }
  .artist {
    color: var(--text);
  }
  .sep {
    color: var(--text-3);
  }
  .hero-progress {
    max-width: 420px;
    margin-top: 24px;
  }
  .times {
    display: flex;
    justify-content: space-between;
    font-size: 10px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
    margin-top: -2px;
  }
  .hero-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 24px;
  }
  .hero-stats {
    display: flex;
    align-items: center;
    gap: 18px;
    margin-top: 34px;
    flex-wrap: wrap;
  }
  .hero-stats div {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }
  .hero-stats b {
    font-size: 13px;
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }
  .hero-stats span {
    font-size: 10.5px;
    color: var(--text-3);
    letter-spacing: 0.04em;
  }
  .hero-stats i {
    width: 1px;
    height: 12px;
    background: var(--line-strong);
  }
  .hero-art {
    position: relative;
    z-index: 2;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }
  .halo {
    position: absolute;
    width: 78%;
    aspect-ratio: 1;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -58%);
    background: radial-gradient(circle, rgba(var(--accent-rgb), 0.45), transparent 62%);
    filter: blur(44px);
    animation: breathe 8s ease-in-out infinite;
  }
  .vinyl {
    width: min(36vw, 330px);
    min-width: 220px;
    z-index: 1;
  }
  .stage-btn {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 32px;
    padding: 0 14px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: var(--glass-2);
    color: var(--text-2);
    font-size: 11px;
    letter-spacing: 0.04em;
    cursor: pointer;
    transition: all 0.3s var(--ease);
  }
  .stage-btn:hover {
    color: var(--text);
    border-color: var(--line-strong);
    transform: translateY(-1px);
  }
  .shelf {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .shelf-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
  }
  .shelf-head h2 {
    font-size: 18px;
    font-weight: 500;
    letter-spacing: -0.02em;
    margin: 0;
  }
  :global(.more) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11.5px;
    color: var(--text-3);
    text-decoration: none;
    transition: color 0.2s var(--ease);
  }
  :global(.more):hover {
    color: var(--text);
  }
  .row {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 172px;
    gap: 16px;
    overflow-x: auto;
    padding: 2px 2px 10px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .skeleton {
    min-height: 430px;
    background: linear-gradient(90deg, var(--glass), var(--glass-2), var(--glass));
    background-size: 200% 100%;
    animation: shimmer 1.6s linear infinite;
  }
  @keyframes shimmer {
    to {
      background-position: -200% 0;
    }
  }
  @media (max-width: 900px) {
    .hero {
      grid-template-columns: 1fr;
      padding: 24px 22px 28px;
      min-height: 0;
      gap: 20px;
    }
    .hero-art {
      order: -1;
    }
    .vinyl {
      width: min(52vw, 220px);
    }
    .hero h1 {
      font-size: clamp(1.8rem, 7vw, 2.4rem);
      margin: 12px 0 10px;
    }
    .hero-stats {
      margin-top: 22px;
      gap: 12px;
    }
    .row {
      grid-auto-columns: 148px;
      gap: 12px;
    }
    .page {
      gap: 32px;
    }
  }
  /* smooth mode: flat wash instead of a blurred cover + breathing halo */
  :global([data-perf='smooth']) .hero-bg,
  :global([data-perf='smooth']) .halo {
    display: none;
  }
  :global([data-perf='smooth']) .hero {
    background:
      radial-gradient(88% 118% at 6% 0%, rgba(var(--accent-rgb), 0.26), transparent 58%),
      linear-gradient(158deg, rgba(var(--accent-2-rgb), 0.13), transparent 52%),
      var(--glass-2);
  }
</style>
