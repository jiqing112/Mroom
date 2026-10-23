<script>
  import { toasts, dismiss } from '../lib/toast.svelte.js'
  import { fly } from 'svelte/transition'
</script>

<div class="toasts" aria-live="polite">
  {#each toasts.items as t (t.id)}
    <button
      class="toast glass-deep"
      class:ok={t.kind === 'ok'}
      class:err={t.kind === 'err'}
      onclick={() => dismiss(t.id)}
      transition:fly={{ y: 16, duration: 320 }}
    >
      <span class="dot"></span>
      <span>{t.message}</span>
    </button>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    left: 50%;
    bottom: 116px;
    transform: translateX(-50%);
    z-index: 80;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    pointer-events: none;
    width: max-content;
    max-width: min(92vw, 520px);
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 11px 16px;
    border-radius: 999px;
    color: var(--text);
    font-size: 13px;
    cursor: pointer;
    max-width: 100%;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    flex: none;
  }
  .ok .dot {
    background: #4ade80;
  }
  .err .dot {
    background: #fb7185;
  }
  @media (max-width: 860px) {
    .toasts {
      bottom: 150px;
    }
  }
</style>
