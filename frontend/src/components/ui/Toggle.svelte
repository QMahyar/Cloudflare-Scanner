<script>

  let {
    checked = $bindable(false),
    label = '',
    title = '',
    ariaLabel = '',
    align = 'start',
    disabled = false,
  } = $props()
</script>

<div class={['toggle-wrap', { 'toggle-align-field': align === 'field' }]}>
  <label class="toggle" {title} aria-label={ariaLabel || label}>
    <input type="checkbox" bind:checked {disabled} />
    <span class="slider"></span>
  </label>
  {#if label}
    <span class="toggle-label" {title}>{label}</span>
  {/if}
</div>
