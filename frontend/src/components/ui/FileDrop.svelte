<script>
  // File picker with label surface. Hides the native input; shows selected name.
  let {
    id = '',
    accept = '',
    multiple = false,
    disabled = false,
    files = $bindable(null),
    label = '',
    selectedLabel = '',
    title = '',
    onchange = undefined,
    icon = undefined,
  } = $props()

  const hasFile = $derived(!!(files && files.length))
  const display = $derived(
    hasFile
      ? (selectedLabel || (multiple ? String(files.length) : files[0].name))
      : label,
  )

  function handleChange(e) {
    files = e.currentTarget.files
    onchange?.(e)
    // Reset the input value so picking the SAME file again still fires change
    // (browsers skip change when the input still holds the previous selection).
    // `files` above already captured the FileList, so clearing is safe.
    e.currentTarget.value = ''
  }
</script>

<label class="file-input-wrap" for={id} {title}>
  <input
    type="file"
    {id}
    {accept}
    {multiple}
    {disabled}
    onchange={handleChange}
  />
  <div class={['file-label', { selected: hasFile }]} {title}>
    {#if icon}{@render icon()}{/if}
    <span>{display}</span>
  </div>
</label>
