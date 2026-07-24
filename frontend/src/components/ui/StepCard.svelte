<script>
  // Card with optional numbered step heading, count chip, and trailing header slot.
  let {
    step = null, // number | null
    title = '',
    count = null, // number | null — shown as count-chip when non-null
    id = undefined,
    children,
    header = undefined, // optional trailing content (filters, etc.)
  } = $props()
</script>

<div class="card" {id}>
  {#if title || step != null}
    <div class="section-header">
      <h2>
        {#if step != null}<span class="step-num">{step}</span>{/if}
        {#if title}<span>{title}</span>{/if}
        {#if count != null}<span class="count-chip">{count}</span>{/if}
      </h2>
      {#if header}
        <div class="section-header-actions">
          {@render header()}
        </div>
      {/if}
    </div>
  {/if}
  {@render children?.()}
</div>
