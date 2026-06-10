import { writable, get } from 'svelte/store'

// Which tab is visible. A store (not local App state) so any component can
// navigate — e.g. "Use" on an endpoint switches to the Replacer tab.
export const activeTab = writable('endpoint')

// ─── Settings persistence (cfscanner_settings) ───
// Same localStorage key and field-name-keyed shape as the original, so existing
// users keep their saved settings. Components read initial values via
// getSetting() and write back via setSetting() (debounced).
const SETTINGS_KEY = 'cfscanner_settings'
function loadSettings() {
  try { return JSON.parse(localStorage.getItem(SETTINGS_KEY) || 'null') || {} } catch { return {} }
}
export const settings = writable(loadSettings())

let saveTimer
settings.subscribe((v) => {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    try { localStorage.setItem(SETTINGS_KEY, JSON.stringify(v)) } catch {}
  }, 300)
})

export function getSetting(key, fallback) {
  const v = get(settings)[key]
  return v === undefined ? fallback : v
}
export function setSetting(key, value) {
  settings.update((s) => ({ ...s, [key]: value }))
}

// ─── Result stores (cfscanner_results) ───
// endpointRaw: successful WARP endpoint results. cleanData: the IP-scanner
// result payload. replacerGenerated: generated share URLs (not persisted).
export const endpointRaw = writable([])
export const cleanData = writable(null)
export const replacerGenerated = writable([])

const RESULTS_KEY = 'cfscanner_results'
export function loadResults() {
  try { return JSON.parse(localStorage.getItem(RESULTS_KEY) || 'null') } catch { return null }
}
function persistResults() {
  try {
    localStorage.setItem(RESULTS_KEY, JSON.stringify({
      endpointRaw: get(endpointRaw) || [],
      cleanData: get(cleanData) || null,
    }))
  } catch {}
}
let persisting = false
export function beginResultsPersistence() {
  if (persisting) return
  persisting = true
  endpointRaw.subscribe(persistResults)
  cleanData.subscribe(persistResults)
}
