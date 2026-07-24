<script>
  // Labeled control wrapper: <label for> + default snippet. Keeps label/control pairing consistent.
  let {
    id = '',
    label = '',
    title = '',
    class: className = '',
    children,
  } = $props()
</script>

<div class="field {className}">
  {#if label}
    <label for={id || undefined} {title}>{label}</label>
  {/if}
  {@render children?.()}
</div>
