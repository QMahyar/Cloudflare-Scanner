<script>
  import { toast } from '../lib/toast.js'

  // Mirror the original showToast(): fade in via .show on the next frame, fade
  // out after the store hides it, then drop from layout 300ms later (matching
  // the CSS transition) so the invisible toast never traps pointer events.
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
    <div class="toast" class:show={shown} class:error role="status" aria-live="polite">{msg}</div>
  </div>
{/if}
