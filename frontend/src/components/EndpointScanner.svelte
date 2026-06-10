<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON } from '../lib/api.js'
  import { copyToClipboard, sleep, downloadText } from '../lib/clipboard.js'
  import { formatEps } from '../lib/copymode.js'
  import { sortEntries, parseLatency, latClass, toggleSort } from '../lib/sort.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'
  import { notifyDone, scanRateText } from '../lib/notify.js'
  import { endpointRaw, activeTab, getSetting, setSetting } from '../lib/stores.js'
  import { pendingWarpEndpoint, replacerCtype } from '../lib/handoff.js'
  import SplitCopyButton from './SplitCopyButton.svelte'
  import HelpPanel from './HelpPanel.svelte'

  const DEPTHS = [
    { v: '100', k: 'settings.depth.quick' },
    { v: '500', k: 'settings.depth.normal' },
    { v: '1000', k: 'settings.depth.deep' },
    { v: '5000', k: 'settings.depth.insane' },
    { v: '10000', k: 'settings.depth.massive' },
    { v: '0', k: 'settings.depth.custom' },
  ]

  // ─── Settings (persisted under the original cfscanner_settings keys) ───
  let useConfig = $state(getSetting('useConfigEndpoint', true))
  let scanDepth = $state(getSetting('scanDepth', '500'))
  let customCount = $state(getSetting('customCount', ''))
  let ipVersion = $state(getSetting('ipVersion', '4'))
  let advOpen = $state(getSetting('endpointAdv', false))
  let noise = $state(getSetting('noiseToggle', true))
  let timeoutMs = $state(getSetting('endpointTimeout', '6000'))
  let stopAfter = $state(getSetting('stopAfter', '0'))
  let notify = $state(getSetting('notifyEndpoint', false))

  $effect(() => {
    setSetting('useConfigEndpoint', useConfig)
    setSetting('scanDepth', scanDepth)
    setSetting('customCount', customCount)
    setSetting('ipVersion', ipVersion)
    setSetting('endpointAdv', advOpen)
    setSetting('noiseToggle', noise)
    setSetting('endpointTimeout', timeoutMs)
    setSetting('stopAfter', stopAfter)
    setSetting('notifyEndpoint', notify)
  })

  // ─── File ───
  let files = $state(null)
  const hasFile = $derived(!!(files && files.length))
  const fileName = $derived(hasFile ? files[0].name : '')

  // ─── Results filters ───
  let outCount = $state('0')
  let maxLatency = $state('0')
  let sort = $state({ field: 'num', dir: 'asc' })

  // ─── Scan state ───
  let jobId = $state(null)
  let status = $state('idle') // idle | running | done | cancelled
  let progressPct = $state(0)
  let progressText = $state('')
  let liveCountN = $state(0)
  let total = $state(0)
  let startTime = 0
  let statusTimer = null
  let resultsTimer = null
  let selected = $state(new Set())

  const scanDesc = $derived(useConfig ? $_('endpoint.descFull') : $_('endpoint.descTCP'))
  const startDisabled = $derived(status === 'running' || (useConfig && !hasFile))
  const hasResults = $derived(($endpointRaw?.length || 0) > 0)

  const pool = $derived.by(() => {
    let p = sortEntries($endpointRaw, sort.field, sort.dir)
    const maxLat = parseInt(maxLatency) || 0
    const oc = parseInt(outCount) || 0
    if (maxLat > 0) p = p.filter((e) => parseLatency(e.latency) <= maxLat)
    if (oc > 0 && p.length > oc) p = p.slice(0, oc)
    return p
  })

  function clearTimers() {
    clearInterval(statusTimer); statusTimer = null
    clearInterval(resultsTimer); resultsTimer = null
  }

  async function startScan() {
    if (useConfig && !hasFile) return
    jobId = null
    status = 'running'
    progressPct = 0
    progressText = $_('scan.progressTemplate', { values: { p: 0, t: 0 } })
    liveCountN = 0
    selected = new Set()

    let count = parseInt(scanDepth)
    if (scanDepth === '0') { count = parseInt(customCount) || 100; if (count < 1) count = 100 }
    const params = {
      noise,
      ipv4: ipVersion === '4' || ipVersion === '46',
      ipv6: ipVersion === '6' || ipVersion === '46',
      count,
      outCount: parseInt(outCount) || 0,
      timeoutMs: parseInt(timeoutMs) || 0,
      stop_after: parseInt(stopAfter) || 0,
    }
    const fd = new FormData()
    if (useConfig && hasFile) fd.append('config', files[0])
    fd.append('params', JSON.stringify(params))

    try {
      const data = await apiJSON('/api/scan', { method: 'POST', body: fd })
      jobId = data.id
      startTime = Date.now()
      total = parseInt(data.total)
      pollStatus(data.id)
      pollResults(data.id)
    } catch (e) {
      showToast($_('scan.failed', { values: { msg: e.message } }), true)
      status = 'idle'
    }
  }

  function pollStatus(id) {
    clearInterval(statusTimer)
    statusTimer = setInterval(async () => {
      try {
        const data = await apiJSON('/api/status/' + id)
        const pct = total > 0 ? Math.round((data.progress / total) * 100) : 0
        progressPct = pct
        const rate = data.status === 'running'
          ? scanRateText(data.progress, total, startTime, (s) => $_('scan.eta', { values: { s } }))
          : ''
        progressText = $_('scan.progressTemplate', { values: { p: data.progress, t: total } }) + rate
        if (data.status === 'done' || data.status === 'cancelled') {
          clearTimers()
          status = data.status
          await fetchResults(id, data.status)
          if (notify) notifyDone($_('notify.title'), $_('notify.endpointBody', { values: { n: ($endpointRaw || []).length } }))
        }
      } catch {
        clearInterval(statusTimer); statusTimer = null
      }
    }, 300)
  }

  function pollResults(id) {
    clearInterval(resultsTimer)
    resultsTimer = setInterval(async () => {
      try {
        const data = await apiJSON('/api/results/' + id)
        const raw = data.raw || []
        if (raw.length > 0) {
          endpointRaw.set(raw)
          liveCountN = raw.length
        }
      } catch {}
    }, 1500)
  }

  async function fetchResults(id) {
    for (let i = 0; i < 4; i++) {
      try {
        const data = await apiJSON('/api/results/' + id)
        endpointRaw.set(data.raw || [])
        liveCountN = 0
        return
      } catch {
        await sleep(250)
      }
    }
    liveCountN = 0
  }

  async function stopScan() {
    if (!jobId) return
    try { await fetch('/api/stop/' + jobId, { method: 'POST' }) } catch {}
  }

  function resetAll() {
    clearTimers()
    jobId = null
    status = 'idle'
    progressPct = 0
    progressText = ''
    liveCountN = 0
    selected = new Set()
    endpointRaw.set([])
  }

  function onSort(field) { sort = toggleSort(sort, field) }
  function sortArrow(field) { return sort.dir === 'asc' ? '▲' : '▼' }

  function toggleSelect(ep, on) {
    const s = new Set(selected)
    if (on) s.add(ep); else s.delete(ep)
    selected = s
  }
  function selectAll(on) {
    selected = on ? new Set(pool.map((e) => e.endpoint)) : new Set()
  }

  async function copyAll() {
    let raw
    try { raw = (await apiJSON('/api/results/' + jobId)).raw } catch { raw = $endpointRaw || [] }
    copyToClipboard(formatEps((raw || []).map((r) => r.endpoint)))
    showToast($_('copied.clipboard'))
  }
  async function download() {
    let raw
    try { raw = (await apiJSON('/api/results/' + jobId)).raw } catch { raw = $endpointRaw || [] }
    const text = '# Warp Working Endpoints\n# Generated by Cloudflare Scanner\n\n' +
      formatEps((raw || []).map((r) => r.endpoint)) + '\n'
    downloadText('warp_endpoints.txt', text)
  }
  function copySelected() {
    if (!selected.size) { showToast($_('clean.errNoSelection')); return }
    copyToClipboard(formatEps([...selected]))
    showToast($_('copied.clipboard'))
  }
  function qrAll() {
    showQR(formatEps(($endpointRaw || []).map((r) => r.endpoint)))
  }
  function useEndpoint(ep) {
    pendingWarpEndpoint.set(ep)
    replacerCtype.set('warp')
    activeTab.set('replacer')
    showToast($_('apply.pushed', { values: { ep } }))
  }

  function onKeydown(e) {
    if (e.key === 'Enter' && e.target.matches('input[type=text]') && !startDisabled) {
      e.preventDefault(); startScan()
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div onkeydown={onKeydown}>
  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
      <span>{$_('config.header')}</span>
    </h2>
    <div class="row">
      <div class="col">
        <div class="toggle-wrap" style="padding-top:0">
          <label class="toggle" title={$_('endpoint.useConfigTitle')} aria-label="Toggle config usage">
            <input type="checkbox" bind:checked={useConfig} />
            <span class="slider"></span>
          </label>
          <span class="toggle-label">{$_('endpoint.useConfig')}</span>
        </div>
        <div class:config-fields-disabled={!useConfig}>
          <label for="configFile" title={$_('config.fileTitle')}>{$_('config.fileLabel')}</label>
          <label class="file-input-wrap" for="configFile">
            <input type="file" id="configFile" accept=".conf,.txt" disabled={!useConfig} bind:files />
            <div class="file-label" class:selected={hasFile} title={$_('config.chooseTitle')}>
              {hasFile ? fileName : $_('config.choose')}
            </div>
          </label>
        </div>
      </div>
    </div>
  </div>

  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
      <span>{$_('settings.header')}</span>
    </h2>
    <div class="row">
      <div class="col">
        <label title={$_('settings.depthTitle')}>{$_('settings.scanDepth')}</label>
        <div class="preset-bar">
          {#each DEPTHS as d}
            <button class="preset-btn" class:active={scanDepth === d.v} onclick={() => (scanDepth = d.v)}>{$_(d.k)}</button>
          {/each}
        </div>
        {#if scanDepth === '0'}
          <div class="status-slot">
            <input type="text" bind:value={customCount} placeholder={$_('settings.customPlaceholder')} title={$_('settings.customTitle')} inputmode="numeric" />
          </div>
        {/if}
      </div>
      <div class="col">
        <label for="ipVersion" title={$_('settings.ipTitle')}>{$_('settings.ipVersion')}</label>
        <select id="ipVersion" bind:value={ipVersion} title={$_('settings.ipTitle')}>
          <option value="4">{$_('settings.ipv4')}</option>
          <option value="6">{$_('settings.ipv6')}</option>
          <option value="46">{$_('settings.ipv46')}</option>
        </select>
      </div>
    </div>
    <details class="adv-settings" bind:open={advOpen}>
      <summary>{$_('settings.advanced')}</summary>
      <div class="row">
        <div class="col">
          <div class="toggle-wrap">
            <label class="toggle" title={$_('settings.noiseTitle')} aria-label="Toggle UDP noise">
              <input type="checkbox" bind:checked={noise} />
              <span class="slider"></span>
            </label>
            <span class="toggle-label" title={$_('settings.noiseTitle')}>{$_('settings.noise')}</span>
          </div>
        </div>
        <div class="col">
          <label for="endpointTimeout" title={$_('settings.timeoutTitle')}>{$_('settings.timeoutLabel')}</label>
          <input id="endpointTimeout" type="text" bind:value={timeoutMs} inputmode="numeric" title={$_('settings.timeoutTitle')} />
        </div>
      </div>
      <div class="row">
        <div class="col">
          <label for="stopAfter" title={$_('settings.stopAfterTitle')}>{$_('settings.stopAfter')}</label>
          <input id="stopAfter" type="text" bind:value={stopAfter} inputmode="numeric" title={$_('settings.stopAfterTitle')} />
        </div>
        <div class="col">
          <div class="toggle-wrap" style="padding-top:22px">
            <label class="toggle" title={$_('settings.notifyTitle')} aria-label="Toggle completion notification">
              <input type="checkbox" bind:checked={notify} />
              <span class="slider"></span>
            </label>
            <span class="toggle-label" title={$_('settings.notifyTitle')}>{$_('settings.notify')}</span>
          </div>
        </div>
      </div>
    </details>
    <div class="scan-desc">{scanDesc}</div>
  </div>

  <div class="btn-bar">
    <button class="btn btn-primary" onclick={startScan} disabled={startDisabled} title={$_('buttons.startTitle')}>
      {status === 'running' ? $_('scan.scanning') : $_('buttons.start')}
    </button>
    <button class="btn btn-danger" onclick={stopScan} disabled={status !== 'running'} title={$_('buttons.stopTitle')}>{$_('buttons.stop')}</button>
    <button class="btn btn-secondary" onclick={startScan} disabled={status === 'running' || !hasResults} title={$_('buttons.rescanTitle')}>{$_('buttons.rescan')}</button>
    <button class="btn btn-secondary" onclick={resetAll} title={$_('buttons.resetTitle')}>{$_('buttons.reset')}</button>
  </div>

  <div class="card" id="resultsCard">
    <div class="section-header">
      <h2>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 3v18h18"/><path d="m19 9-5 5-4-4-3 3"/></svg>
        <span>{$_('results.header')}</span>
      </h2>
      <div style="display:flex;gap:var(--space-md);align-items:center;flex-wrap:wrap">
        <div class="compact-control">
          <label for="maxLatency" title={$_('results.maxLatTitle')}>{$_('results.maxLat')}</label>
          <input class="compact-input" id="maxLatency" type="text" bind:value={maxLatency} title={$_('results.maxLatTitle')} inputmode="numeric" />
        </div>
        <div class="compact-control">
          <label for="outCount" title={$_('settings.outCountTitle')}>{$_('settings.outCount')}</label>
          <input class="compact-input" id="outCount" type="text" bind:value={outCount} title={$_('settings.outCountTitle')} inputmode="numeric" />
        </div>
      </div>
    </div>

    {#if status === 'running' || status === 'done' || status === 'cancelled'}
      <div class="progress-wrap active">
        <div class="progress-bar"><div class="progress-fill" class:cancelled={status === 'cancelled'} style="width:{progressPct}%"></div></div>
        <div class="progress-text">{progressText}</div>
      </div>
    {/if}

    {#if !hasResults}
      <div class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
        <p>{status === 'done' || status === 'cancelled' ? $_('results.notFound') : $_('results.empty')}</p>
      </div>
    {:else}
      <div class="results-actions results-actions-top">
        <SplitCopyButton oncopy={copyAll} title={$_('results.copyAllTitle')} />
        <button class="btn btn-secondary btn-sm" onclick={download} title={$_('results.downloadTitle')}>{$_('results.downloadRaw')}</button>
        <button class="btn btn-secondary btn-sm" onclick={qrAll} title="QR">QR</button>
        <button class="btn btn-secondary btn-sm" onclick={copySelected} title={$_('results.copySelectedTitle')}>{$_('results.copySelected')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(true)}>{$_('results.selectAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(false)}>{$_('results.deselectAll')}</button>
      </div>
      <div class="results-table-wrap">
        <table class="results-table">
          <thead>
            <tr>
              <th class="sortable" onclick={() => onSort('num')}>{$_('results.tableNum')}{#if sort.field === 'num'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
              <th class="sortable" onclick={() => onSort('endpoint')}>{$_('results.tableEndpoint')}{#if sort.field === 'endpoint'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
              <th class="sortable" onclick={() => onSort('latency')}>{$_('results.tableLatency')}{#if sort.field === 'latency'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
              <th></th>
              <th class="checkbox-cell"></th>
            </tr>
          </thead>
          <tbody>
            {#each pool as e, i (e.endpoint)}
              <tr>
                <td class="num">{i + 1}</td>
                <td><span class="tag" onclick={() => { copyToClipboard(e.endpoint); showToast($_('copied.clipboard')) }} title={$_('results.tableEndpoint')}>{e.endpoint}</span></td>
                <td class={latClass(e.latency)}>{e.latency}</td>
                <td><button class="btn btn-secondary btn-sm" onclick={() => useEndpoint(e.endpoint)} title={$_('results.tableUse')}>{$_('results.tableUse')}</button></td>
                <td class="checkbox-cell"><input type="checkbox" checked={selected.has(e.endpoint)} onchange={(ev) => toggleSelect(e.endpoint, ev.currentTarget.checked)} /></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if status === 'done' || status === 'cancelled'}
        <div class={status === 'cancelled' ? 'error-msg' : 'success-msg'}>
          {status === 'cancelled' ? $_('results.scanCancelled') : $_('results.found', { values: { n: ($endpointRaw || []).length, s: total || ($endpointRaw || []).length } })}
        </div>
      {/if}
    {/if}

    {#if liveCountN > 0}
      <div class="live-count">{$_('results.working', { values: { n: liveCountN } })}</div>
    {/if}
  </div>

  <div class="card">
    <HelpPanel />
  </div>
</div>
