import { describe, expect, it } from 'vitest'
import { parseLatency, sortEntries, toggleSort } from './sort.js'
import { computeSummary } from './scanMetrics.js'

describe('result helpers', () => {
  it('normalizes Go durations', () => {
    expect(parseLatency('1.2s')).toBe(1200)
    expect(parseLatency('800µs')).toBeCloseTo(0.8)
    expect(parseLatency('12ms')).toBe(12)
  })

  it('ranks unmeasured quality last', () => {
    const rows = [{ endpoint: 'none' }, { endpoint: 'fast', score: 90 }, { endpoint: 'ok', score: 50 }]
    expect(sortEntries(rows, 'score', 'desc').map((r) => r.endpoint)).toEqual(['fast', 'ok', 'none'])
  })

  it('computes a finished scan summary', () => {
    expect(computeSummary([{ latency: '20ms' }, { latency: '10ms' }], 100, 2000)).toEqual({
      found: 2, scanned: 100, elapsed: 2, rate: 50, best: 10,
    })
  })

  it('defaults score sorting to descending', () => {
    expect(toggleSort({ field: 'latency', dir: 'asc' }, 'score')).toEqual({ field: 'score', dir: 'desc' })
  })
})
