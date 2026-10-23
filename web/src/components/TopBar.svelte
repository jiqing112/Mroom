<script>
  import { onMount } from 'svelte'
  import { Search, Sun, Moon, Gauge, Settings } from 'lucide-svelte'
  import { perf, setPerfMode } from '../lib/perf.svelte.js'
  import { route, navigate } from '../lib/router.svelte.js'
  import { openSearch, openSettings } from '../lib/ui.svelte.js'
  import { theme, toggleTheme } from '../lib/theme.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import { Home, Music2, Star, Disc3, Users, ListMusic, UploadCloud } from 'lucide-svelte'

  let headerEl = $state(null)
  let scrolled = $state(false)

  onMount(() => {
    const scroller = headerEl.closest('.main')
    if (!scroller) return
    const sync = () => {
      scrolled = scroller.scrollTop > 8
    }
    sync()
    scroller.addEventListener('scroll', sync, { passive: true })
    return () => scroller.removeEventListener('scroll', sync)
  })

  const labels = {
    home: 'Overview',
    songs: 'All tracks',
    favorites: 'Favorites',
    albums: 'Albums',
    artists: 'Artists',
    playlists: 'Playlists',
    playlist: 'Playlist',
    album: 'Album',
    artist: 'Artist',
    upload: 'Upload',
    settings: 'Settings'
  }

  const mobileItems = $derived([
    { href: '/', icon: Home, name: 'home', label: '首页' },
    { href: '/songs', icon: Music2, name: 'songs', label: '歌曲' },
    { href: '/favorites', icon: Star, name: 'favorites', label: '收藏' },
    { href: '/albums', icon: Disc3, name: 'albums', label: '专辑' },
    { href: '/artists', icon: Users, name: 'artists', label: '艺术家' },
    { href: '/playlists', icon: ListMusic, name: 'playlists', label: '歌单' },
    ...(auth.user?.isAdmin ? [{ href: '/upload', icon: UploadCloud, name: 'upload', label: '上传' }] : [])
  ])
</script>

<header class="topbar" class:scrolled bind:this={headerEl}>
  <div class="topbar-inner">
  <div class="left">
    <span class="mark-mobile">M</span>
    <span class="eyebrow">{labels[route.name] || 'Mrome'}</span>
  </div>
  <div class="right">
    <button class="search-trigger" onclick={openSearch} aria-label="搜索">
      <Search size={15} strokeWidth={1.7} />
      <span class="hint">搜索歌曲、专辑、艺术家</span>
      <kbd>⌘K</kbd>
    </button>
    <button
      class="icon-btn settings-btn"
      onclick={openSettings}
      aria-label="打开设置"
      title="设置 · 外观与性能、音乐库、账号"
    >
      <Settings size={16} strokeWidth={1.6} />
    </button>
    <button
      class="icon-btn perf-toggle"
      class:active={perf.smooth}
      onclick={() => setPerfMode(perf.smooth ? 'full' : 'smooth')}
      aria-label={perf.smooth ? '切换到完整效果' : '切换到流畅模式'}
      title={perf.static
        ? '静止模式已开启 · 点击恢复完整效果'
        : perf.smooth
          ? '流畅模式已开启 · 点击恢复完整效果'
          : '流畅模式 · 关闭动效与模糊'}
    >
      <Gauge size={16} strokeWidth={1.6} />
    </button>
    <button class="icon-btn" onclick={toggleTheme} aria-label="切换主题" title="切换亮/暗模式">
      {#if theme.mode === 'dark'}
        <Sun size={16} strokeWidth={1.6} />
      {:else}
        <Moon size={16} strokeWidth={1.6} />
      {/if}
    </button>
  </div>
  </div>
</header>

<nav class="mobile-nav" aria-label="移动端导航">
  {#each mobileItems as item}
    <button
      class="chip"
      class:active={route.name === item.name}
      onclick={() => navigate(item.href)}
    >
      <item.icon size={14} strokeWidth={1.7} />
      <span>{item.label}</span>
    </button>
  {/each}
</nav>

<style>
  .topbar {
    position: sticky;
    top: 0;
    z-index: 30;
    padding: 22px 0 16px;
    background: transparent;
    border-bottom: 1px solid transparent;
    box-shadow: 0 14px 36px -30px transparent;
    transition:
      background 0.35s var(--ease),
      border-color 0.35s var(--ease),
      box-shadow 0.35s var(--ease);
  }
  /* At rest the header sits on the same aurora wash as the page title below, so
     the 86% fill would cut a visible band across it. The glass surface only
     appears once content is actually scrolling underneath. */
  .topbar.scrolled {
    background: color-mix(in srgb, var(--bg) 86%, transparent);
    border-bottom-color: var(--line-strong);
    box-shadow: 0 14px 36px -30px rgba(0, 0, 0, 0.45);
    backdrop-filter: blur(14px) saturate(120%);
  }
  .topbar-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    max-width: 1480px;
    margin: 0 auto;
    padding: 0 var(--page-gutter);
  }
  .left {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .mark-mobile {
    display: none;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: conic-gradient(from 200deg, var(--accent), var(--accent-2), var(--accent));
    color: #0a0a0f;
    font-size: 11px;
    font-weight: 700;
    place-items: center;
  }
  .right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .search-trigger {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 38px;
    padding: 0 12px 0 14px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: var(--glass);
    color: var(--text-3);
    font-size: 12.5px;
    cursor: pointer;
    transition: all 0.3s var(--ease);
  }
  .search-trigger:hover {
    border-color: var(--line-strong);
    background: var(--glass-2);
    color: var(--text-2);
  }
  .hint {
    min-width: 170px;
    text-align: left;
  }
  kbd {
    font-family: var(--font-sans);
    font-size: 10px;
    padding: 2px 6px;
    border-radius: 5px;
    border: 1px solid var(--line);
    color: var(--text-3);
    background: var(--glass);
  }
  .mobile-nav {
    display: none;
    gap: 7px;
    overflow-x: auto;
    padding: 2px var(--page-gutter) 10px;
    margin-bottom: 2px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 31px;
    padding: 0 12px;
    border-radius: 999px;
    border: 1px solid var(--line);
    background: transparent;
    color: var(--text-2);
    font-size: 12px;
    white-space: nowrap;
    cursor: pointer;
  }
  .chip.active {
    color: var(--text);
    background: var(--glass-2);
    border-color: var(--line-strong);
  }
  @media (max-width: 1023px) {
    .topbar {
      padding: 14px 0 8px;
    }
    .mark-mobile {
      display: grid;
    }
    .mobile-nav {
      display: flex;
    }
    .hint,
    kbd {
      display: none;
    }
    .search-trigger {
      width: 38px;
      padding: 0;
      justify-content: center;
    }
  }
  .perf-toggle.active {
    color: var(--accent);
    border-color: rgba(var(--accent-rgb), 0.4);
    background: rgba(var(--accent-rgb), 0.12);
  }
</style>
