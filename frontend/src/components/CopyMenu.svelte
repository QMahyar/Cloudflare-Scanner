<script>
  import { _ } from 'svelte-i18n'
  import { copyMenu, closeCopyMenu } from '../lib/copymenu.js'
  import { copyWithPorts, setCopyMode } from '../lib/copymode.js'
  import { showToast } from '../lib/toast.js'

  function choose(withPorts) {
    setCopyMode(withPorts)
    closeCopyMenu()
    showToast(withPorts ? $_('copy.nowWithPort') : $_('copy.nowIpOnly'))
  }
  function onDocClick(e) {
    if (!e.target.closest('.copy-menu') && !e.target.closest('.split-caret')) closeCopyMenu()
  }
  function onKeydown(e) { if (e.key === 'Escape') closeCopyMenu() }
</script>

<svelte:window onclick={onDocClick} onkeydown={onKeydown} onresize={closeCopyMenu} />

<div
  class="copy-menu"
  class:open={$copyMenu.open}
  role="menu"
  aria-hidden={!$copyMenu.open}
  style="top:{$copyMenu.y}px; left:{$copyMenu.x}px"
>
  <button class="copy-menu-item" class:checked={$copyWithPorts} role="menuitemradio" aria-checked={$copyWithPorts} onclick={() => choose(true)}>
    <span class="copy-menu-check">✓</span><span>{$_('copy.withPort')}</span>
  </button>
  <button class="copy-menu-item" class:checked={!$copyWithPorts} role="menuitemradio" aria-checked={!$copyWithPorts} onclick={() => choose(false)}>
    <span class="copy-menu-check">✓</span><span>{$_('copy.ipOnly')}</span>
  </button>
</div>
