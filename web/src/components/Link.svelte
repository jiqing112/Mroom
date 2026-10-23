<script>
  import { navigate } from '../lib/router.svelte.js'

  let { href = '/', class: cls = '', children, onclick, ...rest } = $props()

  function go(e) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return
    e.preventDefault()
    onclick?.(e)
    navigate(href)
  }
</script>

<a {href} class={cls} onclick={go} {...rest}>
  {@render children?.()}
</a>
