import { writable } from 'svelte/store'

export const qrText = writable(null)

export function showQR(text) {
  qrText.set(text)
}

export function closeQR() {
  qrText.set(null)
}
