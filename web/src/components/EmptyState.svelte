<script>
  let { icon: Icon, title = '这里还没有内容', desc = '', children } = $props()
</script>

<div class="empty">
  {#if Icon}
    <div class="mark"><Icon size={22} strokeWidth={1.4} /></div>
  {/if}
  <div class="title">{title}</div>
  {#if desc}<div class="desc">{desc}</div>{/if}
  {#if children}
    <div class="action">{@render children()}</div>
  {/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 56px 20px;
    border: 1px dashed var(--line-strong);
    border-radius: 24px;
    background: var(--glass);
  }
  .mark {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--glass-2);
    border: 1px solid var(--line);
    color: var(--text-2);
    margin-bottom: 14px;
  }
  .title {
    font-size: 14.5px;
    font-weight: 500;
  }
  .desc {
    margin-top: 6px;
    font-size: 12.5px;
    color: var(--text-3);
    max-width: 380px;
    line-height: 1.7;
  }
  .action {
    margin-top: 18px;
  }
</style>
