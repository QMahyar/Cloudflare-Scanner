import { downloadText } from './clipboard.js'

const COLUMNS = [
  ['endpoint', 'endpoint'],
  ['score', 'score'],
  ['latency', 'latency'],
  ['best', 'best'],
  ['jitter', 'jitter'],
  ['loss', 'loss_pct'],
  ['h3', 'quic'],
  ['colo', 'colo'],
  ['loc', 'location'],
]

function csvCell(v) {
  if (v === undefined || v === null) return ''
  const s = typeof v === 'boolean' ? (v ? 'yes' : 'no') : String(v)
  return /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
}

export function exportCSV(filename, entries) {
  const rows = entries || []
  const header = COLUMNS.map(([, label]) => label).join(',')
  const body = rows.map((e) => COLUMNS.map(([key]) => csvCell(e[key])).join(',')).join('\n')
  downloadText(filename, header + '\n' + body + '\n', 'text/csv')
}

export function exportJSON(filename, entries) {
  downloadText(filename, JSON.stringify(entries || [], null, 2) + '\n', 'application/json')
}
