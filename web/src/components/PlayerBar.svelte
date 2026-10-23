<script>
  import {
    Play,
    Pause,
    SkipBack,
    SkipForward,
    Shuffle,
    Repeat,
    Repeat1,
    ListStart,
    Volume2,
    Volume1,
    VolumeX,
    ListMusic,
    Mic2,
    Maximize2,
    Star,
    Loader2
  } from 'lucide-svelte'
  import { fly } from 'svelte/transition'
  import { onDestroy } from 'svelte'
  import Cover from './Cover.svelte'
  import Slider from './Slider.svelte'
  import {
    player,
    currentTrack,
    toggle,
    next,
    prev,
    seek,
    setVolume,
    toggleMute,
    setPlayMode,
    playMode,
    PLAY_MODES
  } from '../lib/player.svelte.js'
  import { openStage, openStageQueue, ui } from '../lib/ui.svelte.js'
  import { fmtTime } from '../lib/format.js'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/toast.svelte.js'

  const track = $derived(currentTrack())
  const duration = $derived(player.duration || track?.duration || 0)
  const pct = $derived(duration > 0 ? Math.min(100, (player.time / duration) * 100) : 0)
  const mode = $derived(playMode())

  // Transient hint above the bar that names the mode just selected. It lives
  // next to the player instead of the global toast stack so it can point at the
  // controls it belongs to and never collides with playback error messages.
  let tipText = $state('')
  let tipSeq = $state(0)
  let tipTimer = 0

  function pickMode(id) {
    setPlayMode(id)
    tipText = PLAY_MODES.find((m) => m.id === id)?.label || ''
    tipSeq += 1
    clearTimeout(tipTimer)
    tipTimer = setTimeout(() => (tipText = ''), 2000)
  }

  // Mobile keeps a single mode button that cycles order → one → shuffle.
  function cycleMode() {
    pickMode(mode === 'order' ? 'one' : mode === 'one' ? 'shuffle' : 'order')
  }

  onDestroy(() => clearTimeout(tipTimer))

  async function toggleStar() {
    if (!track) return
    const nextState = !track.starred
    track.starred = nextState
    try {
      await api.post('/api/star', { id: track.id, type: 'song', starred: nextState })
      notify.ok(nextState ? '已收藏' : '已取消收藏')
      window.dispatchEvent(new CustomEvent('mrome:stars'))
    } catch (e) {
      track.starred = !nextState
      notify.err(e.message)
    }
  }
</script>

{#if track}
  <div class="player glass-deep">
    <div class="m-progress" aria-hidden="true"><i style={`transform:scaleX(${pct / 100})`}></i></div>

    <div class="tip-layer" aria-live="polite">
      {#if tipText}
        {#key tipSeq}
          <div class="mode-tip" transition:fly={{ y: 8, duration: 200 }}>
            <span class="tip-ico">
              {#if mode === 'one'}
                <Repeat1 size={13} strokeWidth={1.8} />
              {:else if mode === 'shuffle'}
                <Shuffle size={13} strokeWidth={1.8} />
              {:else}
                <ListStart size={13} strokeWidth={1.8} />
              {/if}
            </span>
            <span>{tipText}</span>
          </div>
        {/key}
      {/if}
    </div>

    <div class="left">
      <button class="cover-btn" onclick={() => openStage(false)} aria-label="打开播放舞台">
        <Cover id={track.albumId || track.coverArt} alt={track.album} rounded="rounded-[10px]" class="thumb" eager size={160} />
        <span class="expand-hint"></span>
      </button>
      <div class="meta">
        <div class="t clamp-1">{track.title}</div>
        <div class="a clamp-1">{track.artist}</div>
      </div>
      <button
        class="icon-btn star"
        class:active={track.starred}
        onclick={toggleStar}
        aria-label="收藏"
        title="收藏"
      >
        <Star size={15} strokeWidth={1.6} fill={track.starred ? 'currentColor' : 'none'} />
      </button>

      <div class="m-btns">
        <button
          class="icon-btn"
          class:active={mode !== 'order'}
          onclick={cycleMode}
          aria-label="切换播放模式"
          title={PLAY_MODES.find((m) => m.id === mode)?.label || '播放模式'}
        >
          {#if mode === 'one'}
            <Repeat1 size={15} strokeWidth={1.6} />
          {:else if mode === 'shuffle'}
            <Shuffle size={15} strokeWidth={1.6} />
          {:else}
            <ListStart size={15} strokeWidth={1.6} />
          {/if}
        </button>
        <button class="icon-btn" onclick={prev} aria-label="上一首">
          <SkipBack size={16} strokeWidth={1.7} />
        </button>
        <button class="play" onclick={toggle} aria-label={player.playing ? '暂停' : '播放'}>
          {#if player.loading}
            <Loader2 size={17} class="spin" />
          {:else if player.playing}
            <Pause size={17} fill="currentColor" />
          {:else}
            <Play size={17} fill="currentColor" style="margin-left:2px" />
          {/if}
        </button>
        <button class="icon-btn" onclick={() => next()} aria-label="下一首">
          <SkipForward size={16} strokeWidth={1.7} />
        </button>
      </div>
    </div>

    <div class="m-timeline">
      <span>{fmtTime(player.time)}</span>
      <Slider value={player.time} max={duration} onseek={seek} label="播放进度" />
      <span>{fmtTime(duration)}</span>
    </div>

    <div class="center">
      <div class="controls">
        <button
          class="icon-btn"
          class:active={mode === 'shuffle'}
          onclick={() => pickMode(mode === 'shuffle' ? 'order' : 'shuffle')}
          title="随机播放"
          aria-label="随机播放"
          aria-pressed={mode === 'shuffle'}
        >
          <Shuffle size={15} strokeWidth={1.6} />
        </button>
        <button class="icon-btn" onclick={prev} title="上一首" aria-label="上一首">
          <SkipBack size={17} strokeWidth={1.7} />
        </button>
        <button class="play" onclick={toggle} aria-label={player.playing ? '暂停' : '播放'}>
          {#if player.loading}
            <Loader2 size={19} class="spin" />
          {:else if player.playing}
            <Pause size={19} fill="currentColor" />
          {:else}
            <Play size={19} fill="currentColor" style="margin-left:2px" />
          {/if}
        </button>
        <button class="icon-btn" onclick={() => next()} title="下一首" aria-label="下一首">
          <SkipForward size={17} strokeWidth={1.7} />
        </button>
        <button
          class="icon-btn"
          class:active={mode === 'one'}
          onclick={() => pickMode(mode === 'one' ? 'order' : 'one')}
          title="单曲循环"
          aria-label="单曲循环"
          aria-pressed={mode === 'one'}
        >
          {#if mode === 'one'}
            <Repeat1 size={15} strokeWidth={1.6} />
          {:else}
            <Repeat size={15} strokeWidth={1.6} />
          {/if}
        </button>
      </div>
      <div class="timeline">
        <span>{fmtTime(player.time)}</span>
        <Slider value={player.time} max={duration} onseek={seek} label="播放进度" />
        <span>{fmtTime(duration)}</span>
      </div>
    </div>

    <div class="right">
      <button class="icon-btn" onclick={() => openStage(true)} title="歌词" aria-label="歌词">
        <Mic2 size={16} strokeWidth={1.6} />
      </button>
      <button
        class="icon-btn queue-btn"
        class:active={ui.stageOpen && ui.queueOpen}
        onclick={openStageQueue}
        title="播放队列"
        aria-label="播放队列"
      >
        <ListMusic size={16} strokeWidth={1.6} />
      </button>
      <div class="vol">
        <button class="icon-btn" onclick={toggleMute} title="静音" aria-label="静音">
          {#if player.muted || player.volume === 0}
            <VolumeX size={16} strokeWidth={1.6} />
          {:else if player.volume < 0.5}
            <Volume1 size={16} strokeWidth={1.6} />
          {:else}
            <Volume2 size={16} strokeWidth={1.6} />
          {/if}
        </button>
        <Slider
          class="vol-slider"
          value={player.muted ? 0 : player.volume * 100}
          max={100}
          onseek={(v) => setVolume(v / 100)}
          label="音量"
        />
      </div>
      <button class="icon-btn expand" onclick={() => openStage(false)} title="展开" aria-label="展开播放器">
        <Maximize2 size={15} strokeWidth={1.6} />
      </button>
    </div>
  </div>
{/if}

<style>
  .player {
    position: fixed;
    left: 16px;
    right: 16px;
    bottom: 16px;
    z-index: 45;
    height: 88px;
    border-radius: 24px;
    background: var(--bar-bg);
    backdrop-filter: blur(36px) saturate(170%);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 1.25fr) minmax(0, 1fr);
    align-items: center;
    gap: 20px;
    padding: 0 18px;
  }
  .left {
    display: flex;
    align-items: center;
    gap: 13px;
    min-width: 0;
  }
  .cover-btn {
    position: relative;
    flex: none;
    border: none;
    background: none;
    padding: 0;
    cursor: pointer;
    border-radius: 12px;
  }
  .cover-btn :global(.thumb) {
    width: 52px;
    height: 52px;
    box-shadow: 0 10px 24px -12px rgba(0, 0, 0, 0.9);
    transition: transform 0.35s var(--ease);
  }
  .cover-btn:hover :global(.thumb) {
    transform: scale(1.04);
  }
  .meta {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .t {
    font-size: 13.5px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }
  .a {
    font-size: 11.5px;
    color: var(--text-2);
  }
  .star {
    margin-left: 2px;
    width: 32px;
    height: 32px;
  }
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 7px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  /* Floating hint above the bar, naming the mode that was just selected. */
  .tip-layer {
    position: absolute;
    left: 0;
    right: 0;
    top: -46px;
    display: flex;
    justify-content: center;
    pointer-events: none;
    z-index: 2;
  }
  .mode-tip {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 30px;
    padding: 0 13px;
    border-radius: 999px;
    background: var(--glass-3);
    border: 1px solid var(--line);
    box-shadow: var(--shadow-soft);
    backdrop-filter: blur(24px) saturate(150%);
    color: var(--text);
    font-size: 12.5px;
    white-space: nowrap;
  }
  .tip-ico {
    display: grid;
    place-items: center;
    flex: none;
    color: var(--accent);
  }
  .play {
    width: 38px;
    height: 38px;
    margin: 0 6px;
    border-radius: 50%;
    border: none;
    background: var(--text);
    color: var(--bg);
    display: grid;
    place-items: center;
    cursor: pointer;
    transition: all 0.25s var(--ease);
    box-shadow: 0 8px 24px -10px rgba(0, 0, 0, 0.7);
  }
  .play:hover {
    transform: scale(1.06);
  }
  .play:active {
    transform: scale(0.96);
  }
  .timeline {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    max-width: 520px;
    font-size: 10.5px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  .timeline span {
    width: 38px;
    text-align: center;
    flex: none;
  }
  .right {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
  }
  .vol {
    display: flex;
    align-items: center;
    gap: 2px;
    margin: 0 2px;
  }
  :global(.vol-slider) {
    width: 78px;
  }
  .m-btns,
  .m-timeline {
    display: none;
  }
  .m-progress {
    display: none;
  }
  @media (max-width: 1180px) {
    .player {
      grid-template-columns: minmax(0, 1fr) minmax(260px, 1.2fr) auto;
    }
    :global(.vol-slider) {
      display: none;
    }
    .expand {
      display: none;
    }
  }
  @media (max-width: 860px) {
    .player {
      left: 8px;
      right: 8px;
      bottom: 8px;
      height: 84px;
      padding: 8px 12px;
      border-radius: 20px;
      display: flex;
      flex-direction: column;
      justify-content: center;
      gap: 5px;
    }
    .left {
      width: 100%;
      gap: 10px;
    }
    .cover-btn :global(.thumb) {
      width: 40px;
      height: 40px;
      border-radius: 9px;
    }
    .center,
    .right,
    .star,
    .m-progress {
      display: none;
    }
    /* Mobile gets the full control set back: mode, previous, play, next. */
    .m-btns {
      display: flex;
      align-items: center;
      gap: 0;
      flex: none;
    }
    .m-btns .icon-btn {
      width: 32px;
      height: 32px;
    }
    .m-btns .play {
      width: 32px;
      height: 32px;
      margin: 0 3px;
    }
    .m-timeline {
      display: flex;
      align-items: center;
      gap: 8px;
      width: 100%;
      font-size: 10px;
      color: var(--text-3);
      font-variant-numeric: tabular-nums;
    }
    .m-timeline span {
      width: 34px;
      text-align: center;
      flex: none;
    }
  }
</style>
