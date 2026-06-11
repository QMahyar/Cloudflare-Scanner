// Subscribe to a scan job's status with Server-Sent Events, falling back to
// interval polling when EventSource is unavailable or the stream can't be
// established. Only the status/progress channel uses this — results stay on
// their own poll so the sort/filter pipeline is untouched.
//
//   stop = subscribeStatus(sseUrl, pollUrl, { onStatus, isDone, interval })
//
// onStatus(data) is called with each status snapshot (same JSON shape the
// polling endpoint returns). isDone(data) decides when the job is finished;
// once true the subscription tears itself down. Call the returned stop() to
// cancel early (e.g. on reset/unmount); it is idempotent.
export function subscribeStatus(sseUrl, pollUrl, { onStatus, isDone, interval = 300 }) {
  let stopped = false
  let es = null
  let timer = null

  function stop() {
    stopped = true
    if (es) { es.close(); es = null }
    if (timer) { clearInterval(timer); timer = null }
  }

  function deliver(data) {
    onStatus(data)
    if (isDone(data)) stop()
  }

  function startPolling() {
    if (stopped || timer) return
    timer = setInterval(async () => {
      try {
        const res = await fetch(pollUrl)
        if (!res.ok) throw new Error('status ' + res.status)
        deliver(await res.json())
      } catch {
        // Transient status errors: keep polling until done or stopped.
      }
    }, interval)
  }

  if (typeof EventSource === 'undefined') {
    startPolling()
    return stop
  }

  try {
    es = new EventSource(sseUrl)
    let got = false
    es.onmessage = (e) => {
      got = true
      let data
      try { data = JSON.parse(e.data) } catch { return }
      deliver(data)
    }
    es.onerror = () => {
      // Never received a frame → the endpoint is unavailable (e.g. older
      // build, proxy stripping the stream). Drop SSE and poll instead. If we
      // had already received frames, leave EventSource to auto-reconnect.
      if (!got && !stopped) {
        es.close(); es = null
        startPolling()
      }
    }
  } catch {
    startPolling()
  }

  return stop
}
