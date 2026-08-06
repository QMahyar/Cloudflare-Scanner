import { writable, get } from 'svelte/store'

const KEY = 'cfscanner_copyports'
export const copyWithPorts = writable(localStorage.getItem(KEY) !== '0')

export function setCopyMode(withPorts) {
  copyWithPorts.set(withPorts)
  localStorage.setItem(KEY, withPorts ? '1' : '0')
}

function stripPort(ep) {
  ep = (ep || '').trim()
  if (!ep) return ep
  if (ep[0] === '[') {
    const i = ep.indexOf(']')
    return i > 0 ? ep.slice(1, i) : ep
  }
  const c = ep.lastIndexOf(':')
  if (c < 0) return ep
  if (ep.indexOf(':') !== c) return ep
  return ep.slice(0, c)
}

export function formatEps(list) {
  const arr = (list || []).map((s) => (s || '').trim()).filter(Boolean)
  const withPorts = get(copyWithPorts)
  return (withPorts ? arr : arr.map(stripPort)).join('\n')
}
