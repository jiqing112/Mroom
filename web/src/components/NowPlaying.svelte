<script>
  import {
    X,
    Play,
    Pause,
    SkipBack,
    SkipForward,
    Shuffle,
    Repeat,
    Repeat1,
    Volume2,
    VolumeX,
    ListMusic,
    Mic2,
    Trash2,
    Loader2
  } from 'lucide-svelte'
  import Cover from './Cover.svelte'
  import Slider from './Slider.svelte'
  import Rating from './Rating.svelte'
  import {
    player,
    currentTrack,
    toggle,
    next,
    prev,
    seek,
    setPlayMode,
    playMode,
    setVolume,
    toggleMute,
    playTrack,
    removeFromQueue,
    clearQueue
  } from '../lib/player.svelte.js'
  import { ui, closeStage } from '../lib/ui.svelte.js'
  import { fmtTime } from '../lib/format.js'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'
  import { parseLyrics, activeLyricIndex } from '../lib/lyrics.js'
  import { fade } from 'svelte/transition'

  const track = $derived(currentTrack())
  const duration = $derived(player.duration || track?.duration || 0)
  // Same three mutually exclusive modes as the player bar, so both surfaces
  // can never disagree about what the queue will do next.
  const mode = $derived(playMode())

  let lyrics = $state([])
  let lyricsLoading = $state(false)
  let lyricsEl = $state(null)
  let lineRefs = $state([])
  let tab = $state('lyrics')
  let dragY = $state(0)
  let startY = 0
  let dragging = false

  const active = $derived(activeLyricIndex(lyrics, player.time * 1000))

  // Opening the stage with an explicit intent (lyrics / queue) selects that tab.
  $effect(() => {
    if (!ui.stageOpen) return
    if (ui.queueOpen) tab = 'queue'
    else if (ui.lyricsMode) tab = 'lyrics'
  })

  $effect(() => {
    const t = track
    if (!t || !ui.stageOpen) return
    let cancelled = false
    lyrics = []
    lineRefs = []
    lyricsLoading = true
    api
      .get(`/api/songs/${t.id}/lyrics`)
      .then((d) => {
        if (cancelled) return
        lyrics = parseLyrics(d.lyrics || '')
      })
      .catch(() => {
        if (!cancelled) lyrics = []
      })
      .finally(() => {
        if (!cancelled) lyricsLoading = false
      })
    return () => {
      cancelled = true
    }
  })

  $effect(() => {
    const i = active
    if (!ui.stageOpen || !lyricsEl || i < 0) return
    const el = lineRefs[i]
    if (!el) return
    const top = el.offsetTop - lyricsEl.clientHeight / 2 + el.offsetHeight / 2
    lyricsEl.scrollTo({ top: Math.max(0, top), behavior: 'smooth' })
  })

  function onKey(e) {
    if (e.key === 'Escape') closeStage()
  }

  function touchStart(e) {
    if (!e.touches.length) return
    startY = e.touches[0].clientY
    dragging = true
    dragY = 0
  }
  function touchMove(e) {
    if (!dragging) return
    dragY = Math.max(0, e.touches[0].clientY - startY)
  }
  function touchEnd() {
    dragging = false
    if (dragY > 90) closeStage()
    dragY = 0
  }

  function setTab(name) {
    tab = name
    ui.queueOpen = name === 'queue'
    ui.lyricsMode = name === 'lyrics'
  }

  function seekTo(line) {
    if (line.time === null) return
    seek(line.time / 1000)
  }

  async function rate(n) {
    const t = track
    if (!t) return
    const prev = t.rating || 0
    t.rating = n
    try {
      await api.post('/api/rating', { id: t.id, rating: n })
      notify.ok(n ? `已评分 ${n} 星` : '已清除评分')
    } catch (e) {
      t.rating = prev
      notify.err(e.message)
    }
  }

  function clear() {
    if (!player.queue.length) return
    if (!confirm('清空播放队列？')) return
    clearQueue()
    notify.ok('播放队列已清空')
  }
</script>

<svelte:window onkeydown={onKey} />

{#if ui.stageOpen && track}
  <div
    class="stage"
    transition:fade={{ duration: 260 }}
    style={`--drag:${dragY}px`}
    role="dialog"
    aria-modal="true"
    aria-label="正在播放"
  >
    <div class="bg" aria-hidden="true">
      <Cover id={track.albumId || track.coverArt} alt="" rounded="rounded-none" class="bg-img" eager sizes="320px" />
      <div class="veil"></div>
    </div>

    <div
      class="inner"
      ontransitionend={() => {}}
      style={`transform: translateY(${dragY * 0.4}px)`}
    >
      <header class="head">
        <div class="head-left">
          <span class="eyebrow">Now Playing</span>
          <span class="dot-sep"></span>
          <span class="src">{track.suffix?.toUpperCase()} · {track.bitrate || '—'} kbps</span>
        </div>
        <button class="icon-btn close" onclick={closeStage} aria-label="关闭">
          <X size={18} strokeWidth={1.6} />
        </button>
      </header>

      <div class="body">
        <section
          class="visual"
          role="group"
          aria-label="封面与播放控制"
          ontouchstart={touchStart}
          ontouchmove={touchMove}
          ontouchend={touchEnd}
        >
          <div class="halo" aria-hidden="true"></div>
          <div
            class="vinyl {player.playing ? 'playing' : ''}"
            onclick={() => setTab(tab === 'lyrics' ? 'queue' : 'lyrics')}
            role="button"
            tabindex="0"
            aria-label="切换歌词"
            onkeydown={(e) => e.key === 'Enter' && setTab(tab === 'lyrics' ? 'queue' : 'lyrics')}
          >
            <div class="disc">
              <div class="label">
                <Cover id={track.albumId || track.coverArt} alt={track.album} rounded="rounded-full" eager sizes="(max-width: 640px) 52vw, 390px" />
              </div>
              <div class="hole"></div>
            </div>
            <div class="sheen"></div>
          </div>

          <div class="info">
            <h1 class="title-tight">{track.title}</h1>
            <div class="sub">
              <span>{track.artist}</span>
              {#if track.album}<span class="sep">·</span><span class="faint">{track.album}</span>{/if}
              {#if track.year}<span class="sep">·</span><span class="faint">{track.year}</span>{/if}
            </div>
          </div>

          <div class="progress">
            <Slider value={player.time} max={duration} onseek={seek} label="播放进度" />
            <div class="times">
              <span>{fmtTime(player.time)}</span>
              <span>{fmtTime(duration)}</span>
            </div>
          </div>

          <div class="controls">
            <button
              class="icon-btn"
              class:active={mode === 'shuffle'}
              onclick={() => setPlayMode(mode === 'shuffle' ? 'order' : 'shuffle')}
              aria-label="随机播放"
              aria-pressed={mode === 'shuffle'}
              title="随机播放"
            >
              <Shuffle size={17} strokeWidth={1.6} />
            </button>
            <button class="icon-btn" onclick={prev} aria-label="上一首">
              <SkipBack size={20} strokeWidth={1.6} />
            </button>
            <button class="big-play" onclick={toggle} aria-label="播放/暂停">
              {#if player.loading}
                <Loader2 size={24} class="spin" />
              {:else if player.playing}
                <Pause size={24} fill="currentColor" />
              {:else}
                <Play size={24} fill="currentColor" style="margin-left:3px" />
              {/if}
            </button>
            <button class="icon-btn" onclick={() => next()} aria-label="下一首">
              <SkipForward size={20} strokeWidth={1.6} />
            </button>
            <button
              class="icon-btn"
              class:active={mode === 'one'}
              onclick={() => setPlayMode(mode === 'one' ? 'order' : 'one')}
              aria-label="单曲循环"
              aria-pressed={mode === 'one'}
              title="单曲循环"
            >
              {#if mode === 'one'}
                <Repeat1 size={17} strokeWidth={1.6} />
              {:else}
                <Repeat size={17} strokeWidth={1.6} />
              {/if}
            </button>
          </div>

          <div class="extra">
            <Rating value={track.rating || 0} onrate={rate} size={15} label="歌曲评分" />
            <span class="extra-sep"></span>
            <button class="icon-btn" onclick={toggleMute} aria-label="静音">
              {#if player.muted}<VolumeX size={15} strokeWidth={1.6} />{:else}<Volume2 size={15} strokeWidth={1.6} />{/if}
            </button>
            <Slider class="vol" value={player.muted ? 0 : player.volume * 100} max={100} onseek={(v) => setVolume(v / 100)} label="音量" />
          </div>
        </section>

        <section class="panel glass">
          <div class="tabs">
            <button class="tab" class:active={tab === 'lyrics'} onclick={() => setTab('lyrics')}>
              <Mic2 size={14} strokeWidth={1.7} /> 歌词
            </button>
            <button class="tab" class:active={tab === 'queue'} onclick={() => setTab('queue')}>
              <ListMusic size={14} strokeWidth={1.7} /> 队列
              <span class="count">{player.queue.length}</span>
            </button>
            {#if tab === 'queue' && player.queue.length}
              <button class="clear-queue" onclick={clear} title="清空播放队列">清空</button>
            {/if}
          </div>

          {#if tab === 'lyrics'}
            <div class="lyrics scroll" bind:this={lyricsEl}>
              {#if lyricsLoading}
                <div class="lyrics-empty">正在读取歌词…</div>
              {:else if !lyrics.length}
                <div class="lyrics-empty">
                  <div class="empty-mark">♪</div>
                  <p>这首歌还没有歌词</p>
                  <span>将同名 .lrc 文件放在音频旁，或写入内嵌歌词后重新扫描。</span>
                </div>
              {:else}
                <div class="lyrics-pad"></div>
                {#each lyrics as line, i (i)}
                  <button
                    class="line"
                    class:active={i === active}
                    class:passed={active >= 0 && i < active}
                    class:plain={line.time === null}
                    bind:this={lineRefs[i]}
                    onclick={() => seekTo(line)}
                  >
                    {line.text}
                  </button>
                {/each}
                <div class="lyrics-pad"></div>
              {/if}
            </div>
          {:else}
            <div class="queue scroll">
              {#if !player.queue.length}
                <div class="lyrics-empty"><p>队列是空的</p></div>
              {:else}
                {#each player.queue as item, i (item.id + i)}
                  <div class="q-row" class:current={i === player.index}>
                    <button class="q-main" onclick={() => playTrack(item, player.queue, i)}>
                      <span class="q-idx">
                        {#if i === player.index}
                          <span class="playing-bars" class:paused={!player.playing}><i></i><i></i><i></i></span>
                        {:else}
                          {i + 1}
                        {/if}
                      </span>
                      <Cover id={item.albumId} alt={item.album} rounded="rounded-md" class="q-cover" size={96} />
                      <span class="q-meta">
                        <span class="q-title clamp-1">{item.title}</span>
                        <span class="q-artist clamp-1">{item.artist}</span>
                      </span>
                      <span class="q-dur">{fmtTime(item.duration)}</span>
                    </button>
                    <button class="icon-btn q-del" onclick={() => removeFromQueue(i)} aria-label="移出队列">
                      <Trash2 size={13} strokeWidth={1.6} />
                    </button>
                  </div>
                {/each}
              {/if}
            </div>
          {/if}
        </section>
      </div>
    </div>
  </div>
{/if}

<style>
  .stage {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: flex;
    color: var(--text);
    background: #06060a;
    isolation: isolate;
  }
  :global([data-theme='light']) .stage {
    background: #f1eee8;
  }
  .bg {
    position: absolute;
    inset: 0;
    overflow: hidden;
  }
  :global(.bg-img) {
    position: absolute;
    inset: -12%;
    width: 124%;
    height: 124%;
    filter: blur(100px) saturate(150%);
    opacity: 0.72;
    transform: scale(1.12);
  }
  .veil {
    position: absolute;
    inset: 0;
    background:
      radial-gradient(110% 85% at 24% 6%, rgba(6, 6, 10, 0.2), rgba(6, 6, 10, 0.62) 58%, rgba(6, 6, 10, 0.9)),
      linear-gradient(180deg, rgba(6, 6, 10, 0.42), rgba(6, 6, 10, 0.72) 58%, rgba(6, 6, 10, 0.94));
  }
  :global([data-theme='light']) .veil {
    background:
      radial-gradient(110% 85% at 24% 6%, rgba(241, 238, 232, 0.42), rgba(241, 238, 232, 0.74) 58%, rgba(241, 238, 232, 0.94)),
      linear-gradient(180deg, rgba(241, 238, 232, 0.6), rgba(241, 238, 232, 0.84) 58%, #f1eee8);
  }
  .inner {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: 1560px;
    margin: 0 auto;
    padding: 28px 40px 32px;
    display: flex;
    flex-direction: column;
    transition: transform 0.2s var(--ease-soft);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 18px;
  }
  .head-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .dot-sep {
    width: 3px;
    height: 3px;
    border-radius: 50%;
    background: var(--text-3);
  }
  .src {
    font-size: 10px;
    letter-spacing: 0.12em;
    color: var(--text-3);
    text-transform: uppercase;
  }
  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
    gap: 48px;
    align-items: center;
  }
  .visual {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 0;
    position: relative;
    padding: 10px 0;
  }
  .halo {
    position: absolute;
    width: min(56vh, 480px);
    height: min(56vh, 480px);
    top: 50%;
    left: 50%;
    transform: translate(-50%, -60%);
    background: radial-gradient(circle, rgba(var(--accent-rgb), 0.4), transparent 62%);
    filter: blur(40px);
    animation: breathe 7s ease-in-out infinite;
    pointer-events: none;
  }
  .vinyl {
    width: min(45vh, 390px);
    max-width: 74vw;
    cursor: pointer;
    z-index: 1;
  }
  .info {
    text-align: center;
    margin-top: 30px;
    max-width: 560px;
  }
  .info h1 {
    font-size: clamp(1.9rem, 3.2vw, 2.9rem);
    font-weight: 500;
    margin: 0 0 10px;
    letter-spacing: -0.04em;
  }
  .sub {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    flex-wrap: wrap;
    font-size: 13px;
    color: var(--text-2);
  }
  .sep {
    color: var(--text-3);
  }
  .progress {
    width: min(460px, 86%);
    margin-top: 30px;
  }
  .times {
    display: flex;
    justify-content: space-between;
    font-size: 10.5px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
    margin-top: -2px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-top: 18px;
  }
  .big-play {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    border: none;
    background: var(--text);
    color: var(--bg);
    display: grid;
    place-items: center;
    cursor: pointer;
    transition: transform 0.25s var(--ease);
    box-shadow: 0 20px 50px -20px rgba(0, 0, 0, 0.8);
  }
  .big-play:hover {
    transform: scale(1.05);
  }
  .extra {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 18px;
    opacity: 0.6;
    transition: opacity 0.3s var(--ease);
  }
  .extra:hover {
    opacity: 1;
  }
  .extra-sep {
    width: 1px;
    height: 14px;
    background: var(--line-strong);
    margin: 0 6px;
  }
  :global(.vol) {
    width: 90px;
  }
  .panel {
    height: min(70vh, 660px);
    border-radius: 28px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .tabs {
    display: flex;
    gap: 4px;
    padding: 14px 16px 10px;
    border-bottom: 1px solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 32px;
    padding: 0 12px;
    border-radius: 999px;
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-3);
    font-size: 12.5px;
    cursor: pointer;
    transition: all 0.25s var(--ease);
  }
  .tab:hover {
    color: var(--text-2);
  }
  .tab.active {
    color: var(--text);
    background: var(--glass-2);
    border-color: var(--line);
  }
  .clear-queue {
    margin-left: auto;
    align-self: center;
    padding: 0 8px;
    height: 26px;
    border: 1px solid var(--line);
    border-radius: 999px;
    background: transparent;
    color: var(--text-3);
    font-size: 11px;
    font-family: var(--font-sans);
    cursor: pointer;
    transition: all 0.2s var(--ease);
  }
  .clear-queue:hover {
    color: #fb7185;
    border-color: rgba(251, 113, 133, 0.35);
  }
  .count {
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 999px;
    background: var(--glass-2);
    color: var(--text-2);
  }
  .lyrics,
  .queue {
    flex: 1;
    overflow-y: auto;
    padding: 8px 26px;
    scroll-behavior: smooth;
  }
  .lyrics {
    mask-image: linear-gradient(180deg, transparent, #000 14%, #000 86%, transparent);
  }
  .lyrics-pad {
    height: 30%;
  }
  .line {
    display: block;
    width: 100%;
    text-align: left;
    padding: 9px 0;
    border: none;
    background: none;
    color: var(--text-3);
    font-size: clamp(15px, 1.35vw, 19px);
    line-height: 1.6;
    letter-spacing: -0.01em;
    cursor: pointer;
    transition: all 0.4s var(--ease);
    transform-origin: left center;
    filter: blur(0.4px);
  }
  .line.passed {
    color: var(--text-3);
    filter: blur(0.6px);
  }
  .line.active {
    color: var(--text);
    font-weight: 500;
    transform: scale(1.045);
    filter: none;
  }
  .line.plain {
    color: var(--text-2);
  }
  .lyrics-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 100%;
    min-height: 280px;
    text-align: center;
    color: var(--text-3);
    padding: 0 20px;
  }
  .lyrics-empty p {
    margin: 0;
    color: var(--text-2);
    font-size: 14px;
  }
  .lyrics-empty span {
    font-size: 11.5px;
    max-width: 280px;
    line-height: 1.6;
  }
  .empty-mark {
    font-size: 34px;
    font-family: var(--font-serif);
    color: var(--accent);
    opacity: 0.7;
  }
  .q-row {
    display: flex;
    align-items: center;
    gap: 4px;
    border-radius: 12px;
    border: 1px solid transparent;
    transition: background 0.2s var(--ease);
  }
  .q-row:hover {
    background: var(--glass);
    border-color: var(--line);
  }
  .q-row.current {
    background: rgba(var(--accent-rgb), 0.1);
    border-color: rgba(var(--accent-rgb), 0.2);
  }
  .q-main {
    flex: 1;
    min-width: 0;
    display: grid;
    grid-template-columns: 22px 36px minmax(0, 1fr) auto;
    align-items: center;
    gap: 11px;
    padding: 7px 6px;
    background: none;
    border: none;
    color: var(--text);
    cursor: pointer;
    text-align: left;
  }
  .q-idx {
    font-size: 10.5px;
    color: var(--text-3);
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
  :global(.q-cover) {
    width: 36px;
    height: 36px;
  }
  .q-meta {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .q-title {
    font-size: 12.5px;
  }
  .q-artist {
    font-size: 10.5px;
    color: var(--text-3);
  }
  .q-dur {
    font-size: 10.5px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  .q-del {
    opacity: 0;
    width: 28px;
    height: 28px;
    margin-right: 6px;
  }
  .q-row:hover .q-del {
    opacity: 1;
  }
  @media (max-width: 1023px) {
    .inner {
      padding: 16px 18px 24px;
      overflow-y: auto;
    }
    .body {
      grid-template-columns: 1fr;
      gap: 26px;
      align-items: start;
    }
    .vinyl {
      width: min(58vw, 260px);
    }
    .info {
      margin-top: 24px;
    }
    .info h1 {
      font-size: 1.65rem;
    }
    .panel {
      height: 46vh;
      min-height: 320px;
    }
    .head .eyebrow {
      font-size: 9px;
    }
  }
  @media (max-width: 640px) {
    .inner {
      padding: 10px 14px 18px;
    }
    .vinyl {
      width: min(52vw, 200px);
    }
    .info {
      margin-top: 18px;
    }
    .progress {
      margin-top: 18px;
      width: 100%;
    }
    .controls {
      margin-top: 10px;
      gap: 10px;
    }
    .big-play {
      width: 52px;
      height: 52px;
    }
    .extra {
      display: none;
    }
    .panel {
      height: 42vh;
      min-height: 280px;
      border-radius: 22px;
    }
    .lyrics,
    .queue {
      padding: 6px 18px;
    }
    .lyrics-pad {
      height: 24%;
    }
  }
  /* smooth mode: no 100px backdrop blur, no breathing halo, no per-line filter */
  :global([data-perf='smooth']) .bg,
  :global([data-perf='smooth']) .halo {
    display: none;
  }
  :global([data-perf='smooth']) .stage {
    background:
      radial-gradient(96% 68% at 18% 0%, rgba(var(--accent-rgb), 0.26), transparent 62%),
      radial-gradient(76% 58% at 86% 92%, rgba(var(--accent-2-rgb), 0.15), transparent 66%),
      #06060a;
  }
  :global([data-theme='light'][data-perf='smooth']) .stage {
    background:
      radial-gradient(96% 68% at 18% 0%, rgba(var(--accent-rgb), 0.18), transparent 62%),
      radial-gradient(76% 58% at 86% 92%, rgba(var(--accent-2-rgb), 0.12), transparent 66%),
      #f1eee8;
  }
  :global([data-perf='smooth']) .line {
    filter: none !important;
  }
</style>
