import { writable } from 'svelte/store'

// Shared "Copy All ▾" popover position/state. One menu instance lives in App;
// every split button opens it anchored to its caret.
export const copyMenu = writable({ open: false, x: 0, y: 0 })

export function openCopyMenuAt(ev) {
  ev.stopPropagation()
  const r = ev.currentTarget.getBoundingClientRect()
  const w = 190
  const rtl = document.documentElement.dir === 'rtl'
  let left = rtl ? r.left : r.right - w
  left = Math.max(8, Math.min(left, window.innerWidth - w - 8))
  copyMenu.set({ open: true, x: left, y: r.bottom + 4 })
}

export function closeCopyMenu() {
  copyMenu.update((m) => (m.open ? { ...m, open: false } : m))
}
