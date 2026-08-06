// Svelte 5 stores

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

const SETTINGS_KEY = 'cfscanner_settings'

function loadSettings() {
	if (typeof localStorage === 'undefined') return {}
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

export const settingsState = $state(loadSettings())
let saveTimer
export function persistSettings() {
    if (typeof localStorage === 'undefined') return
	clearTimeout(saveTimer)
	saveTimer = setTimeout(() => {
		try {
			localStorage.setItem(SETTINGS_KEY, JSON.stringify(settingsState))
		} catch {}
	}, 300)
}

export function getSetting(key, fallback) {
	const v = settingsState[key]
	return v === undefined ? fallback : v
}

export function setSetting(key, value) {
	settingsState[key] = value
    persistSettings()
}

// ─── Result stores (cfscanner_results) ───
const RESULTS_KEY = 'cfscanner_results'
const MAX_PERSISTED_ROWS = 10000

export const appState = $state({
    activeTab: 'endpoint',
    endpointScanning: false,
    cleanScanning: false,
    endpointRaw: [],
    cleanData: null,
    replacerGenerated: []
})

function loadResults() {
	if (typeof localStorage === 'undefined') return null
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
export function persistResults() {
	if (typeof localStorage === 'undefined') return
	// Skip mid-scan frames entirely
	if (appState.endpointScanning || appState.cleanScanning) return
	clearTimeout(persistTimer)
	persistTimer = setTimeout(() => {
		if (appState.endpointScanning || appState.cleanScanning) return
		try {
			const cd = appState.cleanData
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
					endpointRaw: capRows(appState.endpointRaw || []),
					cleanData: clean,
				}),
			)
		} catch {}
	}, 400)
}

export function initResults() {
    const saved = loadResults()
    if (saved) {
        if (Array.isArray(saved.endpointRaw) && saved.endpointRaw.length) {
            appState.endpointRaw = saved.endpointRaw
        }
        if (saved.cleanData) {
            appState.cleanData = saved.cleanData
        }
    }
}