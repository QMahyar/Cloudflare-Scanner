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
