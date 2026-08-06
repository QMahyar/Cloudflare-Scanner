<script>
  import { _ } from 'svelte-i18n'

  let { dense = false } = $props()
</script>

{#snippet body()}
  <p class="desc help-intro">{$_('cleanHelp.intro')}</p>
  <div class="help-list">
    <div>{@html $_('cleanHelp.p1')}</div>
    <div>{@html $_('cleanHelp.p2')}</div>
    <div>{@html $_('cleanHelp.p3')}</div>
    <div>{@html $_('cleanHelp.p4')}</div>
    <div>{@html $_('cleanHelp.p5')}</div>
  </div>
  <p class="desc">{$_('cleanHelp.tip')}</p>
{/snippet}

{#if dense}
  <div class="help-panel help-panel-dense">
    {@render body()}
  </div>
{:else}
  <details class="help-panel">
    <summary>{$_('cleanHelp.header')}</summary>
    {@render body()}
  </details>
{/if}
