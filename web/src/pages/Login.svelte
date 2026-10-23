<script>
  import { ArrowRight, Loader2, Sun, Moon, Disc3, Radio } from 'lucide-svelte'
  import { login } from '../lib/auth.svelte.js'
  import { navigate } from '../lib/router.svelte.js'
  import { theme, toggleTheme } from '../lib/theme.svelte.js'

  let username = $state('')
  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(e) {
    e.preventDefault()
    if (busy) return
    error = ''
    busy = true
    try {
      await login(username.trim(), password)
      navigate('/')
    } catch (err) {
      error = err.message || '登录失败'
      busy = false
    }
  }
</script>

<div class="login">
  <div class="visual">
    <div class="visual-inner">
      <div class="brand">
        <div class="mark"><span></span></div>
        <span class="wordmark">MROME</span>
      </div>

      <div class="statement">
        <div class="eyebrow">Self-hosted · Subsonic compatible</div>
        <h1>
          把音乐<br />
          留在<span class="serif">自己</span>手里
        </h1>
        <p>
          上传你的收藏，让标签自动归档。随时随地在网页或任意 Subsonic 客户端里，
          以它应有的样子聆听。
        </p>
      </div>

      <div class="disc-deco" aria-hidden="true">
        <div class="disc">
          <div class="label"></div>
          <div class="hole"></div>
        </div>
      </div>

      <div class="visual-foot">
        <span><Radio size={12} /> Subsonic 1.16.1</span>
        <span><Disc3 size={12} /> FLAC / MP3 / M4A</span>
      </div>
    </div>
  </div>

  <div class="form-side">
    <button class="theme-btn icon-btn" onclick={toggleTheme} aria-label="切换主题">
      {#if theme.mode === 'dark'}<Sun size={16} strokeWidth={1.6} />{:else}<Moon size={16} strokeWidth={1.6} />{/if}
    </button>

    <form class="card glass" onsubmit={submit}>
      <div class="card-head">
        <div class="eyebrow">Welcome back</div>
        <h2>登录你的音乐库</h2>
        <p>使用管理员或已创建的账号继续。</p>
      </div>

      <label class="field">
        <span>用户名</span>
        <input bind:value={username} placeholder="admin" autocomplete="username" spellcheck="false" />
      </label>

      <label class="field">
        <span>密码</span>
        <input bind:value={password} type="password" placeholder="••••••" autocomplete="current-password" />
      </label>

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <button class="submit" type="submit" disabled={busy || !username || !password}>
        {#if busy}
          <Loader2 size={16} class="spin" /> 正在进入
        {:else}
          进入音乐库 <ArrowRight size={16} />
        {/if}
      </button>

      <div class="hint">
        首次部署默认账号为 <code>admin</code> / <code>admin</code>，登录后请尽快修改。
      </div>
    </form>

    <div class="foot">Mrome 1.0.0 — 私人音乐库</div>
  </div>
</div>

<style>
  .login {
    position: relative;
    z-index: 1;
    min-height: 100dvh;
    display: grid;
    grid-template-columns: 1.15fr 1fr;
  }
  .visual {
    position: relative;
    display: flex;
    align-items: center;
    padding: 56px clamp(40px, 6vw, 90px);
    overflow: hidden;
  }
  .visual::after {
    content: '';
    position: absolute;
    inset: 0;
    background: radial-gradient(80% 60% at 20% 20%, rgba(var(--accent-rgb), 0.18), transparent 60%);
    pointer-events: none;
  }
  .visual-inner {
    position: relative;
    z-index: 2;
    display: flex;
    flex-direction: column;
    gap: 40px;
    width: 100%;
    max-width: 620px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .mark {
    width: 30px;
    height: 30px;
    border-radius: 50%;
    background: conic-gradient(from 200deg, var(--accent), var(--accent-2), var(--accent));
    display: grid;
    place-items: center;
    box-shadow: 0 10px 30px -10px rgba(var(--accent-rgb), 0.8);
  }
  .mark span {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #0a0a0f;
    box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.3);
  }
  .wordmark {
    font-size: 12px;
    letter-spacing: 0.34em;
    font-weight: 600;
  }
  .statement h1 {
    font-size: clamp(2.6rem, 4.6vw, 4.3rem);
    line-height: 1.08;
    letter-spacing: -0.045em;
    font-weight: 400;
    margin: 18px 0 22px;
  }
  .serif {
    font-family: var(--font-serif);
    font-style: italic;
    color: var(--accent);
    padding-right: 4px;
  }
  .statement p {
    font-size: 14px;
    line-height: 1.9;
    color: var(--text-2);
    max-width: 430px;
    margin: 0;
  }
  .disc-deco {
    position: absolute;
    right: -12%;
    top: 46%;
    width: 460px;
    height: 460px;
    opacity: 0.16;
    pointer-events: none;
    animation: spin 60s linear infinite;
  }
  .disc {
    position: relative;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    background: repeating-radial-gradient(circle at 50% 50%, #fff 0 1px, transparent 1px 7px);
  }
  .label {
    position: absolute;
    inset: 33%;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.5);
  }
  .hole {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 8px;
    height: 8px;
    margin: -4px 0 0 -4px;
    border-radius: 50%;
    background: var(--bg);
    border: 1px solid rgba(255, 255, 255, 0.4);
  }
  .visual-foot {
    display: flex;
    gap: 26px;
    font-size: 10.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .visual-foot span {
    display: inline-flex;
    align-items: center;
    gap: 7px;
  }
  .form-side {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px;
    border-left: 1px solid var(--line);
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent);
  }
  .theme-btn {
    position: absolute;
    top: 26px;
    right: 26px;
  }
  .card {
    width: min(390px, 100%);
    border-radius: 26px;
    padding: 34px 30px 28px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    animation: rise 0.7s var(--ease) both;
  }
  .card-head h2 {
    font-size: 24px;
    font-weight: 500;
    letter-spacing: -0.03em;
    margin: 8px 0 6px;
  }
  .card-head p {
    font-size: 12px;
    color: var(--text-3);
    margin: 0;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }
  .field span {
    font-size: 10.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .field input {
    height: 46px;
    border-radius: 12px;
    border: 1px solid var(--line);
    background: var(--glass-2);
    color: var(--text);
    padding: 0 14px;
    font-size: 14px;
    font-family: var(--font-sans);
    outline: none;
    transition: border-color 0.25s var(--ease), background 0.25s var(--ease);
  }
  .field input:focus {
    border-color: rgba(var(--accent-rgb), 0.65);
    background: var(--glass);
  }
  .submit {
    height: 48px;
    border-radius: 12px;
    border: none;
    background: var(--text);
    color: var(--bg);
    font-size: 13.5px;
    font-weight: 550;
    font-family: var(--font-sans);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    cursor: pointer;
    margin-top: 6px;
    transition: all 0.3s var(--ease);
  }
  .submit:hover:not(:disabled) {
    transform: translateY(-1px);
    opacity: 0.92;
  }
  .submit:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .error {
    font-size: 12px;
    color: #fb7185;
    background: rgba(251, 113, 133, 0.1);
    border: 1px solid rgba(251, 113, 133, 0.25);
    padding: 9px 12px;
    border-radius: 10px;
  }
  .hint {
    font-size: 10.5px;
    color: var(--text-3);
    line-height: 1.8;
    text-align: center;
    margin-top: 4px;
  }
  code {
    font-family: ui-monospace, monospace;
    background: var(--glass-2);
    padding: 1px 5px;
    border-radius: 4px;
    color: var(--text-2);
  }
  .foot {
    position: absolute;
    bottom: 26px;
    font-size: 10px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  @media (max-width: 900px) {
    .login {
      grid-template-columns: 1fr;
    }
    .visual {
      display: none;
    }
    .form-side {
      min-height: 100dvh;
      border-left: none;
      padding: 24px;
    }
  }
</style>
