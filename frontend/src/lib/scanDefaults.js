export const SCAN_DEPTHS = [
	{ v: '100', k: 'settings.depth.quick' },
	{ v: '500', k: 'settings.depth.normal' },
	{ v: '1000', k: 'settings.depth.deep' },
	{ v: '5000', k: 'settings.depth.insane' },
	{ v: '10000', k: 'settings.depth.massive' },
	{ v: '0', k: 'settings.depth.custom' },
]

export const HTTPS_PORTS = [443, 8443, 2053, 2083, 2087, 2096]
export const HTTP_PORTS = [80, 8080, 8880, 2052, 2082, 2086, 2095]

export const RANGE_PRESETS = [
	'104.16.0.0/13',
	'104.24.0.0/14',
	'172.64.0.0/13',
	'162.159.0.0/16',
	'188.114.96.0/20',
	'198.41.128.0/17',
	'2606:4700::/32',
]

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

export const ENDPOINT_CONCURRENCY_OPTIONS = [
	{ v: '0', label: 'Auto' },
	{ v: '25', label: '25' },
	{ v: '64', label: '64' },
	{ v: '128', label: '128' },
	{ v: '256', label: '256' },
	{ v: '512', label: '512' },
	{ v: '1024', label: '1024' },
]

export const ENDPOINT_ATTEMPTS_OPTIONS = ['1', '2', '3']

export const CLEAN_DEFAULTS = {
	useConfig: true,
	source: 'pool',
	customRanges: '',
	scanDepth: '500',
	customCount: '',
	ipVersion: '4',
	advOpen: false,

	phase1Probes: '100',
	phase2Probes: '10',

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

export const PHASE1_PROBES_OPTIONS = [
	{ v: '0', label: 'Auto' },
	{ v: '100', label: '100' },
	{ v: '250', label: '250' },
	{ v: '500', label: '500' },
	{ v: '1000', label: '1000' },
	{ v: '2000', label: '2000' },
]

export const PHASE2_PROBES_OPTIONS = [
	{ v: '8', label: '8' },
	{ v: '10', label: '10' },
	{ v: '16', label: '16' },
	{ v: '32', label: '32' },
	{ v: '48', label: '48' },
	{ v: '64', label: '64' },
	{ v: '128', label: '128' },
]

export const PHASE2_COUNT_OPTIONS = ['0', '10', '20', '30', '50', '100']
