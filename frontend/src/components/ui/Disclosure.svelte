<script>

  let {
    open = $bindable(false),
    summary = '',
    class: className = 'adv-settings',
    children,
  } = $props()
</script>

<details class={[className]} bind:open>
  <summary>{summary}</summary>
  {@render children?.()}
</details>
