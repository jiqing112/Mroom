<script>
  let { value = 0, max = 100, onseek, onchange, class: cls = '', label = '进度' } = $props()

  const pct = $derived(max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0)

  function input(e) {
    onseek?.(Number(e.currentTarget.value))
  }
  function change(e) {
    onchange?.(Number(e.currentTarget.value))
  }
</script>

<input
  type="range"
  class="slider {cls}"
  min="0"
  max={max > 0 ? max : 1}
  step="0.01"
  {value}
  style={`--p:${pct}%`}
  aria-label={label}
  oninput={input}
  onchange={change}
/>
