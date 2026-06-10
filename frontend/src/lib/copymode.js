import { writable, get } from 'svelte/store'

// Whether bulk copy/download/QR include the :port. Persisted under the original
// localStorage key so existing users keep their preference.
const KEY = 'cfscanner_copyports'
export const copyWithPorts = writable(localStorage.getItem(KEY) !== '0')

export function setCopyMode(withPorts) {
  copyWithPorts.set(withPorts)
  localStorage.setItem(KEY, withPorts ? '1' : '0')
}

// stripPort removes the port from an endpoint, IPv6-aware:
//   162.159.1.1:443    -> 162.159.1.1
//   [2606:4700::1]:443 -> 2606:4700::1
//   2606:4700::1       -> 2606:4700::1  (bare IPv6, no port — kept whole)
export function stripPort(ep) {
  ep = (ep || '').trim()
  if (!ep) return ep
  if (ep[0] === '[') {
    const i = ep.indexOf(']')
    return i > 0 ? ep.slice(1, i) : ep
  }
  const c = ep.lastIndexOf(':')
  if (c < 0) return ep
  if (ep.indexOf(':') !== c) return ep // bare IPv6, no port
  return ep.slice(0, c)
}

// formatEps joins endpoints into newline-separated text honoring the copy mode.
export function formatEps(list) {
  const arr = (list || []).map((s) => (s || '').trim()).filter(Boolean)
  const withPorts = get(copyWithPorts)
  return (withPorts ? arr : arr.map(stripPort)).join('\n')
}
