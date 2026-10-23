<script>
  import { onMount } from 'svelte'
  import { ListMusic, Plus, Play, Trash2 } from 'lucide-svelte'
  import Cover from '../components/Cover.svelte'
  import Link from '../components/Link.svelte'
  import EmptyState from '../components/EmptyState.svelte'
  import { api } from '../lib/api.js'
  import { navigate } from '../lib/router.svelte.js'
  import { notify } from '../lib/toast.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import { fmtDuration } from '../lib/format.js'
  import { playQueue } from '../lib/player.svelte.js'

  let playlists = $state([])
  let loading = $state(true)

  async function load() {
    loading = true
    try {
      const data = await api.get('/api/playlists')
      playlists = data.playlists || []
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    const onChanged = () => load()
    window.addEventListener('mrome:playlist-changed', onChanged)
    window.addEventListener('mrome:refresh', onChanged)
    return () => {
      window.removeEventListener('mrome:playlist-changed', onChanged)
      window.removeEventListener('mrome:refresh', onChanged)
    }
  })

  async function create() {
    const name = prompt('新歌单名称', '未命名歌单')
    if (!name) return
    try {
      const data = await api.post('/api/playlists', { name, public: true })
      navigate(`/playlists/${data.playlist.id}`)
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function remove(p, e) {
    e.preventDefault()
    e.stopPropagation()
    if (!confirm(`删除歌单「${p.name}」？`)) return
    try {
      await api.del(`/api/playlists/${p.id}`)
      notify.ok('歌单已删除')
      load()
    } catch (err) {
      notify.err(err.message)
    }
  }

  async function play(p, e) {
    e.preventDefault()
    e.stopPropagation()
    try {
      const data = await api.get(`/api/playlists/${p.id}`)
      if (data.playlist?.entries?.length) playQueue(data.playlist.entries, 0)
      else notify.info('这个歌单还是空的')
    } catch (err) {
      notify.err(err.message)
    }
  }
</script>

<div class="page rise">
  <header class="page-head">
    <div>
      <div class="eyebrow">Playlists</div>
      <h1 class="page-title">歌单</h1>
      <div class="page-sub">{playlists.length} 个歌单 · 与 Subsonic 客户端双向同步</div>
    </div>
    <button class="btn btn-primary" onclick={create}>
      <Plus size={15} /> 新建歌单
    </button>
  </header>

  {#if loading && !playlists.length}
    <div class="loading">正在载入…</div>
  {:else if !playlists.length}
    <EmptyState icon={ListMusic} title="还没有歌单" desc="创建歌单，把喜欢的曲目串成一段旅程；在 Subsonic 客户端里创建的歌单也会出现在这里。">
      <button class="btn btn-primary" onclick={create}><Plus size={14} /> 新建歌单</button>
    </EmptyState>
  {:else}
    <div class="grid">
      {#each playlists as p (p.id)}
        <Link href={`/playlists/${p.id}`} class="pl">
          <div class="stack">
            <div class="layer l2"></div>
            <div class="layer l1"></div>
            <Cover id={p.coverArt} alt={p.name} rounded="rounded-[14px]" class="cover" sizes="(max-width: 860px) 44vw, 200px" />
            <button class="play" onclick={(e) => play(p, e)} aria-label="播放歌单">
              <Play size={15} fill="currentColor" style="margin-left:2px" />
            </button>
            {#if p.ownerId === auth.user?.id || auth.user?.isAdmin}
              <button class="del" onclick={(e) => remove(p, e)} aria-label="删除歌单">
                <Trash2 size={13} />
              </button>
            {/if}
          </div>
          <div class="name clamp-1">{p.name}</div>
          <div class="sub">{p.songCount} 首 · {fmtDuration(p.duration)}</div>
        </Link>
      {/each}
      <button class="pl-new" onclick={create}>
        <span class="plus-box"><Plus size={20} strokeWidth={1.6} /></span>
        <span class="name">新建歌单</span>
        <span class="sub">从空白开始</span>
      </button>
    </div>
  {/if}
</div>

<style>
  .page {
    padding-bottom: 20px;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 30px 20px;
  }
  :global(.pl) {
    text-decoration: none;
    color: var(--text);
    min-width: 0;
    display: block;
  }
  .stack {
    position: relative;
    aspect-ratio: 1;
    margin-bottom: 12px;
    transition: transform 0.45s var(--ease);
  }
  :global(.pl):hover .stack {
    transform: translateY(-4px);
  }
  .layer {
    position: absolute;
    inset: 0;
    border-radius: 14px;
    background: var(--glass-2);
    border: 1px solid var(--line);
    transition: transform 0.45s var(--ease);
  }
  .l1 {
    transform: translate(5px, 5px) rotate(1.6deg);
  }
  .l2 {
    transform: translate(10px, 10px) rotate(3.2deg);
    opacity: 0.55;
  }
  :global(.pl):hover .l1 {
    transform: translate(7px, 7px) rotate(2.4deg);
  }
  :global(.pl):hover .l2 {
    transform: translate(14px, 14px) rotate(4.6deg);
  }
  .stack :global(.cover) {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    box-shadow: 0 20px 46px -24px rgba(0, 0, 0, 0.85);
  }
  .play {
    position: absolute;
    right: 11px;
    bottom: 11px;
    width: 38px;
    height: 38px;
    border-radius: 50%;
    border: none;
    display: grid;
    place-items: center;
    background: rgba(255, 255, 255, 0.92);
    color: #111;
    cursor: pointer;
    opacity: 0;
    transform: translateY(6px);
    transition: all 0.3s var(--ease);
  }
  :global(.pl):hover .play {
    opacity: 1;
    transform: translateY(0);
  }
  .del {
    position: absolute;
    left: 10px;
    top: 10px;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.2);
    background: rgba(0, 0, 0, 0.4);
    color: rgba(255, 255, 255, 0.85);
    display: grid;
    place-items: center;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.25s var(--ease);
  }
  :global(.pl):hover .del {
    opacity: 1;
  }
  .name {
    font-size: 13px;
    font-weight: 500;
  }
  .sub {
    font-size: 11px;
    color: var(--text-3);
    margin-top: 3px;
  }
  .pl-new {
    display: block;
    padding: 0;
    border: none;
    background: none;
    text-align: left;
    color: var(--text-3);
    cursor: pointer;
    min-width: 0;
  }
  .plus-box {
    display: grid;
    place-items: center;
    aspect-ratio: 1;
    margin-bottom: 12px;
    border-radius: 14px;
    border: 1px dashed var(--line-strong);
    background: var(--glass);
    color: var(--text-3);
    transition: all 0.35s var(--ease);
  }
  .pl-new:hover .plus-box {
    border-color: rgba(var(--accent-rgb), 0.55);
    background: rgba(var(--accent-rgb), 0.08);
    color: var(--accent);
    transform: translateY(-4px);
  }
  .pl-new:hover .name {
    color: var(--text);
  }
  .pl-new .name,
  .pl-new .sub {
    display: block;
  }
  .loading {
    padding: 60px 0;
    text-align: center;
    color: var(--text-3);
  }
  @media (max-width: 860px) {
    .grid {
      grid-template-columns: repeat(auto-fill, minmax(136px, 1fr));
      gap: 22px 12px;
    }
  }
</style>
