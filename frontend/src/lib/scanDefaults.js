// Single source of truth for scanner presets and default field values.
// Keep these aligned with backend defaults in cleanip.go / scanner.go /
// cleanscan_handlers.go / scan_handlers.go (and the i18n "default N" strings).

/** @type {{ v: string, k: string }[]} */
export const SCAN_DEPTHS = [
	{ v: '100', k: 'settings.depth.quick' },
	{ v: '500', k: 'settings.depth.normal' },
	{ v: '1000', k: 'settings.depth.deep' },
	{ v: '5000', k: 'settings.depth.insane' },
	{ v: '10000', k: 'settings.depth.massive' },
	{ v: '0', k: 'settings.depth.custom' },
]

// Official Cloudflare CDN ports (cloudflare.com/ips + support docs).
export const HTTPS_PORTS = [443, 8443, 2053, 2083, 2087, 2096]
export const HTTP_PORTS = [80, 8080, 8880, 2052, 2082, 2086, 2095]

// Common high-density CF ranges for the custom-range chips (not the full pool).
export const RANGE_PRESETS = [
	'104.16.0.0/13',
	'104.24.0.0/14',
	'172.64.0.0/13',
	'162.159.0.0/16',
	'188.114.96.0/20',
	'198.41.128.0/17',
	'2606:4700::/32',
]

// ─── Endpoint Scanner (WARP) ───────────────────────────────────────────────
// Native handshake default path: concurrency 256 (or modest UI preset), timeout
// 6s (scanner.go). Noise mode still uses DefaultConcurrency when concurrency is
// left at 0 (auto). Sub-second timeouts false-negative both native retransmits
// and the xray noise path (SOCKS + 204 through WireGuard).
export const ENDPOINT_DEFAULTS = {
	useConfig: true,
	scanDepth: '500',
	customCount: '',
	ipVersion: '4',
	advOpen: false,
	noise: false,
	timeoutMs: '6000',
	concurrency: '25',
	attempts: '2',
	stopAfter: '0',
	notify: false,
	outCount: '0',
	maxLatency: '0',
}

/** Concurrent workers: 0 = auto. Noise path is process-heavy; keep options modest. */
export const ENDPOINT_CONCURRENCY_OPTIONS = [
	{ v: '0', label: 'Auto' },
	{ v: '25', label: '25' },
	{ v: '64', label: '64' },
	{ v: '128', label: '128' },
	{ v: '256', label: '256' },
	{ v: '512', label: '512' },
	{ v: '1024', label: '1024' },
]

/** Per-endpoint handshake attempts (server clamps 1–5). */
export const ENDPOINT_ATTEMPTS_OPTIONS = ['1', '2', '3']

// ─── IP Scanner (clean Cloudflare IPs) ─────────────────────────────────────
// Moderate P1 concurrency, validate all P1 winners (phase2Count 0). Timeouts
// match cleanip.go: 3s Phase-1 dial, 8s Phase-2 xray validation. Advanced
// fields remain editable.
export const CLEAN_DEFAULTS = {
	useConfig: true,
	source: 'pool',
	customRanges: '',
	scanDepth: '500',
	customCount: '',
	ipVersion: '4',
	advOpen: false,
	// 0 = server chooses (NumCPU-aware, lower on Windows).
	phase1Probes: '100',
	phase2Probes: '10',
	// 0 = validate every Phase-1 success (backend treats 0 as unlimited).
	phase2Count: '0',
	ports: [443],
	nearby: false,
	timeout1: '3000',
	timeout2: '8000',
	stopAfter: '0',
	notify: false,
	outCount: '0',
	maxLatency: '0',
}

/** Phase-1 TCP concurrency. 0 = auto (server-side). */
export const PHASE1_PROBES_OPTIONS = [
	{ v: '0', label: 'Auto' },
	{ v: '100', label: '100' },
	{ v: '250', label: '250' },
	{ v: '500', label: '500' },
	{ v: '1000', label: '1000' },
	{ v: '2000', label: '2000' },
]

/**
 * Phase-2 concurrent endpoint slots. Mapped to batches of 16, so 16 = one xray
 * process, 32 = two, etc.
 */
export const PHASE2_PROBES_OPTIONS = [
	{ v: '8', label: '8' },
	{ v: '10', label: '10' },
	{ v: '16', label: '16' },
	{ v: '32', label: '32' },
	{ v: '48', label: '48' },
	{ v: '64', label: '64' },
	{ v: '128', label: '128' },
]

/** How many Phase-1 winners to validate through xray. 0 = all. */
export const PHASE2_COUNT_OPTIONS = ['0', '10', '20', '30', '50', '100']
