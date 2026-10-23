<script>
  import { X, RefreshCw, Users, Database, Link2, LayoutGrid, Copy, Check, Trash2, Plus, KeyRound, ShieldCheck, Gauge, Unlock } from 'lucide-svelte'
  import qrcode from 'qrcode-generator'
  import { api } from '../lib/api.js'
  import { ui, closeSettings } from '../lib/ui.svelte.js'
  import { auth } from '../lib/auth.svelte.js'
  import { notify } from '../lib/toast.svelte.js'
  import { fmtDuration, fmtDateShort } from '../lib/format.js'
  import { perf, setPerfMode } from '../lib/perf.svelte.js'

  let tab = $state('overview')
  let data = $state(null)
  let loading = $state(true)
  let scanning = $state(false)
  let scan = $state(null)
  let copied = $state('')
  let newUser = $state({ username: '', password: '', isAdmin: false })
  let busy = $state(false)
  let guestBusy = $state(false)
  let timer = null
  let pollGen = 0

  const isAdmin = $derived(auth.user?.isAdmin)
  const base = $derived(typeof location !== 'undefined' ? `${location.protocol}//${location.host}` : '')
  // QR code pointing at this instance, so a phone on the same LAN can open the
  // web player without typing the IP. Generated locally, nothing is uploaded.
  const qrSvg = $derived.by(() => {
    if (!base) return ''
    try {
      const qr = qrcode(0, 'M')
      qr.addData(base)
      qr.make()
      return qr.createSvgTag({ cellSize: 4, margin: 0, scalable: true })
    } catch {
      return ''
    }
  })
  const apiKey = $derived(
    data?.users?.find((u) => u.username === auth.user?.username)?.apiKey || auth.user?.apiKey || ''
  )

  function stopPolling() {
    pollGen++
    if (timer) clearInterval(timer)
    timer = null
  }

  function startPolling() {
    if (timer) return
    const gen = ++pollGen
    timer = setInterval(async () => {
      try {
        const r = await api.get('/api/admin/scan')
        if (gen !== pollGen) return
        scan = r.scan
        if (!r.scan.running) {
          stopPolling()
          scanning = false
          notify.ok(`扫描完成：新增 ${r.scan.added} 首，更新 ${r.scan.updated} 首`)
          load()
          window.dispatchEvent(new CustomEvent('mrome:refresh'))
        }
      } catch {
        /* transient error: try again on the next tick */
      }
    }, 1000)
  }

  async function load() {
    loading = true
    try {
      if (isAdmin) {
        data = await api.get('/api/admin/settings')
        scan = data.scan
        scanning = !!data.scan?.running
        if (scanning) startPolling()
      }
    } catch (e) {
      notify.err(e.message)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    if (!ui.settingsOpen) {
      stopPolling()
      return
    }
    load()
  })

  $effect(() => {
    if (!ui.settingsOpen) return
    const onKey = (e) => e.key === 'Escape' && closeSettings()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  async function startScan() {
    if (scanning) return
    scanning = true
    try {
      const res = await api.post('/api/admin/scan')
      scan = res.scan
      if (res.scan?.running === false) scanning = false
      else startPolling()
    } catch (e) {
      scanning = false
      notify.err(e.message)
    }
  }

  async function copyText(text, key) {
    try {
      await navigator.clipboard.writeText(text)
      copied = key
      setTimeout(() => (copied = ''), 1600)
    } catch {
      notify.err('复制失败，请手动选择')
    }
  }

  async function createUser() {
    if (!newUser.username || !newUser.password) {
      notify.err('请填写用户名和密码')
      return
    }
    busy = true
    try {
      await api.post('/api/admin/users', newUser)
      newUser = { username: '', password: '', isAdmin: false }
      notify.ok('用户已创建')
      load()
    } catch (e) {
      notify.err(e.message)
    } finally {
      busy = false
    }
  }

  async function toggleAdmin(u) {
    try {
      await api.patch(`/api/admin/users/${u.id}`, { isAdmin: !u.isAdmin })
      load()
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function resetPassword(u) {
    const pwd = prompt(`为 ${u.username} 设置新密码（至少 3 位）`)
    if (!pwd) return
    try {
      await api.patch(`/api/admin/users/${u.id}`, { password: pwd })
      notify.ok('密码已更新')
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function removeUser(u) {
    if (!confirm(`确定删除用户 ${u.username}？此操作不可撤销。`)) return
    try {
      await api.del(`/api/admin/users/${u.id}`)
      notify.ok('用户已删除')
      load()
    } catch (e) {
      notify.err(e.message)
    }
  }

  async function regenKey(u) {
    if (!confirm('重新生成 API Key？旧 Key 将立即失效。')) return
    try {
      const r = await api.post(`/api/admin/users/${u.id}/apikey`)
      notify.ok('已生成新的 API Key')
      load()
      copied = ''
      return r.apiKey
    } catch (e) {
      notify.err(e.message)
    }
  }

  // One-click switch for the "no login required" mode. The server persists the
  // choice and rejects turning it off while no administrator exists.
  async function toggleGuest() {
    if (!data || guestBusy) return
    const next = !data.guestMode
    if (!next && !confirm('关闭后必须登录才能访问，确定关闭免登录？')) return
    guestBusy = true
    try {
      const r = await api.patch('/api/admin/guest-mode', { enabled: next })
      data.guestMode = r.guestMode
      auth.guestMode = r.guestMode
      notify.ok(next ? '已开启免登录访问' : '已关闭免登录访问')
    } catch (e) {
      notify.err(e.message)
    } finally {
      guestBusy = false
    }
  }

  const progress = $derived(scan && scan.total > 0 ? Math.min(100, (scan.done / scan.total) * 100) : 0)
</script>

{#if ui.settingsOpen}
  <div class="overlay" role="presentation" onclick={closeSettings}>
    <div
      class="panel glass-deep"
      role="dialog"
      aria-modal="true"
      aria-label="高级设置"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => {
        if (e.key === 'Escape') closeSettings()
        e.stopPropagation()
      }}
    >
      <aside class="side">
        <div class="side-head">
          <div class="eyebrow">Advanced</div>
          <h2>设置</h2>
        </div>
        <button class="s-tab" class:active={tab === 'overview'} onclick={() => (tab = 'overview')}>
          <LayoutGrid size={15} strokeWidth={1.6} /> 概览
        </button>
        <button class="s-tab" class:active={tab === 'appearance'} onclick={() => (tab = 'appearance')}>
          <Gauge size={15} strokeWidth={1.6} /> 外观与性能
        </button>
        {#if isAdmin}
          <button class="s-tab" class:active={tab === 'library'} onclick={() => (tab = 'library')}>
            <Database size={15} strokeWidth={1.6} /> 音乐库
          </button>
          <button class="s-tab" class:active={tab === 'users'} onclick={() => (tab = 'users')}>
            <Users size={15} strokeWidth={1.6} /> 用户
          </button>
        {/if}
        <button class="s-tab" class:active={tab === 'connect'} onclick={() => (tab = 'connect')}>
          <Link2 size={15} strokeWidth={1.6} /> 客户端连接
        </button>
        <div class="grow"></div>
        <div class="version">
          <div>Mrome 1.0.0</div>
          <div class="faint">Subsonic API 1.16.1</div>
        </div>
      </aside>

      <section class="content">
        <button class="icon-btn close" onclick={closeSettings} aria-label="关闭">
          <X size={17} strokeWidth={1.6} />
        </button>

        {#if tab === 'overview'}
          <div class="pane">
            <header class="pane-head">
              <div>
                <div class="eyebrow">Overview</div>
                <h3>音乐库概览</h3>
              </div>
              {#if isAdmin}
                <button class="btn" onclick={startScan} disabled={scanning}>
                  <RefreshCw size={14} class={scanning ? 'spin' : ''} /> {scanning ? '扫描中…' : '重新扫描'}
                </button>
              {/if}
            </header>

            <div class="stats">
              <div class="stat glass">
                <div class="k">曲目</div>
                <div class="v">{data?.stats?.songs ?? '—'}</div>
              </div>
              <div class="stat glass">
                <div class="k">专辑</div>
                <div class="v">{data?.stats?.albums ?? '—'}</div>
              </div>
              <div class="stat glass">
                <div class="k">艺术家</div>
                <div class="v">{data?.stats?.artists ?? '—'}</div>
              </div>
              <div class="stat glass">
                <div class="k">总时长</div>
                <div class="v small">{fmtDuration(data?.stats?.duration)}</div>
              </div>
            </div>

            {#if scan}
              <div class="scan-card glass">
                <div class="scan-top">
                  <div>
                    <div class="scan-title">
                      {scan.running ? '正在扫描音乐目录' : '上次扫描已完成'}
                    </div>
                    <div class="scan-sub">
                      {#if scan.running}
                        {scan.done} / {scan.total} · {scan.current || '准备中'}
                      {:else}
                        {scan.total} 个文件 · 新增 {scan.added} · 更新 {scan.updated} · 移除 {scan.removed}
                        {#if scan.finishedAt}<span class="faint"> · {fmtDateShort(scan.finishedAt)}</span>{/if}
                      {/if}
                    </div>
                  </div>
                  <div class="scan-num">{Math.round(progress)}%</div>
                </div>
                <div class="bar"><i style={`width:${scan.running ? progress : 100}%`}></i></div>
              </div>
            {/if}

            <div class="notes">
              <h4>关于 Mrome</h4>
              <p>
                一个自托管的私人音乐库：上传文件、自动解析标签、按专辑与艺术家归档，并以 Subsonic
                协议对外提供浏览与流式播放。网页端使用玻璃拟态与随封面流动的极光背景。
              </p>
              <p class="faint">
                右上角齿轮图标（或双击左上角 Logo）可随时回到这里；所有设置在本地 SQLite 中保存，无需外部服务。
              </p>
            </div>
          </div>
        {:else if tab === 'appearance'}
          <div class="pane">
            <header class="pane-head">
              <div>
                <div class="eyebrow">Rendering</div>
                <h3>外观与性能</h3>
              </div>
            </header>

            <div class="perf-card glass">
              <div class="perf-title">渲染模式</div>
              <p class="perf-desc">
                完整效果包含极光动画、毛玻璃、封面模糊与循环动效。流畅模式会把这些逐帧重绘的
                效果换成静态配色，播放音频本身几乎不占用 GPU；静止模式在此之上再停止黑胶与均衡器
                动画，适合远程桌面、核显或需要极致省电的场景。
              </p>

              <div class="seg" role="group" aria-label="渲染模式">
                <button class="seg-btn" class:active={perf.mode === 'auto'} onclick={() => setPerfMode('auto')}>
                  自动
                </button>
                <button class="seg-btn" class:active={perf.mode === 'full'} onclick={() => setPerfMode('full')}>
                  完整效果
                </button>
                <button class="seg-btn" class:active={perf.mode === 'smooth'} onclick={() => setPerfMode('smooth')}>
                  流畅模式
                </button>
                <button class="seg-btn" class:active={perf.mode === 'static'} onclick={() => setPerfMode('static')}>
                  静止模式
                </button>
              </div>

              <div class="perf-state">
                {#if perf.static}
                  <span class="dot on"></span>
                  <span>正在使用<strong>静止模式</strong><span class="faint">（已停止装饰动画）</span></span>
                {:else if perf.smooth}
                  <span class="dot on"></span>
                  <span>
                    正在使用<strong>流畅模式</strong>
                    {#if perf.mode === 'auto' && perf.reasons.length}
                      <span class="faint">（自动判定：{perf.reasons.join(' · ')}）</span>
                    {/if}
                  </span>
                {:else}
                  <span class="dot"></span>
                  <span>正在使用<strong>完整效果</strong></span>
                {/if}
              </div>

              <div class="perf-list">
                <div class="perf-item"><span>极光背景</span><span>{perf.smooth ? '静态渐变' : 'Canvas 动画'}</span></div>
                <div class="perf-item"><span>毛玻璃 / 背景模糊</span><span>{perf.smooth ? '关闭' : '开启'}</span></div>
                <div class="perf-item"><span>封面模糊、光晕、噪点</span><span>{perf.smooth ? '关闭' : '开启'}</span></div>
                <div class="perf-item"><span>黑胶旋转与均衡器</span><span>{perf.static ? '关闭' : perf.smooth ? '开启（合成层）' : '开启'}</span></div>
              </div>
            </div>

            <div class="notes">
              <h4>低配设备建议</h4>
              <p>
                如果播放时风扇转速升高、拖动进度条掉帧，或者远程桌面里画面卡顿，选「流畅模式」；
                如果连黑胶旋转都希望停止，选「静止模式」。设置保存在本机浏览器，刷新后依然生效。
              </p>
              <p>
                流畅模式下仍保留黑胶旋转与播放均衡器——它们是 <code>transform</code> 动画，
                由合成层处理，不触发重绘，几乎不占 GPU。
              </p>
            </div>
          </div>
        {:else if tab === 'library'}
          <div class="pane">
            <header class="pane-head">
              <div>
                <div class="eyebrow">Storage</div>
                <h3>音乐库</h3>
              </div>
              <button class="btn" onclick={startScan} disabled={scanning}>
                <RefreshCw size={14} class={scanning ? 'spin' : ''} /> 扫描
              </button>
            </header>

            <div class="field-list">
              <div class="field">
                <div class="label">音乐目录</div>
                <div class="value mono">{data?.musicDir || '—'}</div>
                <div class="hint">将音频文件放入该目录后点击“扫描”，或通过上传页面上传。</div>
              </div>
              <div class="field">
                <div class="label">数据库</div>
                <div class="value mono">{data?.dbPath || '—'}</div>
              </div>
              <div class="field">
                <div class="label">单文件大小上限</div>
                <div class="value">{data?.maxUploadMB || 512} MB</div>
                <div class="hint">可通过环境变量 MROME_MAX_UPLOAD_MB 调整。</div>
              </div>
              <div class="field">
                <div class="label">支持格式</div>
                <div class="value">MP3 · FLAC · M4A · WAV · OGG · OPUS · AAC · AIFF · WMA</div>
              </div>
            </div>

            <div class="notes">
              <h4>扫描行为</h4>
              <p>
                扫描会遍历音乐目录、读取 ID3 / Vorbis / MP4 标签与内嵌封面，自动归入艺术家与专辑；
                无标签文件会进入“未知艺术家 / 未知专辑”。若目录中存在 cover.jpg，会作为专辑封面。
              </p>
              <p class="faint">删除的音频文件会在下次扫描时从库中移除，不会删除磁盘文件。</p>
            </div>
          </div>
        {:else if tab === 'users'}
          <div class="pane">
            <header class="pane-head">
              <div>
                <div class="eyebrow">Accounts</div>
                <h3>用户管理</h3>
              </div>
            </header>

            <div class="guest-card glass">
              <div class="guest-text">
                <div class="guest-title">
                  免登录访问
                  {#if data?.guestMode}<span class="tag"><Unlock size={11} /> 已开启</span>{/if}
                </div>
                <p class="guest-desc">
                  开启后，任何人打开网页都会以「访客」身份直接进入，可以浏览、搜索、播放与收藏，
                  但不能上传、扫描或管理用户。关闭后必须登录才能访问。
                </p>
                {#if !data?.canLogin}
                  <p class="guest-warn">还没有管理员账号，请先创建一个管理员再关闭免登录。</p>
                {/if}
              </div>
              <button
                class="switch"
                class:on={data?.guestMode}
                role="switch"
                aria-checked={data?.guestMode}
                aria-label="免登录访问"
                onclick={toggleGuest}
                disabled={guestBusy || (data?.guestMode && !data?.canLogin)}
              >
                <i></i>
              </button>
            </div>

            <div class="user-list">
              {#each data?.users || [] as u}
                <div class="u-row">
                  <div class="u-avatar">{u.username.slice(0, 1).toUpperCase()}</div>
                  <div class="u-meta">
                    <div class="u-name">
                      {u.username}
                      {#if u.isAdmin}<span class="tag"><ShieldCheck size={11} /> 管理员</span>{/if}
                    </div>
                    <div class="u-key mono">{u.apiKey ? u.apiKey.slice(0, 10) + '…' : '无 API Key'}</div>
                  </div>
                  <div class="u-actions">
                    <button class="pill" onclick={() => toggleAdmin(u)}>{u.isAdmin ? '取消管理员' : '设为管理员'}</button>
                    <button class="icon-btn" onclick={() => resetPassword(u)} title="重置密码"><KeyRound size={14} /></button>
                    <button class="icon-btn" onclick={() => regenKey(u)} title="重置 API Key"><RefreshCw size={14} /></button>
                    <button class="icon-btn" onclick={() => removeUser(u)} title="删除用户"><Trash2 size={14} /></button>
                  </div>
                </div>
              {/each}
            </div>

            <div class="new-user glass">
              <div class="eyebrow">New account</div>
              <div class="nu-grid">
                <input placeholder="用户名" bind:value={newUser.username} autocomplete="off" />
                <input placeholder="密码（至少 3 位）" type="password" bind:value={newUser.password} autocomplete="new-password" />
                <label class="check">
                  <input type="checkbox" bind:checked={newUser.isAdmin} />
                  <span>管理员</span>
                </label>
                <button class="btn btn-primary" onclick={createUser} disabled={busy}>
                  <Plus size={14} /> 创建
                </button>
              </div>
            </div>
          </div>
        {:else if tab === 'connect'}
          <div class="pane">
            <header class="pane-head">
              <div>
                <div class="eyebrow">Subsonic</div>
                <h3>客户端连接</h3>
              </div>
              <span class="status"><i></i> 服务运行中</span>
            </header>

            <div class="conn">
              <div class="conn-row">
                <div class="conn-label">服务器地址</div>
                <div class="conn-value mono">{base}</div>
                <button class="icon-btn" onclick={() => copyText(base, 'url')}>
                  {#if copied === 'url'}<Check size={14} />{:else}<Copy size={14} />{/if}
                </button>
              </div>
              <div class="conn-row">
                <div class="conn-label">API 路径</div>
                <div class="conn-value mono faint">{base}/rest</div>
                <button class="icon-btn" onclick={() => copyText(`${base}/rest`, 'api')}>
                  {#if copied === 'api'}<Check size={14} />{:else}<Copy size={14} />{/if}
                </button>
              </div>
              <div class="conn-row">
                <div class="conn-label">用户名</div>
                <div class="conn-value mono">{auth.user?.username}</div>
              </div>
              <div class="conn-row">
                <div class="conn-label">密码</div>
                <div class="conn-value faint">你的登录密码（客户端使用 token 认证）</div>
              </div>
              {#if apiKey}
                <div class="conn-row">
                  <div class="conn-label">API Key</div>
                  <div class="conn-value mono">{apiKey}</div>
                  <button class="icon-btn" onclick={() => copyText(apiKey, 'key')}>
                    {#if copied === 'key'}<Check size={14} />{:else}<Copy size={14} />{/if}
                  </button>
                </div>
              {/if}
            </div>
            <p class="conn-note">
              客户端（音流、DSub、Symfonium 等）的「服务器地址」只填上面的主机地址，<code>/rest</code>
              由客户端自己拼接；多写一段会变成 <code>/rest/rest/ping.view</code> 而连不上。
            </p>

            {#if qrSvg}
              <div class="qr-card glass">
                <div class="qr-box">{@html qrSvg}</div>
                <div class="qr-text">
                  <div class="qr-title">扫码在手机上打开</div>
                  <p class="qr-desc">
                    手机与服务器在同一局域网时，用相机或微信扫一下即可打开网页播放器，
                    免去手输 IP。扫码打不开通常是手机连了别的 Wi-Fi。
                  </p>
                  <div class="qr-url mono">{base}</div>
                </div>
              </div>
            {/if}

            <div class="proto glass">
              <div class="proto-head">支持的客户端协议</div>
              <div class="proto-row">
                <span class="proto-name">Subsonic / OpenSubsonic</span>
                <span class="proto-addr mono">{base}/rest</span>
              </div>
              <div class="proto-row">
                <span class="proto-name">Navidrome</span>
                <span class="proto-addr mono">{base}</span>
              </div>
              <div class="proto-row">
                <span class="proto-name">Jellyfin</span>
                <span class="proto-addr mono">{base}</span>
              </div>
              <div class="proto-row">
                <span class="proto-name">Emby</span>
                <span class="proto-addr mono">{base}/emby</span>
              </div>
              <div class="proto-row">
                <span class="proto-name">Audio Station</span>
                <span class="proto-addr mono">{base}</span>
              </div>
              <div class="proto-row">
                <span class="proto-name">Plex</span>
                <span class="proto-addr mono">{base}</span>
              </div>
              {#if data?.plexToken}
                <div class="proto-token">
                  <span class="faint">Plex Token</span>
                  <code class="mono">{data.plexToken}</code>
                  <button class="icon-btn" onclick={() => copyText(data.plexToken, 'plex')}>
                    {#if copied === 'plex'}<Check size={13} />{:else}<Copy size={13} />{/if}
                  </button>
                </div>
              {/if}
              <p class="proto-note">
                用户名密码与网页登录相同。Jellyfin / Emby 需要在客户端里选择对应类型；
                Plex 客户端通常要登录 Plex 账号，只有支持「手动地址 + Token」的版本能直连本机。
              </p>
            </div>

            <div class="notes">
              <h4>兼容性说明</h4>
              <p>
                已实现 ping、getArtists、getArtist、getAlbum、getSong、getAlbumList2、search3、stream（支持
                Range / 206）、getCoverArt、歌单增删改查、star / unstar、setRating、scrobble 等核心端点；
                视频、jukebox 与聊天类接口返回标准“数据未找到”错误。
              </p>
              <div class="chips">
                <span class="chip-tag">OpenSubsonic</span>
                <span class="chip-tag">apiKeyAuthentication</span>
                <span class="chip-tag">songLyrics</span>
                <span class="chip-tag">CORS 已开启</span>
              </div>
              <p class="faint mono curl">
                curl "{base}/rest/ping.view?u={auth.user?.username}&amp;t=...&amp;s=...&amp;v=1.16.1&amp;c=test&amp;f=json"
              </p>
            </div>
          </div>
        {/if}
      </section>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 85;
    background: rgba(4, 5, 8, 0.5);
    backdrop-filter: blur(10px);
    display: grid;
    place-items: center;
    padding: 28px;
    animation: fade-in 0.3s var(--ease) both;
  }
  .panel {
    width: min(960px, 96vw);
    height: min(700px, 88vh);
    border-radius: 28px;
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    overflow: hidden;
    animation: rise 0.45s var(--ease) both;
  }
  .side {
    padding: 26px 16px 20px;
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 3px;
    background: var(--glass);
  }
  .side-head {
    padding: 0 10px 18px;
  }
  .side-head h2 {
    margin: 6px 0 0;
    font-size: 20px;
    font-weight: 500;
    letter-spacing: -0.02em;
  }
  .s-tab {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 38px;
    padding: 0 10px;
    border-radius: 10px;
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-2);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.25s var(--ease);
    text-align: left;
  }
  .s-tab:hover {
    color: var(--text);
    background: var(--glass);
  }
  .s-tab.active {
    color: var(--text);
    background: var(--glass-2);
    border-color: var(--line);
  }
  .grow {
    flex: 1;
  }
  .version {
    padding: 0 10px;
    font-size: 10.5px;
    color: var(--text-2);
    line-height: 1.7;
  }
  .content {
    position: relative;
    overflow-y: auto;
    padding: 30px 34px 34px;
  }
  .close {
    position: absolute;
    top: 18px;
    right: 18px;
    z-index: 2;
  }
  .pane-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 22px;
  }
  .pane-head h3 {
    margin: 6px 0 0;
    font-size: 23px;
    font-weight: 500;
    letter-spacing: -0.03em;
  }
  .stats {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 10px;
    margin-bottom: 16px;
  }
  .stat {
    border-radius: 16px;
    padding: 14px 15px;
  }
  .stat .k {
    font-size: 10.5px;
    color: var(--text-3);
    letter-spacing: 0.06em;
  }
  .stat .v {
    font-size: 24px;
    font-weight: 300;
    letter-spacing: -0.03em;
    margin-top: 6px;
    font-variant-numeric: tabular-nums;
  }
  .stat .v.small {
    font-size: 14px;
    font-weight: 450;
    padding: 5px 0;
  }
  .scan-card {
    border-radius: 18px;
    padding: 16px 18px;
    margin-bottom: 22px;
  }
  .scan-top {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .scan-title {
    font-size: 13px;
    font-weight: 500;
  }
  .scan-sub {
    font-size: 11.5px;
    color: var(--text-3);
    margin-top: 4px;
    word-break: break-all;
  }
  .scan-num {
    font-size: 18px;
    font-weight: 300;
    font-variant-numeric: tabular-nums;
    color: var(--text-2);
  }
  .bar {
    height: 3px;
    border-radius: 99px;
    background: var(--line);
    margin-top: 14px;
    overflow: hidden;
  }
  .bar i {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-2));
    transition: width 0.4s var(--ease);
  }
  .notes {
    margin-top: 22px;
  }
  .perf-card {
    border-radius: 18px;
    padding: 20px 20px 18px;
  }
  .perf-title {
    font-size: 13.5px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }
  .perf-desc {
    margin: 8px 0 16px;
    font-size: 12px;
    line-height: 1.75;
    color: var(--text-2);
  }
  .seg {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 4px;
    padding: 4px;
    border-radius: 12px;
    border: 1px solid var(--line);
    background: var(--glass);
  }
  .seg-btn {
    height: 34px;
    border: 1px solid transparent;
    border-radius: 9px;
    background: none;
    color: var(--text-2);
    font-size: 12.5px;
    font-family: var(--font-sans);
    cursor: pointer;
    transition: all 0.22s var(--ease);
  }
  .seg-btn:hover {
    color: var(--text);
    background: var(--glass-2);
  }
  .seg-btn.active {
    color: var(--bg);
    background: var(--text);
    font-weight: 500;
  }
  .perf-state {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 14px;
    font-size: 12px;
    color: var(--text-2);
  }
  .perf-state strong {
    font-weight: 500;
    color: var(--text);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-3);
    flex: none;
  }
  .dot.on {
    background: var(--accent);
    box-shadow: 0 0 0 3px rgba(var(--accent-rgb), 0.18);
  }
  .perf-list {
    margin-top: 14px;
    padding-top: 14px;
    border-top: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 7px;
  }
  .perf-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 11.5px;
    color: var(--text-3);
  }
  .perf-item span:last-child {
    color: var(--text-2);
    font-variant-numeric: tabular-nums;
  }
  .notes h4 {
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.06em;
    margin: 0 0 8px;
    color: var(--text-2);
  }
  .notes p {
    font-size: 12.5px;
    line-height: 1.85;
    color: var(--text-2);
    margin: 0 0 8px;
  }
  .field-list {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .field {
    padding: 14px 16px;
    border: 1px solid var(--line);
    border-radius: 14px;
    background: var(--glass);
  }
  .field .label {
    font-size: 10.5px;
    letter-spacing: 0.08em;
    color: var(--text-3);
    text-transform: uppercase;
  }
  .field .value {
    margin-top: 6px;
    font-size: 13px;
    word-break: break-all;
  }
  .field .hint {
    margin-top: 5px;
    font-size: 11px;
    color: var(--text-3);
  }
  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11.5px;
  }
  .guest-card {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 18px;
    border-radius: 18px;
    margin-bottom: 18px;
  }
  .guest-text {
    min-width: 0;
  }
  .guest-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13.5px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }
  .guest-desc {
    margin: 7px 0 0;
    font-size: 12px;
    line-height: 1.7;
    color: var(--text-2);
    max-width: 46ch;
  }
  .guest-warn {
    margin: 7px 0 0;
    font-size: 11.5px;
    color: #f0a35e;
  }
  .switch {
    position: relative;
    flex: none;
    width: 44px;
    height: 25px;
    margin-top: 2px;
    padding: 0;
    border-radius: 999px;
    border: 1px solid var(--line-strong);
    background: var(--glass-2);
    cursor: pointer;
    transition:
      background 0.22s var(--ease),
      border-color 0.22s var(--ease);
  }
  .switch i {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 19px;
    height: 19px;
    border-radius: 50%;
    background: var(--text-3);
    transition:
      transform 0.22s var(--ease),
      background 0.22s var(--ease);
  }
  .switch.on {
    background: rgba(var(--accent-rgb), 0.24);
    border-color: rgba(var(--accent-rgb), 0.5);
  }
  .switch.on i {
    transform: translateX(19px);
    background: var(--accent);
  }
  .switch:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .user-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 18px;
  }
  .u-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border: 1px solid var(--line);
    border-radius: 14px;
    background: var(--glass);
  }
  .u-avatar {
    width: 34px;
    height: 34px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 12px;
    background: linear-gradient(140deg, rgba(var(--accent-rgb), 0.45), var(--glass-2));
    border: 1px solid var(--line-strong);
  }
  .u-meta {
    flex: 1;
    min-width: 0;
  }
  .u-name {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  .u-key {
    color: var(--text-3);
    margin-top: 2px;
  }
  .tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 9.5px;
    padding: 2px 7px;
    border-radius: 999px;
    background: rgba(var(--accent-rgb), 0.14);
    color: var(--accent);
    border: 1px solid rgba(var(--accent-rgb), 0.25);
  }
  .u-actions {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .new-user {
    border-radius: 18px;
    padding: 16px;
  }
  .nu-grid {
    display: grid;
    grid-template-columns: 1fr 1fr auto auto;
    gap: 8px;
    align-items: center;
    margin-top: 10px;
  }
  input[type='password'],
  input:not([type]) {
    height: 38px;
    padding: 0 12px;
    border-radius: 10px;
    border: 1px solid var(--line);
    background: var(--bg);
    color: var(--text);
    font-size: 12.5px;
    font-family: var(--font-sans);
    outline: none;
    transition: border-color 0.2s var(--ease);
  }
  input:focus {
    border-color: rgba(var(--accent-rgb), 0.6);
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-2);
    white-space: nowrap;
  }
  .conn {
    border: 1px solid var(--line);
    border-radius: 18px;
    overflow: hidden;
    background: var(--glass);
  }
  .conn-row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 13px 16px;
    border-bottom: 1px solid var(--line);
  }
  .conn-row:last-child {
    border-bottom: none;
  }
  .conn-label {
    width: 84px;
    flex: none;
    font-size: 11px;
    color: var(--text-3);
    letter-spacing: 0.04em;
  }
  .conn-value {
    flex: 1;
    min-width: 0;
    font-size: 12.5px;
    word-break: break-all;
  }
  .conn-note {
    margin: 10px 2px 18px;
    font-size: 11.5px;
    line-height: 1.75;
    color: var(--text-3);
  }
  .conn-note code {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    padding: 1px 5px;
    border-radius: 5px;
    background: var(--glass-2);
    color: var(--text-2);
  }
  .qr-card {
    display: flex;
    align-items: center;
    gap: 18px;
    padding: 16px 18px;
    border-radius: 18px;
    margin-bottom: 20px;
  }
  /* The code has to keep dark modules on a light field in both themes, otherwise
     scanners stop reading it. */
  .qr-box {
    flex: none;
    width: 132px;
    height: 132px;
    padding: 9px;
    border-radius: 14px;
    background: #ffffff;
    box-shadow: 0 8px 24px -14px rgba(0, 0, 0, 0.55);
  }
  .qr-box :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }
  .qr-text {
    min-width: 0;
  }
  .qr-title {
    font-size: 13.5px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }
  .qr-desc {
    margin: 7px 0 10px;
    font-size: 12px;
    line-height: 1.7;
    color: var(--text-2);
    max-width: 46ch;
  }
  .qr-url {
    font-size: 11.5px;
    color: var(--text-3);
    word-break: break-all;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 11px;
    color: var(--text-2);
  }
  .status i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #4ade80;
    box-shadow: 0 0 10px #4ade80;
  }
  .proto {
    border-radius: 18px;
    padding: 16px 18px;
    margin-bottom: 18px;
  }
  .proto-head {
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.01em;
    margin-bottom: 10px;
  }
  .proto-row {
    display: flex;
    align-items: baseline;
    gap: 14px;
    padding: 7px 0;
    border-top: 1px solid var(--line);
  }
  .proto-name {
    width: 168px;
    flex: none;
    font-size: 12px;
    color: var(--text-2);
  }
  .proto-addr {
    flex: 1;
    min-width: 0;
    font-size: 11.5px;
    color: var(--text-3);
    word-break: break-all;
  }
  .proto-token {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-top: 9px;
    border-top: 1px solid var(--line);
    font-size: 11.5px;
  }
  .proto-token code {
    flex: 1;
    min-width: 0;
    font-size: 11px;
    color: var(--text-2);
    word-break: break-all;
  }
  .proto-note {
    margin: 10px 0 0;
    font-size: 11.5px;
    line-height: 1.75;
    color: var(--text-3);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 12px 0;
  }
  .chip-tag {
    font-size: 10px;
    padding: 3px 9px;
    border-radius: 999px;
    border: 1px solid var(--line);
    color: var(--text-2);
    background: var(--glass);
  }
  .curl {
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--bg);
    border: 1px solid var(--line);
    overflow-x: auto;
    white-space: nowrap;
  }
  @media (max-width: 760px) {
    .overlay {
      padding: 0;
    }
    .panel {
      width: 100vw;
      height: 100dvh;
      border-radius: 0;
      grid-template-columns: 1fr;
      grid-template-rows: auto 1fr;
    }
    .side {
      flex-direction: row;
      align-items: center;
      gap: 4px;
      padding: 14px 12px;
      border-right: none;
      border-bottom: 1px solid var(--line);
      overflow-x: auto;
    }
    .side-head,
    .version,
    .grow {
      display: none;
    }
    .s-tab {
      height: 32px;
      padding: 0 11px;
      white-space: nowrap;
    }
    .content {
      padding: 20px 18px 30px;
    }
    .stats {
      grid-template-columns: 1fr 1fr;
    }
    .nu-grid {
      grid-template-columns: 1fr;
    }
    .seg {
      grid-template-columns: repeat(2, 1fr);
    }
    .u-row {
      flex-wrap: wrap;
    }
  }
</style>
