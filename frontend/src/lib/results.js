// Merge live clean-scan snapshots without letting a short/transient response
// erase rows already shown. A new scan explicitly resets the store to null.
//
// `entries` is deliberately NOT in the never-shrink list: the server defines
// it as the *current phase view* — all phase-1 hits while phase 1 runs, only
// xray-validated successes once phase 2 starts. Phase-2 successes are a subset
// of phase-1 hits, so applying the grow-only rule to `entries` froze the stale
// phase-1 list forever once phase 2 began (summary chips and the notification
// then reported unvalidated hits). The phase-specific lists below only ever
// grow, so they stay protected; `entries` always mirrors the latest frame.
const LIST_FIELDS = ['phase1_entries', 'phase2_entries', 'nearby_entries']

export function mergeCleanResults(previous, next) {
  if (!next) return previous || null
  if (!previous) return next

  const merged = { ...previous, ...next }
  for (const key of LIST_FIELDS) {
    const oldRows = previous[key] || []
    const newRows = next[key]
    if (newRows === undefined || newRows.length < oldRows.length) merged[key] = oldRows
  }
  return merged
}
