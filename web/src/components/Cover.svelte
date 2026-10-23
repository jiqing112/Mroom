<script>
  import { hashHue } from '../lib/colors.svelte.js'

  let {
    id = '',
    alt = '',
    class: cls = '',
    imgClass = '',
    rounded = 'rounded-2xl',
    eager = false,
    sizes = '',
    size = 0
  } = $props()

  let failed = $state(false)
  let loaded = $state(false)
  const hue = $derived(hashHue(id || alt || 'mrome'))
  // With a sizes hint the browser can pick a small, server-resized thumbnail
  // instead of downloading the full-size artwork for tiny rows.
  const src = $derived(!id ? '' : size > 0 ? `/api/cover/${id}/${size}` : `/api/cover/${id}`)
  const srcSet = $derived(
    id && !size && sizes
      ? [160, 320, 640, 1024].map((w) => `/api/cover/${id}/${w} ${w}w`).join(', ')
      : undefined
  )

  // The same Cover instance is reused when the player advances to another
  // track; reset loader state so an earlier error does not hide the new art.
  $effect(() => {
    id
    failed = false
    loaded = false
  })
</script>

<div class="cover {rounded} {cls}" style={`--h:${hue}`}>
  <div class="ph" aria-hidden="true"></div>
  {#if id && !failed}
    <img
      {src}
      srcset={srcSet}
      sizes={srcSet ? sizes : undefined}
      {alt}
      class="img {imgClass}"
      class:loaded
      loading={eager ? 'eager' : 'lazy'}
      decoding="async"
      onload={() => (loaded = true)}
      onerror={() => (failed = true)}
    />
  {/if}
</div>

<style>
  .cover {
    position: relative;
    overflow: hidden;
    background: hsl(var(--h) 20% 12%);
    isolation: isolate;
    transform: translateZ(0);
  }
  .ph {
    position: absolute;
    inset: 0;
    background:
      radial-gradient(120% 120% at 18% 12%, hsl(var(--h) 72% 58% / 0.9), transparent 58%),
      radial-gradient(120% 110% at 86% 84%, hsl(calc(var(--h) + 58) 66% 46% / 0.92), transparent 62%),
      linear-gradient(158deg, hsl(var(--h) 34% 24%), hsl(calc(var(--h) + 26) 30% 11%));
  }
  .ph::after {
    content: '';
    position: absolute;
    inset: 0;
    background:
      repeating-radial-gradient(circle at 50% 50%, transparent 0 12px, rgba(255, 255, 255, 0.035) 12px 13px),
      radial-gradient(70% 70% at 50% 45%, transparent 30%, rgba(0, 0, 0, 0.28));
  }
  .img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    opacity: 0;
    transition: opacity 0.55s var(--ease), transform 0.7s var(--ease);
  }
  .img.loaded {
    opacity: 1;
  }
</style>
