// onActivateKey — keydown handler that triggers `fn` on Enter/Space, for
// non-button elements (e.g. `role="button"` spans) that already have an
// onclick handler.
export function onActivateKey(fn) {
  return (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      fn(e)
    }
  }
}

// activateKey — Svelte action equivalent of onActivateKey. Prefer this over
// onActivateKey inside virtual-table row snippets: the action attaches the
// listener once on mount and updates the callback reference in place, so
// Svelte never patches the DOM attribute on re-renders caused by scroll.
//
// Usage:  use:activateKey={() => doSomething()}
export function activateKey(node, fn) {
  let current = fn
  function handler(e) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      current(e)
    }
  }
  node.addEventListener('keydown', handler)
  return {
    update(newFn) { current = newFn },
    destroy() { node.removeEventListener('keydown', handler) },
  }
}
