<script>
  import { UploadCloud, FileAudio, Check, X, RotateCw, FolderOpen, ShieldAlert, ArrowRight, Loader2 } from 'lucide-svelte'
  import { api } from '../lib/api.js'
  import { auth } from '../lib/auth.svelte.js'
  import { notify } from '../lib/toast.svelte.js'
  import { navigate } from '../lib/router.svelte.js'
  import { fmtDuration } from '../lib/format.js'

  const AUDIO = /\.(mp3|flac|m4a|m4b|aac|wav|ogg|oga|opus|aiff?|wma)$/i

  let items = $state([])
  let dragging = $state(false)
  let inputEl = $state(null)
  let dirEl = $state(null)
  let active = 0
  let counter = 0

  const totalBytes = $derived(items.reduce((a, b) => a + b.size, 0))
  const sentBytes = $derived(items.reduce((a, b) => a + b.size * (b.progress || 0), 0))
  const overall = $derived(totalBytes > 0 ? sentBytes / totalBytes : 0)
  const doneCount = $derived(items.filter((i) => i.status === 'done').length)
  const busy = $derived(items.some((i) => i.status === 'uploading' || i.status === 'queued'))

  function humanSize(bytes) {
    if (bytes > 1024 * 1024 * 1024) return `${(bytes / 1024 ** 3).toFixed(1)} GB`
    if (bytes > 1024 * 1024) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
    return `${Math.max(1, Math.round(bytes / 1024))} KB`
  }

  function addFiles(fileList) {
    const files = Array.from(fileList || [])
    let added = 0
    for (const f of files) {
      if (!AUDIO.test(f.name)) continue
      items.push({
        key: ++counter,
        file: f,
        name: f.name,
        size: f.size,
        rel: f.webkitRelativePath || '',
        progress: 0,
        status: 'queued',
        error: '',
        id: null
      })
      added++
    }
    if (added === 0 && files.length) notify.err('没有可用的音频文件')
    pump()
  }

  async function pump() {
    while (active < 2) {
      const next = items.find((i) => i.status === 'queued')
      if (!next) break
      active++
      next.status = 'uploading'
      upload(next).finally(() => {
        active--
        pump()
      })
    }
  }

  async function upload(item) {
    try {
      const init = await api.post('/api/upload/init', {
        filename: item.name,
        relativePath: item.rel,
        size: item.size
      })
      item.id = init.id
      let sent = 0
      while (sent < item.size) {
        if (item.status === 'canceled') return
        const end = Math.min(sent + init.chunkSize, item.size)
        const res = await fetch(`/api/upload/${item.id}?offset=${sent}`, {
          method: 'PUT',
          body: item.file.slice(sent, end),
          credentials: 'same-origin'
        })
        if (res.status === 409) {
          const d = await res.json().catch(() => ({}))
          sent = d.received || 0
          continue
        }
        if (!res.ok) {
          const d = await res.json().catch(() => ({}))
          throw new Error(d.error || `上传失败（${res.status}）`)
        }
        const d = await res.json()
        sent = d.received
        item.progress = sent / item.size
      }
      await api.post(`/api/upload/${item.id}/complete`)
      item.status = 'done'
      item.progress = 1
      window.dispatchEvent(new CustomEvent('mrome:refresh'))
      if (items.every((i) => i.status === 'done' || i.status === 'error')) {
        notify.ok('上传完成，已自动入库')
      }
    } catch (e) {
      item.status = 'error'
      item.error = e.message
      notify.err(`${item.name}: ${e.message}`)
    }
  }

  function retry(item) {
    if (item.id) api.del(`/api/upload/${item.id}`).catch(() => {})
    item.status = 'queued'
    item.progress = 0
    item.error = ''
    item.id = null
    pump()
  }

  function cancel(item) {
    item.status = 'canceled'
    if (item.id) api.del(`/api/upload/${item.id}`).catch(() => {})
    items = items.filter((i) => i.key !== item.key)
  }

  function clearDone() {
    items = items.filter((i) => i.status !== 'done')
  }

  function onDrop(e) {
    e.preventDefault()
    dragging = false
    addFiles(e.dataTransfer?.files)
  }
</script>

{#if !auth.user?.isAdmin}
  <div class="denied">
    <ShieldAlert size={22} strokeWidth={1.5} />
    <div>只有管理员可以上传音乐</div>
    <button class="btn" onclick={() => navigate('/')}>返回首页</button>
  </div>
{:else}
  <div class="page rise">
    <header class="page-head">
      <div>
        <div class="eyebrow">Ingest</div>
        <h1 class="page-title">上传音乐</h1>
        <div class="page-sub">拖入文件或文件夹，自动读取标签并归入音乐库</div>
      </div>
      {#if doneCount > 0}
        <button class="btn" onclick={() => navigate('/songs')}>
          查看歌曲 <ArrowRight size={14} />
        </button>
      {/if}
    </header>

    <div
      class="dropzone"
      class:dragging
      role="button"
      tabindex="0"
      onclick={() => inputEl?.click()}
      onkeydown={(e) => e.key === 'Enter' && inputEl?.click()}
      ondragover={(e) => {
        e.preventDefault()
        dragging = true
      }}
      ondragleave={() => (dragging = false)}
      ondrop={onDrop}
    >
      <div class="drop-icon"><UploadCloud size={26} strokeWidth={1.4} /></div>
      <div class="drop-title">拖拽音频到这里</div>
      <div class="drop-sub">或点击选择文件 · 支持 MP3 / FLAC / M4A / WAV / OGG / OPUS</div>
      <div class="drop-actions">
        <button
          class="btn"
          onclick={(e) => {
            e.stopPropagation()
            inputEl?.click()
          }}
        >
          选择文件
        </button>
        <button
          class="btn"
          onclick={(e) => {
            e.stopPropagation()
            dirEl?.click()
          }}
        >
          <FolderOpen size={14} /> 选择文件夹
        </button>
      </div>
      <div class="drop-foot">大文件自动分片上传（4 MB / 片），中断后可从已上传位置继续</div>
    </div>

    <input
      bind:this={inputEl}
      type="file"
      multiple
      accept=".mp3,.flac,.m4a,.m4b,.aac,.wav,.ogg,.oga,.opus,.aiff,.aif,.wma"
      onchange={(e) => {
        addFiles(e.currentTarget.files)
        e.currentTarget.value = ''
      }}
      hidden
    />
    <input
      bind:this={dirEl}
      type="file"
      webkitdirectory
      multiple
      onchange={(e) => {
        addFiles(e.currentTarget.files)
        e.currentTarget.value = ''
      }}
      hidden
    />

    {#if items.length}
      <section class="queue">
        <div class="queue-head">
          <div>
            <span class="eyebrow">Queue</span>
            <span class="q-summary">
              {doneCount}/{items.length} 完成 · {humanSize(sentBytes)} / {humanSize(totalBytes)}
            </span>
          </div>
          <div class="q-actions">
            {#if busy}<span class="live"><Loader2 size={12} class="spin" /> 上传中</span>{/if}
            {#if doneCount > 0}
              <button class="pill" onclick={clearDone}>清除已完成</button>
            {/if}
          </div>
        </div>

        <div class="overall"><i style={`width:${Math.round(overall * 100)}%`}></i></div>

        <div class="files">
          {#each items as item (item.key)}
            <div class="file" class:done={item.status === 'done'} class:error={item.status === 'error'}>
              <div class="f-icon">
                <FileAudio size={16} strokeWidth={1.5} />
              </div>
              <div class="f-main">
                <div class="f-name clamp-1">{item.name}</div>
                <div class="f-sub">
                  <span>{humanSize(item.size)}</span>
                  {#if item.status === 'uploading'}
                    <span class="sep">·</span><span>{Math.round(item.progress * 100)}%</span>
                  {:else if item.status === 'done'}
                    <span class="sep">·</span><span class="ok">已入库</span>
                  {:else if item.status === 'error'}
                    <span class="sep">·</span><span class="bad">{item.error}</span>
                  {:else if item.status === 'queued'}
                    <span class="sep">·</span><span>等待上传</span>
                  {/if}
                </div>
                <div class="f-bar"><i style={`width:${Math.round(item.progress * 100)}%`}></i></div>
              </div>
              <div class="f-end">
                {#if item.status === 'done'}
                  <span class="check"><Check size={15} /></span>
                {:else if item.status === 'error'}
                  <button class="icon-btn" onclick={() => retry(item)} title="重试"><RotateCw size={14} /></button>
                {:else}
                  <button class="icon-btn" onclick={() => cancel(item)} title="取消"><X size={14} /></button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/if}
  </div>
{/if}

<style>
  .page {
    padding-bottom: 24px;
  }
  .dropzone {
    position: relative;
    border: 1px dashed var(--line-strong);
    border-radius: 28px;
    padding: 56px 24px 40px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 8px;
    cursor: pointer;
    background:
      radial-gradient(80% 100% at 50% 0%, rgba(var(--accent-rgb), 0.09), transparent 70%),
      var(--glass);
    transition: all 0.35s var(--ease);
  }
  .dropzone:hover,
  .dropzone.dragging {
    border-color: rgba(var(--accent-rgb), 0.55);
    background:
      radial-gradient(80% 100% at 50% 0%, rgba(var(--accent-rgb), 0.16), transparent 70%),
      var(--glass-2);
    transform: translateY(-2px);
  }
  .drop-icon {
    width: 62px;
    height: 62px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--glass-2);
    border: 1px solid var(--line);
    color: var(--text-2);
    margin-bottom: 8px;
    transition: transform 0.4s var(--ease);
  }
  .dropzone:hover .drop-icon {
    transform: translateY(-3px);
    color: var(--text);
  }
  .drop-title {
    font-size: 19px;
    font-weight: 500;
    letter-spacing: -0.02em;
  }
  .drop-sub {
    font-size: 12px;
    color: var(--text-3);
  }
  .drop-actions {
    display: flex;
    gap: 8px;
    margin-top: 14px;
  }
  .drop-foot {
    margin-top: 18px;
    font-size: 10.5px;
    color: var(--text-3);
    letter-spacing: 0.02em;
  }
  .queue {
    margin-top: 34px;
  }
  .queue-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
  }
  .q-summary {
    font-size: 11.5px;
    color: var(--text-2);
    margin-left: 10px;
    font-variant-numeric: tabular-nums;
  }
  .q-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .live {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 10.5px;
    color: var(--text-2);
  }
  .overall {
    height: 2px;
    border-radius: 2px;
    background: var(--line);
    overflow: hidden;
    margin-bottom: 14px;
  }
  .overall i {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-2));
    transition: width 0.3s var(--ease);
  }
  .files {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border-radius: 14px;
    border: 1px solid var(--line);
    background: var(--glass);
  }
  .file.done {
    border-color: rgba(74, 222, 128, 0.22);
    background: rgba(74, 222, 128, 0.05);
  }
  .file.error {
    border-color: rgba(251, 113, 133, 0.28);
    background: rgba(251, 113, 133, 0.05);
  }
  .f-icon {
    width: 34px;
    height: 34px;
    border-radius: 10px;
    display: grid;
    place-items: center;
    background: var(--glass-2);
    border: 1px solid var(--line);
    color: var(--text-2);
    flex: none;
  }
  .f-main {
    flex: 1;
    min-width: 0;
  }
  .f-name {
    font-size: 12.5px;
  }
  .f-sub {
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 10.5px;
    color: var(--text-3);
    margin-top: 2px;
    flex-wrap: wrap;
  }
  .ok {
    color: #4ade80;
  }
  .bad {
    color: #fb7185;
  }
  .f-bar {
    height: 2px;
    border-radius: 2px;
    background: var(--line);
    margin-top: 7px;
    overflow: hidden;
  }
  .f-bar i {
    display: block;
    height: 100%;
    background: var(--text-2);
    transition: width 0.25s linear;
  }
  .file.done .f-bar i {
    background: #4ade80;
  }
  .f-end {
    flex: none;
    width: 30px;
    display: grid;
    place-items: center;
  }
  .check {
    color: #4ade80;
  }
  .denied {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 14px;
    min-height: 50vh;
    color: var(--text-2);
    font-size: 13px;
  }
  @media (max-width: 860px) {
    .dropzone {
      padding: 40px 18px 30px;
      border-radius: 22px;
    }
    .drop-title {
      font-size: 16px;
    }
    .drop-actions {
      flex-direction: column;
      width: 100%;
      max-width: 220px;
    }
    .drop-actions .btn {
      width: 100%;
    }
  }
</style>
