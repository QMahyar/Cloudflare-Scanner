import { describe, it, expect } from 'vitest'
import {
	SCAN_DEPTHS,
	HTTPS_PORTS,
	HTTP_PORTS,
	ENDPOINT_DEFAULTS,
	CLEAN_DEFAULTS,
	PHASE1_PROBES_OPTIONS,
	PHASE2_PROBES_OPTIONS,
	PHASE2_COUNT_OPTIONS,
	ENDPOINT_CONCURRENCY_OPTIONS,
	ENDPOINT_ATTEMPTS_OPTIONS,
} from './scanDefaults.js'

describe('scanDefaults', () => {
	it('exposes a complete depth ladder including custom', () => {
		expect(SCAN_DEPTHS.map((d) => d.v)).toEqual(['100', '500', '1000', '5000', '10000', '0'])
	})

	it('lists every official CF CDN port without duplicates', () => {
		const all = [...HTTPS_PORTS, ...HTTP_PORTS]
		expect(new Set(all).size).toBe(all.length)
		expect(HTTPS_PORTS).toContain(443)
		expect(HTTP_PORTS).toContain(80)
		expect(all).toHaveLength(13)
	})

	it('aligns endpoint defaults with scanner.go / scan_handlers.go', () => {
		expect(ENDPOINT_DEFAULTS.timeoutMs).toBe('200')
		expect(ENDPOINT_DEFAULTS.concurrency).toBe('25')
		expect(ENDPOINT_DEFAULTS.attempts).toBe('1')
		expect(ENDPOINT_DEFAULTS.scanDepth).toBe('500')
		expect(ENDPOINT_DEFAULTS.noise).toBe(false)
	})

	it('aligns clean defaults with cleanip.go', () => {
		expect(CLEAN_DEFAULTS.timeout1).toBe('200')
		expect(CLEAN_DEFAULTS.timeout2).toBe('500')
		expect(CLEAN_DEFAULTS.phase1Probes).toBe('100')
		expect(CLEAN_DEFAULTS.phase2Probes).toBe('10')
		expect(CLEAN_DEFAULTS.phase2Count).toBe('0')
		expect(CLEAN_DEFAULTS.ports).toEqual([443])
		expect(CLEAN_DEFAULTS.scanDepth).toBe('500')
	})

	it('offers Auto for phase-1 and full-batch steps for phase-2', () => {
		expect(PHASE1_PROBES_OPTIONS[0].v).toBe('0')
		expect(PHASE1_PROBES_OPTIONS.map((o) => o.v)).toContain('100')
		expect(PHASE2_PROBES_OPTIONS.map((o) => o.v)).toEqual(['8', '10', '16', '32', '48', '64', '128'])
		expect(PHASE2_COUNT_OPTIONS[0]).toBe('0')
		expect(PHASE2_COUNT_OPTIONS).toContain('100')
		expect(ENDPOINT_CONCURRENCY_OPTIONS[0].v).toBe('0')
		expect(ENDPOINT_CONCURRENCY_OPTIONS.map((o) => o.v)).toContain('25')
		expect(ENDPOINT_ATTEMPTS_OPTIONS).toEqual(['1', '2', '3'])
	})
})
