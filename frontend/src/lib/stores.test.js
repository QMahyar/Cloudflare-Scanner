import { describe, expect, it } from 'vitest'
import { migrateBrokenTimeouts } from './stores.js'

describe('migrateBrokenTimeouts', () => {
	it('rewrites the aggressive-defaults timeout values that broke scans', () => {
		const { settings, changed } = migrateBrokenTimeouts({
			endpointTimeout: '200',
			cleanTimeout: '200',
			cleanPhase2Timeout: '500',
			phase1Probes: '100',
		})
		expect(changed).toBe(true)
		expect(settings.endpointTimeout).toBe('6000')
		expect(settings.cleanTimeout).toBe('3000')
		expect(settings.cleanPhase2Timeout).toBe('8000')
		expect(settings.phase1Probes).toBe('100')
	})

	it('leaves intentional custom timeouts alone', () => {
		const raw = {
			endpointTimeout: '1500',
			cleanTimeout: '1200',
			cleanPhase2Timeout: '10000',
		}
		const { settings, changed } = migrateBrokenTimeouts(raw)
		expect(changed).toBe(false)
		expect(settings).toEqual(raw)
	})

	it('accepts numeric bad values from older persistence', () => {
		const { settings, changed } = migrateBrokenTimeouts({ cleanPhase2Timeout: 500 })
		expect(changed).toBe(true)
		expect(settings.cleanPhase2Timeout).toBe('8000')
	})
})
