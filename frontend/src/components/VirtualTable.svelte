<script>
  import { untrack } from 'svelte'
  import { get } from 'svelte/store'
  import { createVirtualizer } from '@tanstack/svelte-virtual'

  let {
    items,
    getKey = (_item, i) => i,
    estimateSize = 41,
    maxHeight = '70vh',
    overscan = 12,
    colspan = 1,
    header,
    row,
  } = $props()

  let scrollEl = $state(null)

  let vItems = $state.raw([])
  let totalSize = $state(0)

  const virtualizer = createVirtualizer({
    count: untrack(() => items.length),
    getScrollElement: () => scrollEl,
    estimateSize: () => estimateSize,
    overscan: untrack(() => overscan),
  })

  $effect(() => virtualizer.subscribe((v) => {
    vItems = v.getVirtualItems()
    totalSize = v.getTotalSize()
  }))

  $effect(() => {
    const el = scrollEl
    get(virtualizer).setOptions({
      count: items.length,
      getScrollElement: () => el,
      estimateSize: () => estimateSize,
      overscan,
    })
  })

  function measure(node) {
    get(virtualizer).measureElement(node)
    return () => get(virtualizer).measureElement(null)
  }

  const padTop = $derived(vItems.length ? vItems[0].start : 0)
  const padBottom = $derived(vItems.length ? totalSize - vItems[vItems.length - 1].end : 0)
</script>

<div bind:this={scrollEl} class="results-table-wrap virtual-scroll" style:max-height={maxHeight}>
  <table class="results-table">
    <thead>{@render header()}</thead>
    <tbody>
      {#if padTop > 0}<tr aria-hidden="true" class="virtual-pad"><td {colspan} style:height="{padTop}px" style:padding="0" style:border="0"></td></tr>{/if}
      {#each vItems as vi (getKey(items[vi.index], vi.index))}
        {@render row(items[vi.index], vi.index, measure)}
      {/each}
      {#if padBottom > 0}<tr aria-hidden="true" class="virtual-pad"><td {colspan} style:height="{padBottom}px" style:padding="0" style:border="0"></td></tr>{/if}
    </tbody>
  </table>
</div>
