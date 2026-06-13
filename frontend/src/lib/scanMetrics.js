import { parseLatency } from './sort.js'

// computeSummary reduces a finished scan into the chips ScanProgress renders:
// how many endpoints were found, how many were scanned, the best latency seen,
// wall-clock elapsed (s), and throughput (/s). Pure — each scanner passes its
// own entry list and scanned count, so the metric logic lives in one place
// instead of being re-derived per tab. `scanned` falls back to the entry count
// when the caller has no separate total.
export function computeSummary(entries, scanned, scanMs) {
  const list = entries || []
  let best = Infinity
  for (const e of list) {
    const v = parseLatency(e.latency)
    if (v < best) best = v
  }
  const secs = scanMs / 1000
  const total = scanned || list.length
  const rate = secs > 0.05 ? Math.round(total / secs) : 0
  return { found: list.length, scanned: total, elapsed: secs, rate, best: isFinite(best) ? best : null }
}
