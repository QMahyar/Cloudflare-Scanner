// Subscribe to status with SSE and fall back to bounded polling. EventSource's
// built-in retry is deliberately avoided: an expired job otherwise reconnects
// forever after the server's TTL removes it.
export function subscribeStatus(sseUrl, pollUrl, {
  onStatus,
  isDone,
  interval = 500,
  maxErrors = 12,
  onConnectionChange = () => {},
}) {
  let stopped = false
  let es = null
  let timer = null
  let errors = 0

  function stop() {
    if (stopped) return
    stopped = true
    if (es) { es.close(); es = null }
    if (timer) { clearTimeout(timer); timer = null }
  }

  function deliver(data) {
    errors = 0
    onConnectionChange('connected')
    onStatus(data)
    if (isDone(data)) stop()
  }

  function schedulePoll(delay = interval) {
    if (stopped || timer) return
    timer = setTimeout(poll, delay)
  }

  async function poll() {
    timer = null
    if (stopped) return
    try {
      const res = await fetch(pollUrl)
      if (res.status === 404) { onConnectionChange('expired'); stop(); return }
      if (!res.ok) throw new Error('status ' + res.status)
      deliver(await res.json())
    } catch {
      errors++
      onConnectionChange('reconnecting')
      if (errors >= maxErrors) { onConnectionChange('failed'); stop(); return }
    }
    if (!stopped) schedulePoll(Math.min(interval * 2 ** Math.min(errors, 4), 5000))
  }

  if (typeof EventSource === 'undefined') {
    schedulePoll(0)
    return stop
  }

  try {
    es = new EventSource(sseUrl)
    es.onmessage = (event) => {
      let data
      try { data = JSON.parse(event.data) } catch { return }
      deliver(data)
    }
    es.onerror = () => {
      if (stopped) return
      es.close(); es = null
      onConnectionChange('reconnecting')
      schedulePoll(0)
    }
  } catch {
    schedulePoll(0)
  }

  return stop
}
