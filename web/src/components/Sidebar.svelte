<script>
  import { Home, Music2, Heart, Disc3, Users, ListMusic, UploadCloud, LogOut, LogIn, Radio, History } from 'lucide-svelte'
  import { route } from '../lib/router.svelte.js'
  import { auth, loadMe, logout } from '../lib/auth.svelte.js'
  import { openSettings } from '../lib/ui.svelte.js'
  import { navigate } from '../lib/router.svelte.js'
  import { stop } from '../lib/player.svelte.js'
  import { fmtDuration } from '../lib/format.js'
  import Link from './Link.svelte'

  let { stats = null } = $props()

  const items = $derived([
    { href: '/', label: '首页', icon: Home, name: 'home' },
    { href: '/songs', label: '全部歌曲', icon: Music2, name: 'songs' },
    { href: '/favorites', label: '收藏', icon: Heart, name: 'favorites' },
    { href: '/recent', label: '最近播放', icon: History, name: 'recent' },
    { href: '/albums', label: '专辑', icon: Disc3, name: 'albums' },
    { href: '/artists', label: '艺术家', icon: Users, name: 'artists' },
    { href: '/playlists', label: '歌单', icon: ListMusic, name: 'playlists' },
    ...(auth.user?.isAdmin ? [{ href: '/upload', label: '上传音乐', icon: UploadCloud, name: 'upload' }] : [])
  ])

  async function signOut() {
    stop()
    await logout()
    if (auth.guestMode) {
      await loadMe()
      navigate('/')
      return
    }
    navigate('/login')
  }
</script>

<aside class="sidebar">
  <div
    class="brand"
    role="button"
    tabindex="0"
    title="双击也可快速打开设置"
    ondblclick={openSettings}
    onkeydown={(e) => e.key === 'Enter' && openSettings()}
  >
    <div class="mark" aria-hidden="true">
      <span class="ring"></span>
      <span class="core"></span>
    </div>
    <div class="brand-text">
      <div class="wordmark">MROME</div>
      <div class="tagline">私人音乐库</div>
    </div>
  </div>

  <nav class="nav" aria-label="主导航">
    {#each items as item}
      <Link href={item.href} class={`nav-item ${route.name === item.name ? 'active' : ''}`}>
        <item.icon size={17} strokeWidth={1.6} />
        <span>{item.label}</span>
        <i class="dot"></i>
      </Link>
    {/each}
  </nav>

  <div class="grow"></div>

  <div class="stat-card glass">
    <div class="stat-head">
      <span class="eyebrow">Library</span>
      <Radio size={13} strokeWidth={1.6} class="live" />
    </div>
    <div class="stat-main">
      <span class="num">{stats?.songs ?? '—'}</span>
      <span class="unit">首曲目</span>
    </div>
    <div class="stat-sub">{stats?.albums ?? 0} 张专辑 · {fmtDuration(stats?.duration)}</div>
    <div class="stat-foot">
      <span class="badge"><i></i> Subsonic 1.16.1</span>
    </div>
  </div>

  <div class="user">
    <div class="avatar">{auth.user?.username?.slice(0, 1).toUpperCase() || '?'}</div>
    <div class="user-meta">
      <div class="user-name">{auth.user?.username}</div>
      <div class="user-role">
        {auth.user?.guest ? '免登录访客' : auth.user?.isAdmin ? '管理员' : '听众'}
      </div>
    </div>
    {#if auth.user?.guest}
      {#if auth.canLogin}
        <button class="icon-btn" onclick={() => navigate('/login')} title="管理员登录" aria-label="管理员登录">
          <LogIn size={15} strokeWidth={1.6} />
        </button>
      {/if}
    {:else}
      <button class="icon-btn" onclick={signOut} title="退出登录" aria-label="退出登录">
        <LogOut size={15} strokeWidth={1.6} />
      </button>
    {/if}
  </div>
</aside>

<style>
  .sidebar {
    position: sticky;
    top: 0;
    height: 100dvh;
    width: 264px;
    flex: none;
    display: flex;
    flex-direction: column;
    padding: 28px 18px 118px;
    gap: 26px;
    border-right: 1px solid var(--line);
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent 40%);
    z-index: 20;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 2px 8px;
    cursor: default;
    user-select: none;
    border-radius: 12px;
    transition: background 0.3s var(--ease);
  }
  .brand:hover {
    background: var(--glass);
  }
  .mark {
    position: relative;
    width: 34px;
    height: 34px;
    border-radius: 50%;
    background: conic-gradient(from 200deg, var(--accent), var(--accent-2), var(--accent));
    display: grid;
    place-items: center;
    box-shadow: 0 8px 24px -8px rgba(var(--accent-rgb), 0.7);
  }
  .ring {
    position: absolute;
    inset: 6px;
    border-radius: 50%;
    border: 1px solid rgba(0, 0, 0, 0.35);
  }
  .core {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #0a0a0f;
    box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.25);
  }
  .wordmark {
    font-size: 14px;
    font-weight: 600;
    letter-spacing: 0.28em;
    line-height: 1.1;
  }
  .tagline {
    font-size: 10px;
    letter-spacing: 0.16em;
    color: var(--text-3);
    margin-top: 3px;
  }
  .nav {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  :global(.nav-item) {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 40px;
    padding: 0 12px;
    border-radius: 11px;
    color: var(--text-2);
    font-size: 13.5px;
    font-weight: 450;
    text-decoration: none;
    border: 1px solid transparent;
    transition: all 0.25s var(--ease);
    position: relative;
  }
  :global(.nav-item):hover {
    color: var(--text);
    background: var(--glass);
  }
  :global(.nav-item.active) {
    color: var(--text);
    background: var(--glass-2);
    border-color: var(--line);
    box-shadow: inset 0 1px 0 var(--inset);
  }
  .dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--accent);
    margin-left: auto;
    opacity: 0;
    transform: scale(0.4);
    transition: all 0.3s var(--ease);
    box-shadow: 0 0 10px var(--accent);
  }
  :global(.nav-item.active) .dot {
    opacity: 1;
    transform: scale(1);
  }
  .grow {
    flex: 1;
  }
  .stat-card {
    border-radius: 18px;
    padding: 14px 15px 12px;
  }
  .stat-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
  }
  :global(.live) {
    color: #4ade80;
  }
  .stat-main {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }
  .num {
    font-size: 26px;
    font-weight: 300;
    letter-spacing: -0.03em;
    font-variant-numeric: tabular-nums;
  }
  .unit {
    font-size: 11px;
    color: var(--text-3);
  }
  .stat-sub {
    font-size: 11px;
    color: var(--text-2);
    margin-top: 2px;
  }
  .stat-foot {
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--line);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 9.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--text-2);
  }
  .badge i {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #4ade80;
    box-shadow: 0 0 8px #4ade80;
  }
  .user {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 4px 0;
  }
  .avatar {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 12px;
    font-weight: 600;
    color: var(--text);
    background: linear-gradient(140deg, rgba(var(--accent-rgb), 0.5), var(--glass-2));
    border: 1px solid var(--line-strong);
  }
  .user-meta {
    min-width: 0;
    flex: 1;
  }
  .user-name {
    font-size: 12.5px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .user-role {
    font-size: 10px;
    color: var(--text-3);
    letter-spacing: 0.08em;
  }
  @media (max-width: 1023px) {
    .sidebar {
      display: none;
    }
  }
</style>
