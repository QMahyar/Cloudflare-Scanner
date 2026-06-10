<script>
  import { _ } from 'svelte-i18n'
  import { qrText, closeQR } from '../lib/modal.js'
  import { makeQRDataURL } from '../lib/qr.js'

  let dataUrl = $state('')

  // Regenerate whenever a new payload opens the overlay.
  $effect(() => {
    const text = $qrText
    if (text === null) { dataUrl = ''; return }
    makeQRDataURL(text).then((url) => { dataUrl = url })
  })

  function onOverlayClick(e) {
    if (e.target === e.currentTarget) closeQR()
  }
  function onKeydown(e) {
    if (e.key === 'Escape' && $qrText !== null) closeQR()
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="qr-overlay" class:open={$qrText !== null} role="presentation" onclick={onOverlayClick}>
  <div class="qr-modal" role="dialog" aria-modal="true" aria-label="QR Code">
    <h3>{$_('qr.title')}</h3>
    <div id="qrCode">
      {#if dataUrl}
        <img src={dataUrl} alt="QR code" width="220" height="220" />
      {:else if $qrText !== null}
        <textarea class="apply-config-content-box" readonly style="min-height:160px">{$qrText}</textarea>
      {/if}
    </div>
    <p class="mono-break">{$qrText ?? ''}</p>
    <button class="btn btn-secondary btn-sm qr-close" onclick={closeQR}>{$_('buttons.close')}</button>
  </div>
</div>
