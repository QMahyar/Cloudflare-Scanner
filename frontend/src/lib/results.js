// Merge live clean-scan snapshots without letting a short/transient response
// erase rows already shown. A new scan explicitly resets the store to null.
const LIST_FIELDS = ['entries', 'phase1_entries', 'phase2_entries', 'nearby_entries']

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
