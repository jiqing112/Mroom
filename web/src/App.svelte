<script>
  import { onMount } from 'svelte'
  import { initTheme } from './lib/theme.svelte.js'
  import { initPerf } from './lib/perf.svelte.js'
  import { initRouter, route, navigate } from './lib/router.svelte.js'
  import { auth, loadMe } from './lib/auth.svelte.js'
  import { initPlayer, player, toggle, seek, toggleMute, next, prev, stop } from './lib/player.svelte.js'
  import { ui, openSearch, closeSearch, closeSettings, openStage, closeStage } from './lib/ui.svelte.js'
  import { api } from './lib/api.js'
  import Aurora from './components/Aurora.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import TopBar from './components/TopBar.svelte'
  import PlayerBar from './components/PlayerBar.svelte'
  import NowPlaying from './components/NowPlaying.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import SearchOverlay from './components/SearchOverlay.svelte'
  import PlaylistPicker from './components/PlaylistPicker.svelte'
  import Toast from './components/Toast.svelte'
  import Login from './pages/Login.svelte'
  import Home from './pages/Home.svelte'
  import Songs from './pages/Songs.svelte'
  import Favorites from './pages/Favorites.svelte'
  import Recent from './pages/Recent.svelte'
  import Albums from './pages/Albums.svelte'
  import AlbumDetail from './pages/AlbumDetail.svelte'
  import Artists from './pages/Artists.svelte'
  import ArtistDetail from './pages/ArtistDetail.svelte'
  import Playlists from './pages/Playlists.svelte'
  import PlaylistDetail from './pages/PlaylistDetail.svelte'
  import Upload from './pages/Upload.svelte'

  let ready = $state(false)
  let stats = $state(null)
  let mainEl = $state(null)

  async function loadStats() {
    try {
      const data = await api.get('/api/home')
      stats = data.stats
    } catch {
      /* ignore */
    }
  }

  onMount(() => {
    initPerf()
    initTheme()
    initRouter()

    loadMe().then(() => {
      ready = true
      if (!auth.user) navigate('/login')
      else if (route.name === 'login' && !auth.user.guest) navigate('/')
    })

    const onRefresh = () => loadStats()
    const onNav = () => {
      if (mainEl) mainEl.scrollTo({ top: 0, behavior: 'instant' })
    }
    const onUnauthorized = () => {
      if (!auth.user) return
      stop()
      auth.user = null
      navigate('/login', { replace: true })
    }
    window.addEventListener('mrome:refresh', onRefresh)
    window.addEventListener('mrome:navigate', onNav)
    window.addEventListener('mrome:unauthorized', onUnauthorized)
    return () => {
      window.removeEventListener('mrome:refresh', onRefresh)
      window.removeEventListener('mrome:navigate', onNav)
      window.removeEventListener('mrome:unauthorized', onUnauthorized)
    }
  })

  $effect(() => {
    if (!auth.user) return
    // Only touch the audio pipeline once there is a session: restoring a saved
    // queue on the login screen would request /api/stream and fail with 401.
    initPlayer()
    loadStats()
  })

  $effect(() => {
    if (route.name === 'settings' && auth.user) ui.settingsOpen = true
    if (route.name === 'login' && auth.user && !auth.user.guest) navigate('/')
  })

  function onKey(e) {
    const el = e.target
    const typing =
      el instanceof HTMLElement &&
      (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
    if (typing || !auth.user) return

    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      openSearch()
      return
    }
    if (e.key === 'Escape') {
      if (ui.searchOpen) closeSearch()
      else if (ui.stageOpen) closeStage()
      else if (ui.settingsOpen) closeSettings()
      return
    }
    if (ui.searchOpen || ui.settingsOpen) return

    switch (e.key) {
      case ' ':
        e.preventDefault()
        toggle()
        break
      case 'ArrowRight':
        e.preventDefault()
        seek(player.time + (e.shiftKey ? 30 : 5))
        break
      case 'ArrowLeft':
        e.preventDefault()
        seek(player.time - (e.shiftKey ? 30 : 5))
        break
      case 'ArrowUp':
        e.preventDefault()
        break
      case 'm':
      case 'M':
        toggleMute()
        break
      case 'l':
      case 'L':
        if (player.queue.length) openStage(true)
        break
      case '/':
        e.preventDefault()
        openSearch()
        break
      case 'n':
      case 'N':
        next()
        break
      case 'p':
      case 'P':
        prev()
        break
    }
  }
</script>

<svelte:window onkeydown={onKey} />

{#if !ready}
  <div class="splash">
    <div class="splash-mark"></div>
    <div class="splash-word">MROME</div>
  </div>
{:else if !auth.user || (route.name === 'login' && auth.user.guest)}
  <Aurora />
  <Login />
{:else if route.name === 'login'}
  <Aurora />
  <div class="splash"><div class="splash-word">MROME</div></div>
{:else}
  <div class="app grain">
    <Aurora />
    <Sidebar {stats} />
    <main class="main scroll" bind:this={mainEl}>
      <TopBar />
      <div class="main-inner">
        {#if route.name === 'home' || route.name === 'settings'}
          <Home />
        {:else if route.name === 'songs'}
          <Songs />
        {:else if route.name === 'favorites'}
          <Favorites />
        {:else if route.name === 'recent'}
          <Recent />
        {:else if route.name === 'albums'}
          <Albums />
        {:else if route.name === 'album'}
          {#key route.params.id}<AlbumDetail id={route.params.id} />{/key}
        {:else if route.name === 'artists'}
          <Artists />
        {:else if route.name === 'artist'}
          {#key route.params.id}<ArtistDetail id={route.params.id} />{/key}
        {:else if route.name === 'playlists'}
          <Playlists />
        {:else if route.name === 'playlist'}
          {#key route.params.id}<PlaylistDetail id={route.params.id} />{/key}
        {:else if route.name === 'upload'}
          <Upload />
        {:else}
          <div class="notfound">
            <div class="eyebrow">404</div>
            <h1>这一页没有声音</h1>
            <button class="btn btn-primary" onclick={() => navigate('/')}>回到首页</button>
          </div>
        {/if}
      </div>
    </main>
  </div>

  <PlayerBar />
  <NowPlaying />
  <SettingsModal />
  <SearchOverlay />
  <PlaylistPicker />
  <Toast />
{/if}

<style>
  .app {
    position: relative;
    display: flex;
    height: 100dvh;
    overflow: hidden;
    z-index: 1;
  }
  .main {
    position: relative;
    z-index: 1;
    flex: 1;
    min-width: 0;
    height: 100dvh;
    overflow-y: auto;
    overflow-x: hidden;
    padding-bottom: 132px;
  }
  .main-inner {
    max-width: 1480px;
    margin: 0 auto;
    padding: 0 var(--page-gutter);
  }
  .splash {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 18px;
    z-index: 100;
    background: var(--bg);
  }
  .splash-mark {
    width: 34px;
    height: 34px;
    border-radius: 50%;
    background: conic-gradient(from 200deg, var(--accent), var(--accent-2), var(--accent));
    animation: breathe 2.4s ease-in-out infinite;
  }
  .splash-word {
    font-size: 11px;
    letter-spacing: 0.42em;
    color: var(--text-3);
  }
  .notfound {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 14px;
    padding: 80px 0;
  }
  .notfound h1 {
    font-size: clamp(1.8rem, 4vw, 2.8rem);
    font-weight: 500;
    letter-spacing: -0.04em;
    margin: 0;
  }
  @media (max-width: 860px) {
    .main {
      padding-bottom: 106px;
    }
  }
</style>
