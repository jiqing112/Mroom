<script>
  import { Star } from 'lucide-svelte'

  let { value = 0, onrate = null, size = 14, label = '评分' } = $props()
  let hover = $state(0)
  const shown = $derived(hover || value || 0)

  function pick(n, e) {
    e?.stopPropagation()
    if (!onrate) return
    onrate(n === value ? 0 : n)
  }
</script>

<div class="rating" role="group" aria-label={label}>
  {#each [1, 2, 3, 4, 5] as n}
    <button
      type="button"
      class="star-btn"
      class:on={n <= shown}
      onmouseenter={() => (hover = n)}
      onmouseleave={() => (hover = 0)}
      onclick={(e) => pick(n, e)}
      aria-label={`${n} 星`}
      aria-pressed={n <= value}
    >
      <Star size={size} strokeWidth={1.5} fill={n <= shown ? 'currentColor' : 'none'} />
    </button>
  {/each}
</div>

<style>
  .rating {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    line-height: 1;
  }
  .star-btn {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    background: none;
    color: var(--text-3);
    cursor: pointer;
    transition: color 0.15s var(--ease), transform 0.15s var(--ease);
  }
  .star-btn:hover {
    transform: scale(1.12);
  }
  .star-btn.on {
    color: var(--accent);
  }
</style>
