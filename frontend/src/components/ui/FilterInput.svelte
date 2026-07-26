<script>
  // Compact labeled filter control for results header rows.
  let {
    id = '',
    label = '',
    title = '',
    value = $bindable(''),
    placeholder = '',
    inputmode = undefined,
    width = 'narrow', // narrow | colo | auto
  } = $props()
</script>

<div class="compact-control">
  {#if label}
    <label for={id || undefined} {title}>{label}</label>
  {/if}
  <input
    class={[
      'compact-input',
      {
        'compact-input-colo': width === 'colo',
        'compact-input-auto': width === 'auto',
      },
    ]}
    {id}
    type="text"
    bind:value
    {placeholder}
    {title}
    {inputmode}
  />
</div>
