<script>

  let {
    options = [],
    value = $bindable(),
    class: className = '',
    onchange = undefined,
  } = $props()

  function select(v) {
    value = v
    onchange?.(v)
  }
</script>

<div class={['input-method-bar', className]} role="group">
  {#each options as opt (opt.value)}
    <button
      type="button"
      class={{ active: value === opt.value }}
      title={opt.title || undefined}
      onclick={() => select(opt.value)}
    >{opt.label}{#if opt.suffix}{opt.suffix}{/if}</button>
  {/each}
</div>
