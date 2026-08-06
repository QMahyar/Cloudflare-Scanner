<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON, withCSRF } from '../lib/api.js'
  import { copyToClipboard, sleep, downloadText } from '../lib/clipboard.js'
  import { formatEps } from '../lib/copymode.js'
  import { sortEntries, parseLatency, latClass, latBar, toggleSort, scoreClass, scoreBar, fmtLoss, lossClass } from '../lib/sort.js'
  import { computeSummary } from '../lib/scanMetrics.js'
  import { exportCSV, exportJSON } from '../lib/exporters.js'
  import { activateKey } from '../lib/a11y.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'
  import { notifyDone, scanRateText } from '../lib/notify.js'
  import { subscribeStatus } from '../lib/sse.js'
  import { appState, getSetting, setSetting } from '../lib/stores.svelte.js'
  import {
    SCAN_DEPTHS,
    ENDPOINT_DEFAULTS as D,
    ENDPOINT_CONCURRENCY_OPTIONS,
    ENDPOINT_ATTEMPTS_OPTIONS,
  } from '../lib/scanDefaults.js'
  import { pendingWarpEndpoint, replacerCtype } from '../lib/handoff.js'
  import HelpPanel from './HelpPanel.svelte'
  import VirtualTable from './VirtualTable.svelte'
  import ScanProgress from './ScanProgress.svelte'
  import ResultCharts from './ResultCharts.svelte'
  import ResultsActions from './ResultsActions.svelte'
  import Toggle from './ui/Toggle.svelte'
  import Disclosure from './ui/Disclosure.svelte'
  import FilterInput from './ui/FilterInput.svelte'
  import FileDrop from './ui/FileDrop.svelte'

  let useConfig = $state(getSetting('useConfigEndpoint', D.useConfig))
  let scanDepth = $state(getSetting('scanDepth', D.scanDepth))
  let customCount = $state(getSetting('customCount', D.customCount))
  let ipVersion = $state(getSetting('ipVersion', D.ipVersion))
  let advOpen = $state(getSetting('endpointAdv', D.advOpen))

  let noise = $state(getSetting('noiseToggle', D.noise))

  let timeoutMs = $state(getSetting('endpointTimeout', D.timeoutMs))
  let concurrency = $state(getSetting('endpointConcurrency', D.concurrency))
  let attempts = $state(getSetting('endpointAttempts', D.attempts))
  let stopAfter = $state(getSetting('stopAfter', D.stopAfter))
  let notify = $state(getSetting('notifyEndpoint', D.notify))

  $effect(() => setSetting('useConfigEndpoint', useConfig))
  $effect(() => setSetting('scanDepth', scanDepth))
  $effect(() => setSetting('customCount', customCount))
  $effect(() => setSetting('ipVersion', ipVersion))
  $effect(() => setSetting('endpointAdv', advOpen))
  $effect(() => setSetting('noiseToggle', noise))
  $effect(() => setSetting('endpointTimeout', timeoutMs))
  $effect(() => setSetting('endpointConcurrency', concurrency))
  $effect(() => setSetting('endpointAttempts', attempts))
  $effect(() => setSetting('stopAfter', stopAfter))
  $effect(() => setSetting('notifyEndpoint', notify))

  let files = $state.raw(null)
  let configText = $state(getSetting('endpointConfigText', ''))
  const hasFile = $derived(!!(files && files.length))
  const fileName = $derived(hasFile ? files[0].name : '')
  const hasConfig = $derived(hasFile || !!configText.trim())

  $effect(() => { setSetting('endpointConfigText', configText) })

  let outCount = $state(D.outCount)
  let maxLatency = $state(D.maxLatency)
  let sort = $state({ field: 'score', dir: 'desc' })

  let jobId = $state(null)
  let status = $state('idle')

  $effect(() => { endpointScanning.set(status === 'running') })
  let progressPct = $state(0)
  let progressText = $state('')
  let liveCountN = $state(0)
  let total = $state(0)
  let startTime = 0
  let scanMs = $state(0)
  let statusStop = null
  let lastFetch = 0
  let fetchTimer = null
  let selected = $state(new Set())

  let failInfo = $state.raw(null)

  const scanDesc = $derived(useConfig ? $_('endpoint.descFull') : $_('endpoint.descTCP'))
  const startDisabled = $derived(status === 'running' || (useConfig && !hasConfig))
  const hasResults = $derived((appState.endpointRaw?.length || 0) > 0)

  const failReasons = $derived.by(() => {
    const reasons = failInfo?.reasons || {}
    return Object.keys(reasons).sort((a, b) => reasons[b] - reasons[a]).map((k) => ({ k, n: reasons[k] }))
  })
  const failExamples = $derived((failInfo?.examples || []).slice(0, 5))

  const pool = $derived.by(() => {
    let p = sortEntries(appState.endpointRaw, sort.field, sort.dir)
    const maxLat = parseInt(maxLatency) || 0
    const oc = parseInt(outCount) || 0
    if (maxLat > 0) p = p.filter((e) => parseLatency(e.latency) <= maxLat)
    if (oc > 0 && p.length > oc) p = p.slice(0, oc)
    return p
  })

  const summary = $derived.by(() => {
    if (status !== 'done' && status !== 'cancelled') return null
    return computeSummary(appState.endpointRaw, total, scanMs)
  })

  function clearTimers() {
    if (statusStop) { statusStop(); statusStop = null }
    if (fetchTimer) { clearTimeout(fetchTimer); fetchTimer = null }
  }

  async function startScan() {
    if (useConfig && !hasConfig) return
    jobId = null
    status = 'running'
    progressPct = 0
    progressText = $_('scan.progressTemplate', { values: { p: 0, t: 0 } })
    liveCountN = 0
    selected = new Set()
    failInfo = null

    appState.endpointRaw = []

    let count = parseInt(scanDepth)
    if (scanDepth === '0') { count = parseInt(customCount) || 100; if (count < 1) count = 100 }
    const params = {
      noise,
      ipv4: ipVersion === '4' || ipVersion === '46',
      ipv6: ipVersion === '6' || ipVersion === '46',
      count,
      outCount: parseInt(outCount) || 0,
      concurrency: parseInt(concurrency) || 0,
      attempts: parseInt(attempts) || 1,
      timeoutMs: parseInt(timeoutMs) || 0,
      stop_after: parseInt(stopAfter) || 0,
    }
    const fd = new FormData()

    if (useConfig && hasFile) fd.append('config', files[0])
    else if (useConfig && configText.trim()) fd.append('config_text', configText.trim())
    fd.append('params', JSON.stringify(params))

    try {
      const data = await apiJSON('/api/scan', { method: 'POST', body: fd })
      jobId = data.id
      startTime = Date.now()
      lastFetch = 0
      total = parseInt(data.total)
      pollStatus(data.id)
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
        } else {

          scheduleFetch(id)
        }
      },
      isDone: (d) => d.status === 'done' || d.status === 'cancelled',
    })
  }

  function scheduleFetch(id) {
    const wait = 600 - (Date.now() - lastFetch)
    if (wait <= 0) { lastFetch = Date.now(); liveFetch(id) }
    else if (!fetchTimer) {
      fetchTimer = setTimeout(() => { fetchTimer = null; lastFetch = Date.now(); liveFetch(id) }, wait)
    }
  }

  async function liveFetch(id) {
    try {
      const data = await apiJSON('/api/results/' + id)
      const raw = data.raw || []
      if (raw.length > 0) { appState.endpointRaw = raw; liveCountN = raw.length }
    } catch {}
  }

  async function finishScan(id, st) {
    scanMs = startTime ? Date.now() - startTime : 0
    clearTimers()
    status = st
    await fetchResults(id)

    if (st === 'done' && notify) notifyDone($_('notify.title'), $_('notify.endpointBody', { values: { n: (appState.endpointRaw || []).length } }))
  }

  async function fetchResults(id) {
    for (let i = 0; i < 4; i++) {
      try {
        const data = await apiJSON('/api/results/' + id)
        appState.endpointRaw = data.raw || []
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
    appState.endpointRaw = []
  }

  function onSort(field) { sort = toggleSort(sort, field) }
  function sortArrow() { return sort.dir === 'asc' ? '▲' : '▼' }

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
    try { raw = (await apiJSON('/api/results/' + jobId)).raw } catch { raw = appState.endpointRaw || [] }
    copyToClipboard(formatEps((raw || []).map((r) => r.endpoint)))
    showToast($_('copied.clipboard'))
  }
  async function download() {
    let raw
    try { raw = (await apiJSON('/api/results/' + jobId)).raw } catch { raw = appState.endpointRaw || [] }
    const text = '# Warp Working Endpoints\n# Generated by Cloudflare Scanner\n\n' +
      formatEps((raw || []).map((r) => r.endpoint)) + '\n'
    downloadText('warp_endpoints.txt', text)
  }
  function downloadCsv() {
    if (!pool.length) { showToast($_('clean.errNoSelection')); return }
    exportCSV('warp_endpoints.csv', pool)
  }
  function downloadJson() {
    if (!pool.length) { showToast($_('clean.errNoSelection')); return }
    exportJSON('warp_endpoints.json', pool)
  }
  function copySelected() {
    if (!selected.size) { showToast($_('clean.errNoSelection')); return }
    copyToClipboard(formatEps([...selected]))
    showToast($_('copied.clipboard'))
  }
  function qrAll() {
    showQR(formatEps((appState.endpointRaw || []).map((r) => r.endpoint)))
  }
  function useEndpoint(ep) {
    pendingWarpEndpoint.set(ep)
    replacerCtype.set('warp')
    activeTab.set('replacer')
    showToast($_('apply.pushed', { values: { ep } }))
  }

  const ENTER_START_IDS = new Set(['customCount', 'endpointTimeout', 'stopAfter'])
  function onKeydown(e) {
    if (e.key === 'Enter' && e.target.matches('input[type=text]') && ENTER_START_IDS.has(e.target.id) && !startDisabled) {
      e.preventDefault(); startScan()
    } else if (e.key === 'Escape' && status === 'running') {
      e.preventDefault(); stopScan()
    }
  }
</script>

{#snippet header()}
  <tr>
    <th class="sortable" onclick={() => onSort('num')}>{$_('results.tableNum')}{#if sort.field === 'num'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('endpoint')}>{$_('results.tableEndpoint')}{#if sort.field === 'endpoint'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('score')} title={$_('results.tableScoreTitle')}>{$_('results.tableScore')}{#if sort.field === 'score'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('latency')}>{$_('results.tableLatency')}{#if sort.field === 'latency'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="sortable" onclick={() => onSort('loss')} title={$_('results.tableLossTitle')}>{$_('results.tableLoss')}{#if sort.field === 'loss'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th></th>
    <th class="checkbox-cell"></th>
  </tr>
{/snippet}

{#snippet row(e, i, measure)}
  <tr data-index={i} {@attach measure}>
    <td class="num">{i + 1}</td>
    <td><button type="button" class="tag tag-btn" onclick={() => { copyToClipboard(e.endpoint); showToast($_('copied.clipboard')) }} title={$_('results.tableEndpoint')}>{e.endpoint}</button></td>
    <td class="lat-cell {scoreClass(e.score)}"><span class="lat-meter"><span class="lat-meter-fill" style="width:{scoreBar(e.score)}%"></span></span><span class="lat-val">{e.score || '—'}</span></td>
    <td class="lat-cell {latClass(e.latency)}"><span class="lat-meter"><span class="lat-meter-fill" style="width:{latBar(e.latency)}%"></span></span><span class="lat-val">{e.latency}</span></td>
    <td class="lat-cell {e.score ? lossClass(e.loss) : ''}"><span class="lat-val">{e.score ? fmtLoss(e.loss) : '—'}</span></td>
    <td><button class="btn btn-secondary btn-sm" onclick={() => useEndpoint(e.endpoint)} title={$_('results.tableUse')}>{$_('results.tableUse')}</button></td>
    <td class="checkbox-cell"><input type="checkbox" checked={selected.has(e.endpoint)} onchange={(ev) => toggleSelect(e.endpoint, ev.currentTarget.checked)} /></td>
  </tr>
{/snippet}

<div class="workbench" role="toolbar" tabindex="-1" aria-label={$_('endpoint.setupHeader')} onkeydown={onKeydown}>
  <div class="workbench-main endpoint-main">
    <div class="card card-setup">
      <h2>
        <span class="step-num">1</span>
        <span>{$_('endpoint.setupHeader')}</span>
      </h2>
      <div class="setup-grid setup-split">
        <div class="setup-block">
          <p class="setup-block-title">{$_('config.header')}</p>
          <Toggle bind:checked={useConfig} label={$_('endpoint.useConfig')} title={$_('endpoint.useConfigTitle')} ariaLabel={$_('endpoint.useConfig')} />
          <div class={{ 'config-fields-disabled': !useConfig }}>
            <label for="configFile" title={$_('config.fileTitle')}>{$_('config.fileLabel')}</label>
            <FileDrop
              id="configFile"
              accept=".conf,.txt"
              disabled={!useConfig}
              bind:files
              label={$_('config.choose')}
              selectedLabel={fileName}
              title={$_('config.chooseTitle')}
            />
            <label for="configText" title={$_('config.pasteTitle')}>{$_('config.pasteLabel')}</label>
            <textarea
              id="configText"
              rows="6"
              bind:value={configText}
              disabled={!useConfig}
              placeholder={$_('config.pastePlaceholder')}
              title={$_('config.pasteTitle')}
              spellcheck="false"
            ></textarea>
            <p class="scan-desc">{$_('config.pasteHint')}</p>
          </div>
        </div>
        <div class="setup-block">
          <p class="setup-block-title">{$_('settings.header')}</p>
          <fieldset class="preset-fieldset">
            <legend class="field-label" title={$_('settings.depthTitle')}>{$_('settings.scanDepth')}</legend>
            <div class="preset-bar">
              {#each SCAN_DEPTHS as d (d.v)}
                <button type="button" class={['preset-btn', { active: scanDepth === d.v }]} onclick={() => (scanDepth = d.v)}>{$_(d.k)}</button>
              {/each}
            </div>
          </fieldset>
          {#if scanDepth === '0'}
            <div class="status-slot">
              <input id="customCount" type="text" bind:value={customCount} placeholder={$_('settings.customPlaceholder')} title={$_('settings.customTitle')} inputmode="numeric" />
            </div>
          {/if}
          <label for="ipVersion" title={$_('settings.ipTitle')}>{$_('settings.ipVersion')}</label>
          <select id="ipVersion" bind:value={ipVersion} title={$_('settings.ipTitle')}>
            <option value="4">{$_('settings.ipv4')}</option>
            <option value="6">{$_('settings.ipv6')}</option>
            <option value="46">{$_('settings.ipv46')}</option>
          </select>
          <Disclosure bind:open={advOpen} summary={$_('settings.advanced')}>
            <div class="row">
              <div class="col">
                <Toggle bind:checked={noise} label={$_('settings.noise')} title={$_('settings.noiseTitle')} ariaLabel={$_('settings.noise')} />
                {#if noise && !useConfig}
                  <p class="hint-warn noise-hint">{$_('settings.noiseNeedsConfig')}</p>
                {/if}
              </div>
              <div class="col">
                <label for="endpointAttempts" title={$_('settings.attemptsTitle')}>{$_('settings.attemptsLabel')}</label>
                <select id="endpointAttempts" bind:value={attempts} title={$_('settings.attemptsTitle')}>
                  {#each ENDPOINT_ATTEMPTS_OPTIONS as v (v)}<option value={v}>{v}</option>{/each}
                </select>
              </div>
            </div>
            <div class="row">
              <div class="col">
                <label for="endpointTimeout" title={$_('settings.timeoutTitle')}>{$_('settings.timeoutLabel')}</label>
                <input id="endpointTimeout" type="text" bind:value={timeoutMs} inputmode="numeric" title={$_('settings.timeoutTitle')} placeholder={D.timeoutMs} />
              </div>
              <div class="col">
                <label for="endpointConcurrency" title={$_('settings.concurrencyTitle')}>{$_('settings.concurrencyLabel')}</label>
                <select id="endpointConcurrency" bind:value={concurrency} title={$_('settings.concurrencyTitle')}>
                  {#each ENDPOINT_CONCURRENCY_OPTIONS as o (o.v)}
                    <option value={o.v}>{o.v === '0' ? $_('settings.concurrencyAuto') : o.label}</option>
                  {/each}
                </select>
              </div>
            </div>
            <div class="row">
              <div class="col">
                <label for="stopAfter" title={$_('settings.stopAfterTitle')}>{$_('settings.stopAfter')}</label>
                <input id="stopAfter" type="text" bind:value={stopAfter} inputmode="numeric" title={$_('settings.stopAfterTitle')} placeholder="0" />
              </div>
              <div class="col">
                <Toggle bind:checked={notify} label={$_('settings.notify')} title={$_('settings.notifyTitle')} ariaLabel={$_('settings.notify')} align="field" />
              </div>
            </div>
          </Disclosure>
        </div>
      </div>
    </div>

    <div class="scan-cta-row">
      <button class="btn btn-primary action-primary" onclick={startScan} disabled={startDisabled} title={$_('buttons.startTitle')}>
        {status === 'running' ? $_('scan.scanning') : $_('buttons.start')}
      </button>
      {#if status === 'running'}
        <button class="btn btn-danger" onclick={stopScan} title={$_('buttons.stopTitle')}>{$_('buttons.stop')}</button>
      {/if}
      <button class="btn btn-secondary btn-sm" onclick={startScan} disabled={status === 'running' || !hasResults} title={$_('buttons.rescanTitle')}>{$_('buttons.rescan')}</button>
      <button class="btn btn-ghost btn-sm" onclick={resetAll} title={$_('buttons.resetTitle')}>{$_('buttons.reset')}</button>
      <p class="scan-cta-meta">{scanDesc}</p>
    </div>

    <div class="card card-results" id="resultsCard">
      <div class="section-header">
        <h2>
          <span class="step-num">2</span>
          <span>{$_('results.header')}</span>
          {#if hasResults}<span class="count-chip">{pool.length}</span>{/if}
        </h2>
        <div class="section-header-actions">
          <FilterInput id="maxLatency" label={$_('results.maxLat')} title={$_('results.maxLatTitle')} bind:value={maxLatency} inputmode="numeric" />
          <FilterInput id="outCount" label={$_('settings.outCount')} title={$_('settings.outCountTitle')} bind:value={outCount} inputmode="numeric" />
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
              {#each failReasons as r (r.k)}<li><span class="fail-count">{r.n}×</span> {r.k}</li>{/each}
            </ul>
            {#if failExamples.length > 0}
              <details class="fail-examples">
                <summary>{$_('clean.failExamples')}</summary>
                <div class="fail-ex-wrap">
                  {#each failExamples as f (f.endpoint + '|' + (f.error || ''))}
                    <div class="fail-ex"><span class="tag">{f.endpoint}</span> <span class="fail-ex-err">{f.error || ''}</span></div>
                  {/each}
                </div>
              </details>
            {/if}
          </div>
        {/if}
      {:else}
        <ResultsActions
          onCopyAll={copyAll}
          onDownload={download}
          onCSV={downloadCsv}
          onJSON={downloadJson}
          onQR={qrAll}
          onSelectAll={() => selectAll(true)}
          onDeselectAll={() => selectAll(false)}
          onCopySelected={copySelected}
        />
        <ResultCharts entries={pool} showColo={false} />
        <VirtualTable items={pool} getKey={(e) => e.endpoint} colspan={7} {header} {row} />
      {/if}

      {#if liveCountN > 0}
        <div class="live-count">{$_('results.working', { values: { n: liveCountN } })}</div>
      {/if}
    </div>
  </div>

  <aside class="workbench-pane" aria-label={$_('help.paneLabel')}>
    <h3 class="workbench-pane-title">{$_('help.header')}</h3>
    <div class="workbench-pane-body">
      <HelpPanel dense />
    </div>
  </aside>
</div>
