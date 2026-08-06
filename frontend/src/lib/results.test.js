import { describe, expect, it } from 'vitest'
import { mergeCleanResults } from './results.js'

describe('mergeCleanResults', () => {
  it('keeps phase one rows when phase two starts', () => {
    const first = { phase: 'phase1', entries: [{ endpoint: 'a' }], phase1_entries: [{ endpoint: 'a' }] }
    const second = { phase: 'phase2', entries: [{ endpoint: 'b' }], phase1_entries: [], phase2_entries: [{ endpoint: 'b' }] }
    const got = mergeCleanResults(first, second)
    expect(got.phase1_entries).toEqual(first.phase1_entries)
    expect(got.phase2_entries).toEqual(second.phase2_entries)
  })

  it('does not regress on partial snapshots', () => {
    const old = { entries: [{ endpoint: 'a' }, { endpoint: 'b' }] }
    expect(mergeCleanResults(old, { status: 'running' }).entries).toHaveLength(2)
  })

  it('reflects the current phase view in entries (never freezes the phase-1 list)', () => {
    // Phase 2 successes are always a subset of phase-1 hits, so a grow-only
    // rule on `entries` permanently froze the stale phase-1 list once phase 2
    // began — summary.found / the completion notification then reported
    // unvalidated hits and data.entries contradicted phase2_entries.
    const phase1 = {
      phase: 'phase1',
      entries: [{ endpoint: 'a' }, { endpoint: 'b' }, { endpoint: 'c' }],
      phase1_entries: [{ endpoint: 'a' }, { endpoint: 'b' }, { endpoint: 'c' }],
      phase2_entries: [],
    }
    const phase2 = {
      phase: 'phase2',
      entries: [{ endpoint: 'a' }, { endpoint: 'b' }],
      phase1_entries: [{ endpoint: 'a' }, { endpoint: 'b' }, { endpoint: 'c' }],
      phase2_entries: [{ endpoint: 'a' }, { endpoint: 'b' }],
    }
    const got = mergeCleanResults(phase1, phase2)
    expect(got.entries).toHaveLength(2) // phase-2 validated successes
    expect(got.phase1_entries).toHaveLength(3) // phase-specific lists still grow-only
    expect(got.phase2_entries).toHaveLength(2)
  })
})
