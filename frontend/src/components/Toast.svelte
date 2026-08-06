<script>
  import { toast } from '../lib/toast.js'

  let display = $state(false)
  let shown = $state(false)
  let msg = $state('')
  let error = $state(false)

  let hideTimer

  $effect(() => {
    const t = $toast
    msg = t.msg
    error = t.error
    if (t.visible) {
      clearTimeout(hideTimer)
      display = true
      requestAnimationFrame(() => { shown = true })
    } else {
      shown = false
      clearTimeout(hideTimer)
      hideTimer = setTimeout(() => { display = false }, 300)
    }
  })
</script>

{#if display}
  <div class="toast-container">
    <div class={['toast', { show: shown, error }]} role="status" aria-live="polite">{msg}</div>
  </div>
{/if}
