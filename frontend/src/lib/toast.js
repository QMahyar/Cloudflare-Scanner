import { writable } from 'svelte/store'

export const toast = writable({ msg: '', error: false, visible: false })

let timer
export function showToast(msg, isError = false) {
  toast.set({ msg, error: !!isError, visible: true })
  clearTimeout(timer)
  timer = setTimeout(() => {
    toast.update((t) => ({ ...t, visible: false }))
  }, 2200)
}
