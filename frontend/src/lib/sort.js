export function parseLatency(s) {
  if (!s) return Infinity
  const m = String(s).match(/^([\d.]+)\s*(ms|s|µs)?$/)
  if (!m) return parseFloat(s) || Infinity
  const v = parseFloat(m[1])
  const u = m[2] || 'ms'
  return u === 's' ? v * 1000 : u === 'µs' ? v / 1000 : v
}

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

    else if (field === 'score') { va = a.score ?? -1; vb = b.score ?? -1 }
    else if (field === 'loss') { va = a.loss ?? Infinity; vb = b.loss ?? Infinity }
    else { va = a[field]; vb = b[field] }
    if (va < vb) return dir === 'asc' ? -1 : 1
    if (va > vb) return dir === 'asc' ? 1 : -1
    return 0
  })
  return sorted
}

export function latClass(ms) {
  const v = parseLatency(ms)
  if (v === Infinity) return ''
  return v < 200 ? 'latency-fast' : v < 450 ? 'latency-medium' : 'latency-slow'
}

export function latBar(ms) {
  const v = parseLatency(ms)
  if (!isFinite(v) || v <= 0) return 0
  return Math.max(6, Math.min(100, Math.round(Math.sqrt(v / 1500) * 100)))
}

export function toggleSort(cur, field) {
  if (cur.field === field) return { field, dir: cur.dir === 'asc' ? 'desc' : 'asc' }
  return { field, dir: field === 'score' ? 'desc' : 'asc' }
}

export function scoreClass(s) {
  const v = Number(s)
  if (!isFinite(v) || v <= 0) return ''
  return v >= 75 ? 'latency-fast' : v >= 50 ? 'latency-medium' : 'latency-slow'
}

export function scoreBar(s) {
  const v = Number(s)
  if (!isFinite(v) || v <= 0) return 0
  return Math.max(2, Math.min(100, Math.round(v)))
}

export function fmtLoss(loss) {
  const v = Number(loss)
  if (!isFinite(v)) return '—'
  return Math.round(v) + '%'
}

export function lossClass(loss) {
  const v = Number(loss)
  if (!isFinite(v)) return ''
  return v <= 0 ? 'latency-fast' : v < 25 ? 'latency-medium' : 'latency-slow'
}
