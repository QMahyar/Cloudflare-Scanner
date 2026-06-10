<script>
  import { _ } from 'svelte-i18n'
  import { openCopyMenuAt } from '../lib/copymenu.js'

  // oncopy: called when the main button is clicked. title: hover text.
  let { oncopy, title = '' } = $props()
</script>

<span class="split-btn">
  <button class="btn btn-secondary btn-sm split-main" onclick={oncopy} {title}>{$_('results.copyAll')}</button>
  <button class="btn btn-secondary btn-sm split-caret" onclick={openCopyMenuAt} title={$_('copy.menuTitle')} aria-label={$_('copy.menuTitle')}>▾</button>
</span>
