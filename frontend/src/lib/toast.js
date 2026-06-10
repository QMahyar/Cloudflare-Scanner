import { writable } from 'svelte/store'

// Single shared toast, mirroring the original showToast(): one message at a
// time, auto-hiding after 2200ms, with an error variant.
export const toast = writable({ msg: '', error: false, visible: false })

let timer
export function showToast(msg, isError = false) {
  toast.set({ msg, error: !!isError, visible: true })
  clearTimeout(timer)
  timer = setTimeout(() => {
    toast.update((t) => ({ ...t, visible: false }))
  }, 2200)
}
