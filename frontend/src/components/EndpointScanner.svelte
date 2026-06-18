<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON, withCSRF } from '../lib/api.js'
  import { copyToClipboard, sleep, downloadText } from '../lib/clipboard.js'
  import { formatEps, copyWithPorts, setCopyMode } from '../lib/copymode.js'
  import { sortEntries, parseLatency, latClass, latBar, toggleSort } from '../lib/sort.js'
  import { computeSummary } from '../lib/scanMetrics.js'
  import { activateKey } from '../lib/a11y.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'
  import { notifyDone, scanRateText } from '../lib/notify.js'
  import { subscribeStatus } from '../lib/sse.js'
  import { endpointRaw, activeTab, getSetting, setSetting } from '../lib/stores.js'
  import { pendingWarpEndpoint, replacerCtype } from '../lib/handoff.js'
  import HelpPanel from './HelpPanel.svelte'
  import VirtualTable from './VirtualTable.svelte'
  import ScanProgress from './ScanProgress.svelte'

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
  let concurrency = $state(getSetting('endpointConcurrency', '0'))
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
    setSetting('endpointConcurrency', concurrency)
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
  let scanMs = $state(0)
  let statusStop = null
  let resultsTimer = null
  let selected = $state(new Set())
  let failInfo = $state(null) // { reasons: [{k,n}], examples: [{endpoint,error}], scanned }

  const scanDesc = $derived(useConfig ? $_('endpoint.descFull') : $_('endpoint.descTCP'))
  const startDisabled = $derived(status === 'running' || (useConfig && !hasFile))
  const hasResults = $derived(($endpointRaw?.length || 0) > 0)

  const failReasons = $derived.by(() => {
    const reasons = failInfo?.reasons || {}
    return Object.keys(reasons).sort((a, b) => reasons[b] - reasons[a]).map((k) => ({ k, n: reasons[k] }))
  })
  const failExamples = $derived((failInfo?.examples || []).slice(0, 5))

  const pool = $derived.by(() => {
    let p = sortEntries($endpointRaw, sort.field, sort.dir)
    const maxLat = parseInt(maxLatency) || 0
    const oc = parseInt(outCount) || 0
    if (maxLat > 0) p = p.filter((e) => parseLatency(e.latency) <= maxLat)
    if (oc > 0 && p.length > oc) p = p.slice(0, oc)
    return p
  })

  // Post-scan metrics for the summary strip (null until a scan finishes).
  const summary = $derived.by(() => {
    if (status !== 'done' && status !== 'cancelled') return null
    return computeSummary($endpointRaw, total, scanMs)
  })

  function clearTimers() {
    if (statusStop) { statusStop(); statusStop = null }
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
    failInfo = null

    let count = parseInt(scanDepth)
    if (scanDepth === '0') { count = parseInt(customCount) || 100; if (count < 1) count = 100 }
    const params = {
      noise,
      ipv4: ipVersion === '4' || ipVersion === '46',
      ipv6: ipVersion === '6' || ipVersion === '46',
      count,
      outCount: parseInt(outCount) || 0,
      concurrency: parseInt(concurrency) || 0,
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
    if (statusStop) statusStop()
    statusStop = subscribeStatus('/api/scan-events/' + id, '/api/status/' + id, {
      onStatus(data) {
        const pct = total > 0 ? Math.round((data.progress / total) * 100) : 0
        progressPct = pct
        const rate = data.status === 'running'
          ? scanRateText(data.progress, total, startTime, (s) => $_('scan.eta', { values: { s } }))
          : ''
        progressText = $_('scan.progressTemplate', { values: { p: data.progress, t: total } }) + rate
        if (data.status === 'done' || data.status === 'cancelled') {
          finishScan(id, data.status)
        }
      },
      isDone: (d) => d.status === 'done' || d.status === 'cancelled',
    })
  }

  async function finishScan(id, st) {
    scanMs = startTime ? Date.now() - startTime : 0
    clearTimers()
    status = st
    await fetchResults(id)
    if (notify) notifyDone($_('notify.title'), $_('notify.endpointBody', { values: { n: ($endpointRaw || []).length } }))
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
        failInfo = { reasons: data.fail_reasons || {}, examples: data.failures || [], scanned: data.scanned || 0 }
        return
      } catch {
        await sleep(250)
      }
    }
    liveCountN = 0
  }

  async function stopScan() {
    if (!jobId) return
    try { await fetch('/api/stop/' + jobId, withCSRF({ method: 'POST' })) } catch {}
  }

  function resetAll() {
    clearTimers()
    jobId = null
    status = 'idle'
    progressPct = 0
    progressText = ''
    liveCountN = 0
    selected = new Set()
    failInfo = null
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

{#snippet header()}
  <tr>
    <th class="sortable" onclick={() => onSort('num')}>{$_('results.tableNum')}{#if sort.field === 'num'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('endpoint')}>{$_('results.tableEndpoint')}{#if sort.field === 'endpoint'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('latency')}>{$_('results.tableLatency')}{#if sort.field === 'latency'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th></th>
    <th class="checkbox-cell"></th>
  </tr>
{/snippet}

{#snippet row(e, i, measure)}
  <tr data-index={i} use:measure>
    <td class="num">{i + 1}</td>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <td><span class="tag" role="button" tabindex="0" onclick={() => { copyToClipboard(e.endpoint); showToast($_('copied.clipboard')) }} use:activateKey={() => { copyToClipboard(e.endpoint); showToast($_('copied.clipboard')) }} title={$_('results.tableEndpoint')}>{e.endpoint}</span></td>
    <td class="lat-cell {latClass(e.latency)}"><span class="lat-meter"><span class="lat-meter-fill" style="width:{latBar(e.latency)}%"></span></span><span class="lat-val">{e.latency}</span></td>
    <td><button class="btn btn-secondary btn-sm" onclick={() => useEndpoint(e.endpoint)} title={$_('results.tableUse')}>{$_('results.tableUse')}</button></td>
    <td class="checkbox-cell"><input type="checkbox" checked={selected.has(e.endpoint)} onchange={(ev) => toggleSelect(e.endpoint, ev.currentTarget.checked)} /></td>
  </tr>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div onkeydown={onKeydown}>
  <div class="card">
    <h2>
      <span class="step-num">1</span>
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
      <span class="step-num">2</span>
      <span>{$_('settings.header')}</span>
    </h2>
    <div class="row">
      <div class="col">
        <div class="field-label" title={$_('settings.depthTitle')}>{$_('settings.scanDepth')}</div>
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
        <div class="col"></div>
      </div>
      <div class="row">
        <div class="col">
          <label for="endpointTimeout" title={$_('settings.timeoutTitle')}>{$_('settings.timeoutLabel')}</label>
          <input id="endpointTimeout" type="text" bind:value={timeoutMs} inputmode="numeric" title={$_('settings.timeoutTitle')} />
        </div>
        <div class="col">
          <label for="endpointConcurrency" title={$_('settings.concurrencyTitle')}>{$_('settings.concurrencyLabel')}</label>
          <input id="endpointConcurrency" type="text" bind:value={concurrency} inputmode="numeric" placeholder="0 (auto)" title={$_('settings.concurrencyTitle')} />
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

  <div class="action-bar">
    <button class="btn btn-primary action-primary" onclick={startScan} disabled={startDisabled} title={$_('buttons.startTitle')}>
      {status === 'running' ? $_('scan.scanning') : $_('buttons.start')}
    </button>
    {#if status === 'running'}
      <button class="btn btn-danger" onclick={stopScan} title={$_('buttons.stopTitle')}>{$_('buttons.stop')}</button>
    {/if}
    <div class="action-bar-rest">
      <button class="btn btn-secondary btn-sm" onclick={startScan} disabled={status === 'running' || !hasResults} title={$_('buttons.rescanTitle')}>{$_('buttons.rescan')}</button>
      <button class="btn btn-ghost btn-sm" onclick={resetAll} title={$_('buttons.resetTitle')}>{$_('buttons.reset')}</button>
    </div>
  </div>

  <div class="card" id="resultsCard">
    <div class="section-header">
      <h2>
        <span class="step-num">3</span>
        <span>{$_('results.header')}</span>
        {#if hasResults}<span class="count-chip">{pool.length}</span>{/if}
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

    <ScanProgress {status} {progressPct} {progressText} {summary} runningLabel={$_('scan.scanning')} />

    {#if !hasResults}
      <div class="empty-state">
        {#if status === 'done' || status === 'cancelled'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="m9 9 6 6m0-6-6 6"/></svg>
          <p>{$_('results.notFound')}{#if failInfo?.scanned > 0} ({failInfo.scanned} {$_('results.testedAllFailed')}){/if}</p>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
          <p>{$_('results.empty')}</p>
        {/if}
      </div>
      {#if (status === 'done' || status === 'cancelled') && failReasons.length > 0}
        <div class="fail-panel">
          <div class="fail-title">{$_('results.whyFailed')}</div>
          <ul class="fail-list">
            {#each failReasons as r}<li><span class="fail-count">{r.n}×</span> {r.k}</li>{/each}
          </ul>
          {#if failExamples.length > 0}
            <details class="fail-examples">
              <summary>{$_('clean.failExamples')}</summary>
              <div class="fail-ex-wrap">
                {#each failExamples as f}
                  <div class="fail-ex"><span class="tag">{f.endpoint}</span> <span class="fail-ex-err">{f.error || ''}</span></div>
                {/each}
              </div>
            </details>
          {/if}
        </div>
      {/if}
    {:else}
      <div class="results-actions results-actions-top">
        <button class="btn btn-secondary btn-sm" onclick={copyAll} title={$_('results.copyAllTitle')}>{$_('results.copyAll')}</button>
        <select class="copy-mode-select" value={$copyWithPorts ? 'port' : 'ip'} onchange={(e) => setCopyMode(e.currentTarget.value === 'port')} title={$_('copy.menuTitle')} aria-label={$_('copy.menuTitle')}>
          <option value="port">{$_('copy.withPort')}</option>
          <option value="ip">{$_('copy.ipOnly')}</option>
        </select>
        <button class="btn btn-secondary btn-sm" onclick={download} title={$_('results.downloadTitle')}>{$_('results.downloadRaw')}</button>
        <button class="btn btn-secondary btn-sm" onclick={qrAll} title="QR">QR</button>
        <button class="btn btn-secondary btn-sm" onclick={copySelected} title={$_('results.copySelectedTitle')}>{$_('results.copySelected')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(true)}>{$_('results.selectAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(false)}>{$_('results.deselectAll')}</button>
      </div>
      <VirtualTable items={pool} getKey={(e) => e.endpoint} colspan={5} {header} {row} />
    {/if}

    {#if liveCountN > 0}
      <div class="live-count">{$_('results.working', { values: { n: liveCountN } })}</div>
    {/if}
  </div>

  <div class="card">
    <HelpPanel />
  </div>
</div>
