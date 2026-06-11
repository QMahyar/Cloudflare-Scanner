<script>
  import { get } from 'svelte/store'
  import { createVirtualizer } from '@tanstack/svelte-virtual'

  // Reusable virtualized table body. Keeps real <table> semantics (and the
  // shared .results-table CSS) by rendering only the visible window of rows
  // plus two spacer <tr>s whose heights stand in for the off-screen rows.
  // Rows self-measure (use:measure + data-index), so estimateSize only needs
  // to be roughly right — the virtualizer corrects total height as rows render.
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

  // The virtual window + reserved height, driven straight from the
  // virtualizer's emissions (see the subscription below). These are plain
  // $state — assigning fresh values is what guarantees a re-render. The
  // tempting `$derived(() => $virtualizer.getVirtualItems())` does NOT work
  // under Svelte 5 runes: the @tanstack/svelte-virtual wrapper always re-emits
  // the SAME mutated instance, which a reference-deduped store-rune signal
  // ignores, so scroll/resize updates silently never reach the DOM.
  let vItems = $state([])
  let totalSize = $state(0)

  const virtualizer = createVirtualizer({
    count: items.length,
    getScrollElement: () => scrollEl,
    estimateSize: () => estimateSize,
    overscan,
  })

  // Persistent subscription (defined first so it owns the store's lifetime).
  // Subscribing runs the wrapper's start fn — _didMount() plus the initial
  // setOptions() that attaches the scroll/resize observers to the bound
  // element — and fires on every emission (scroll, resize, measure, count
  // change). Each emission pushes the current window into $state.
  $effect(() => virtualizer.subscribe((v) => {
    vItems = v.getVirtualItems()
    totalSize = v.getTotalSize()
  }))

  // Keep options in sync with the reactive row count and the bound scroll
  // element. Runs after mount (so scrollEl is set) and on every count change.
  // get(virtualizer) reads the store value (the instance carrying setOptions);
  // the persistent subscription above keeps it from being torn down.
  $effect(() => {
    const el = scrollEl
    get(virtualizer).setOptions({
      count: items.length,
      getScrollElement: () => el,
      estimateSize: () => estimateSize,
      overscan,
    })
  })

  // measureElement reads data-index off the node and observes its real height.
  function measure(node) {
    get(virtualizer).measureElement(node)
    return {
      update() { get(virtualizer).measureElement(node) },
    }
  }

  const padTop = $derived(vItems.length ? vItems[0].start : 0)
  const padBottom = $derived(vItems.length ? totalSize - vItems[vItems.length - 1].end : 0)
</script>

<div bind:this={scrollEl} class="results-table-wrap virtual-scroll" style="max-height:{maxHeight}">
  <table class="results-table">
    <thead>{@render header()}</thead>
    <tbody>
      {#if padTop > 0}<tr aria-hidden="true" class="virtual-pad"><td {colspan} style="height:{padTop}px;padding:0;border:0"></td></tr>{/if}
      {#each vItems as vi (getKey(items[vi.index], vi.index))}
        {@render row(items[vi.index], vi.index, measure)}
      {/each}
      {#if padBottom > 0}<tr aria-hidden="true" class="virtual-pad"><td {colspan} style="height:{padBottom}px;padding:0;border:0"></td></tr>{/if}
    </tbody>
  </table>
</div>
