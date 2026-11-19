<script>
  import { ArrowRight, Loader2, Sun, Moon } from 'lucide-svelte'
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
  <button class="theme-btn icon-btn" onclick={toggleTheme} aria-label="切换主题">
    {#if theme.mode === 'dark'}<Sun size={16} strokeWidth={1.6} />{:else}<Moon size={16} strokeWidth={1.6} />{/if}
  </button>

  <form class="card glass" onsubmit={submit}>
    <div class="brand">
      <div class="mark"><span></span></div>
      <span class="wordmark">MROME</span>
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
</div>

<style>
  .login {
    position: relative;
    z-index: 1;
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: 24px;
  }
  .theme-btn {
    position: absolute;
    top: 26px;
    right: 26px;
  }
  .card {
    width: min(390px, 100%);
    border-radius: var(--r-xl);
    padding: 34px 30px 28px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    animation: rise 0.7s var(--ease) both;
  }
  .brand {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    margin-bottom: 8px;
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
    border-radius: var(--r-md);
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
    border-radius: var(--r-md);
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
    color: var(--danger);
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 25%, transparent);
    padding: 9px 12px;
    border-radius: var(--r-sm);
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
</style>
