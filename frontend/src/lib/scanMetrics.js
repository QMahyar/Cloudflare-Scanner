import { parseLatency } from './sort.js'

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
