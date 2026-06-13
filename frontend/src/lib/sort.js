// parseLatency turns a Go-formatted duration string ("12ms", "1.2s", "800µs")
// into milliseconds for comparison. Infinity sorts unparseable values last.
export function parseLatency(s) {
  if (!s) return Infinity
  const m = String(s).match(/^([\d.]+)\s*(ms|s|µs)?$/)
  if (!m) return parseFloat(s) || Infinity
  const v = parseFloat(m[1])
  const u = m[2] || 'ms'
  return u === 's' ? v * 1000 : u === 'µs' ? v / 1000 : v
}

// sortEntries returns a new sorted array. field 'num' preserves insertion order
// (asc) or reverses it (desc) without an O(n^2) index lookup.
export function sortEntries(entries, field, dir) {
  if (field === 'num') {
    const out = entries.slice()
    if (dir === 'desc') out.reverse()
    return out
  }
  const sorted = [...entries]
  sorted.sort((a, b) => {
    let va, vb
    if (field === 'latency') { va = parseLatency(a.latency); vb = parseLatency(b.latency) }
    else if (field === 'endpoint' || field === 'address') { va = (a.endpoint || a.address || '').toLowerCase(); vb = (b.endpoint || b.address || '').toLowerCase() }
    else if (field === 'port') { va = a.port; vb = b.port }
    else if (field === 'remark') { va = (a.remark || '').toLowerCase(); vb = (b.remark || '').toLowerCase() }
    else if (field === 'protocol') { va = (a.protocol || '').toLowerCase(); vb = (b.protocol || '').toLowerCase() }
    else { va = a[field]; vb = b[field] }
    if (va < vb) return dir === 'asc' ? -1 : 1
    if (va > vb) return dir === 'asc' ? 1 : -1
    return 0
  })
  return sorted
}

// latClass maps a latency to the fast/medium/slow color class.
export function latClass(ms) {
  const v = parseLatency(ms)
  if (v === Infinity) return ''
  return v < 100 ? 'latency-fast' : v < 200 ? 'latency-medium' : 'latency-slow'
}

// latBar maps a latency to a 0–100 width for the in-cell latency meter. A sqrt
// curve over a fixed ~1200ms reference keeps the busy sub-200ms range visually
// distinct while staying stable — bars don't reflow as new results stream in.
export function latBar(ms) {
  const v = parseLatency(ms)
  if (!isFinite(v) || v <= 0) return 0
  return Math.max(6, Math.min(100, Math.round(Math.sqrt(v / 1200) * 100)))
}

// toggleSort returns the next {field, dir} given a clicked column.
export function toggleSort(cur, field) {
  if (cur.field === field) return { field, dir: cur.dir === 'asc' ? 'desc' : 'asc' }
  return { field, dir: 'asc' }
}
