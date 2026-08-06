import { writable, get } from 'svelte/store'

// Which tab is visible. A store (not local App state) so any component can
// navigate — e.g. "Use" on an endpoint switches to the Replacer tab.
export const activeTab = writable('endpoint')

// ─── Settings persistence (cfscanner_settings) ───
// Components read initial values via getSetting() and write back via
// setSetting() (debounced). Missing keys fall through to the caller fallback
// (scanDefaults.js).
const SETTINGS_KEY = 'cfscanner_settings'

// The "aggressive defaults" redesign briefly persisted sub-second probe
// timeouts that made both scanners return empty/false-negative result sets.
// Migrate those known-bad saved values back to the working defaults so users
// who opened the app during that window recover without clearing storage.
const BROKEN_TIMEOUT_MIGRATIONS = {
	endpointTimeout: { bad: ['200'], good: '6000' },
	cleanTimeout: { bad: ['200'], good: '3000' },
	cleanPhase2Timeout: { bad: ['500', '200', '100'], good: '8000' },
}

/** @param {Record<string, unknown>} raw */
export function migrateBrokenTimeouts(raw) {
	const out = { ...raw }
	let changed = false
	for (const [key, { bad, good }] of Object.entries(BROKEN_TIMEOUT_MIGRATIONS)) {
		const v = out[key]
		if (v === undefined || v === null) continue
		if (bad.includes(String(v))) {
			out[key] = good
			changed = true
		}
	}
	return { settings: out, changed }
}

function loadSettings() {
	try {
		const raw = JSON.parse(localStorage.getItem(SETTINGS_KEY) || 'null') || {}
		const { settings: migrated, changed } = migrateBrokenTimeouts(raw)
		if (changed) {
			try {
				localStorage.setItem(SETTINGS_KEY, JSON.stringify(migrated))
			} catch {}
		}
		return migrated
	} catch {
		return {}
	}
}
export const settings = writable(loadSettings())

let saveTimer
settings.subscribe((v) => {
	clearTimeout(saveTimer)
	saveTimer = setTimeout(() => {
		try {
			localStorage.setItem(SETTINGS_KEY, JSON.stringify(v))
		} catch {}
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
export const endpointRaw = writable([])
export const cleanData = writable(null)
export const replacerGenerated = writable([])

const RESULTS_KEY = 'cfscanner_results'
// localStorage quota is ~5MB. Persist at most this many rows per list so huge
// scans can't blow the quota (the write is caught, so persistence would just
// silently stop working). The full set stays in memory; only the restored
// snapshot is capped.
const MAX_PERSISTED_ROWS = 10000

export function loadResults() {
	try {
		return JSON.parse(localStorage.getItem(RESULTS_KEY) || 'null')
	} catch {
		return null
	}
}

function capRows(rows) {
	return Array.isArray(rows) && rows.length > MAX_PERSISTED_ROWS ? rows.slice(0, MAX_PERSISTED_ROWS) : rows
}

let persistTimer
function persistResults() {
	// Skip mid-scan frames entirely: results churn every ~600ms while a scan
	// runs, so writing on every change re-stringifies the whole set (jank) and
	// can exceed the quota mid-scan. The scanning-flag subscriptions below
	// persist the settled snapshot once the scan finishes.
	if (get(endpointScanning) || get(cleanScanning)) return
	clearTimeout(persistTimer)
	persistTimer = setTimeout(() => {
		// Re-check: the write may have been scheduled right as a scan started.
		if (get(endpointScanning) || get(cleanScanning)) return
		try {
			const cd = get(cleanData)
			const clean = cd
				? {
					...cd,
					entries: capRows(cd.entries),
					phase1_entries: capRows(cd.phase1_entries),
					phase2_entries: capRows(cd.phase2_entries),
					nearby_entries: capRows(cd.nearby_entries),
				}
				: null
			localStorage.setItem(
				RESULTS_KEY,
				JSON.stringify({
					endpointRaw: capRows(get(endpointRaw) || []),
					cleanData: clean,
				}),
			)
		} catch {}
	}, 400)
}
let persisting = false
export function beginResultsPersistence() {
	if (persisting) return
	persisting = true
	endpointRaw.subscribe(persistResults)
	cleanData.subscribe(persistResults)
	// Persist the settled results once a running scan completes (the scanner
	// components flip these flags off in an $effect when status leaves 'running').
	endpointScanning.subscribe((running) => { if (!running) persistResults() })
	cleanScanning.subscribe((running) => { if (!running) persistResults() })
}

// ─── Live scan-running indicators ───
export const endpointScanning = writable(false)
export const cleanScanning = writable(false)
