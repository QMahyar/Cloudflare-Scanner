<script>
  // Labeled switch. Owns vertical rhythm so parents never need padding-top hacks.
  // `align` = 'start' | 'field' — field aligns the switch with a labeled text input.
  let {
    checked = $bindable(false),
    label = '',
    title = '',
    ariaLabel = '',
    align = 'start', // start | field
    disabled = false,
  } = $props()
</script>

<div class="toggle-wrap" class:toggle-align-field={align === 'field'}>
  <label class="toggle" {title} aria-label={ariaLabel || label}>
    <input type="checkbox" bind:checked {disabled} />
    <span class="slider"></span>
  </label>
  {#if label}
    <span class="toggle-label" {title}>{label}</span>
  {/if}
</div>
