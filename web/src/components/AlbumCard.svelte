<script>
  import { Play } from 'lucide-svelte'
  import Cover from './Cover.svelte'
  import Link from './Link.svelte'
  import { api } from '../lib/api.js'
  import { playQueue } from '../lib/player.svelte.js'
  import { notify } from '../lib/toast.svelte.js'

  let { album, size = 'md' } = $props()

  async function play(e) {
    e.preventDefault()
    e.stopPropagation()
    try {
      const data = await api.get(`/api/albums/${album.id}`)
      if (data.album?.songs?.length) playQueue(data.album.songs, 0)
    } catch (err) {
      notify.err(err.message)
    }
  }
</script>

<Link href={`/albums/${album.id}`} class={`card ${size === 'sm' ? 'compact' : ''}`}>
  <div class="art">
    <Cover id={album.id} alt={album.name} rounded="rounded-[14px]" class="art-img" sizes="(max-width: 860px) 44vw, 200px" />
    <div class="shade"></div>
    <button class="play" onclick={play} aria-label="播放专辑">
      <Play size={16} fill="currentColor" style="margin-left:2px" />
    </button>
  </div>
  <div class="meta">
    <div class="name clamp-1">{album.name}</div>
    <div class="artist clamp-1">{album.artist}</div>
  </div>
</Link>

<style>
  :global(.card) {
    display: block;
    text-decoration: none;
    color: var(--text);
    min-width: 0;
  }
  .art {
    position: relative;
    border-radius: 14px;
    overflow: hidden;
    aspect-ratio: 1;
    transition: transform 0.5s var(--ease), box-shadow 0.5s var(--ease);
    box-shadow: 0 18px 40px -26px rgba(0, 0, 0, 0.8);
  }
  :global(.card):hover .art {
    transform: translateY(-4px);
    box-shadow: 0 30px 60px -30px rgba(0, 0, 0, 0.9);
  }
  :global(.art-img) {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    transition: transform 0.7s var(--ease);
  }
  :global(.card):hover :global(.art-img) {
    transform: scale(1.045);
  }
  .shade {
    position: absolute;
    inset: 0;
    background: linear-gradient(180deg, transparent 55%, rgba(0, 0, 0, 0.5));
    opacity: 0;
    transition: opacity 0.4s var(--ease);
  }
  :global(.card):hover .shade {
    opacity: 1;
  }
  .play {
    position: absolute;
    right: 12px;
    bottom: 12px;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: none;
    display: grid;
    place-items: center;
    background: rgba(255, 255, 255, 0.92);
    color: #111;
    cursor: pointer;
    opacity: 0;
    transform: translateY(8px) scale(0.9);
    transition: all 0.35s var(--ease);
    box-shadow: 0 10px 24px -10px rgba(0, 0, 0, 0.7);
  }
  :global(.card):hover .play {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  .play:hover {
    transform: scale(1.06) !important;
  }
  .meta {
    margin-top: 11px;
    min-width: 0;
  }
  .name {
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }
  .artist {
    font-size: 11.5px;
    color: var(--text-2);
    margin-top: 3px;
  }
  :global(.compact) .name {
    font-size: 12px;
  }
</style>
