// apiJSON — fetch wrapper that parses JSON and throws on non-2xx with the
// server's {error} message. Faithful port of the original apiJSON().
export async function apiJSON(url, opts = {}) {
  const resp = await fetch(url, opts)
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new Error(data.error || resp.statusText)
  return data
}
