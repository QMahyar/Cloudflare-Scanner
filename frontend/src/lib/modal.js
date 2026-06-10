import { writable } from 'svelte/store'

// qrText holds the currently displayed QR payload, or null when the overlay is
// closed. QrModal.svelte renders the overlay + generates the code reactively.
export const qrText = writable(null)

export function showQR(text) {
  qrText.set(text)
}

export function closeQR() {
  qrText.set(null)
}
