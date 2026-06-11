<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON } from '../lib/api.js'
  import { copyToClipboard, downloadText } from '../lib/clipboard.js'
  import { formatEps } from '../lib/copymode.js'
  import { sortEntries, parseLatency, latClass, toggleSort } from '../lib/sort.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'
  import { notifyDone, scanRateText } from '../lib/notify.js'
  import { subscribeStatus } from '../lib/sse.js'
  import { cleanData, activeTab, getSetting, setSetting } from '../lib/stores.js'
  import { pendingProxyEndpoints, replacerCtype } from '../lib/handoff.js'
  import SplitCopyButton from './SplitCopyButton.svelte'
  import VirtualTable from './VirtualTable.svelte'

  // Official published Cloudflare ranges (cloudflare.com/ips).
  const CF_V4_RANGES = ['173.245.48.0/20', '103.21.244.0/22', '103.22.200.0/22', '103.31.4.0/22', '141.101.64.0/18', '108.162.192.0/18', '190.93.240.0/20', '188.114.96.0/20', '197.234.240.0/22', '198.41.128.0/17', '162.158.0.0/15', '104.16.0.0/13', '104.24.0.0/14', '172.64.0.0/13', '131.0.72.0/22']
  const CF_V6_RANGES = ['2400:cb00::/32', '2606:4700::/32', '2803:f800::/32', '2405:b500::/32', '2405:8100::/32', '2a06:98c0::/29', '2c0f:f248::/32']

  const RANGE_PRESETS = ['104.16.0.0/13', '104.24.0.0/14', '172.64.0.0/13', '162.159.0.0/16', '188.114.96.0/20', '198.41.128.0/17', '2606:4700::/32']
  const DEPTHS = [
    { v: '100', k: 'settings.depth.quick' },
    { v: '500', k: 'settings.depth.normal' },
    { v: '1000', k: 'settings.depth.deep' },
    { v: '5000', k: 'settings.depth.insane' },
    { v: '10000', k: 'settings.depth.massive' },
    { v: '0', k: 'settings.depth.custom' },
  ]
  const HTTPS_PORTS = [443, 8443, 2053, 2083, 2087, 2096]
  const HTTP_PORTS = [80, 8080, 8880, 2052, 2082, 2086, 2095]

  // ─── Settings (persisted under the original cfscanner_settings keys) ───
  let useConfig = $state(getSetting('useConfigClean', true))
  let vlessURL = $state(getSetting('cleanVlessURL', ''))
  let source = $state(getSetting('cleanSource', 'pool'))
  let customRanges = $state(getSetting('cleanCustomRanges', ''))
  let scanDepth = $state(getSetting('cleanDepth', '500'))
  let customCount = $state(getSetting('cleanCustomCount', ''))
  let ipVersion = $state(getSetting('cleanIPVersion', '4'))
  let advOpen = $state(getSetting('cleanAdv', false))
  let phase1Probes = $state(getSetting('phase1Probes', '500'))
  let phase2Probes = $state(getSetting('phase2Probes', '12'))
  let phase2Count = $state(getSetting('cleanPhase2', '20'))
  let ports = $state(getSetting('cleanPorts', [443]))
  let nearby = $state(getSetting('nearbyScan', false))
  let timeout1 = $state(getSetting('cleanTimeout', '3000'))
  let timeout2 = $state(getSetting('cleanPhase2Timeout', '5000'))
  let stopAfter = $state(getSetting('cleanStopAfter', '0'))
  let notify = $state(getSetting('notifyClean', false))

  $effect(() => {
    setSetting('useConfigClean', useConfig)
    setSetting('cleanVlessURL', vlessURL)
    setSetting('cleanSource', source)
    setSetting('cleanCustomRanges', customRanges)
    setSetting('cleanDepth', scanDepth)
    setSetting('cleanCustomCount', customCount)
    setSetting('cleanIPVersion', ipVersion)
    setSetting('cleanAdv', advOpen)
    setSetting('phase1Probes', phase1Probes)
    setSetting('phase2Probes', phase2Probes)
    setSetting('cleanPhase2', phase2Count)
    setSetting('cleanPorts', ports)
    setSetting('nearbyScan', nearby)
    setSetting('cleanTimeout', timeout1)
    setSetting('cleanPhase2Timeout', timeout2)
    setSetting('cleanStopAfter', stopAfter)
    setSetting('notifyClean', notify)
  })

  // ─── Results filters ───
  let coloFilter = $state('')
  let maxLatency = $state('0')
  let outCount = $state('0')
  let sort = $state({ field: 'latency', dir: 'asc' })
  let list = $state('direct') // direct | nearby
  let selected = $state(new Set())

  // ─── Scan state ───
  let jobId = $state(null)
  let status = $state('idle') // idle | running | done | cancelled
  let progressPct = $state(0)
  let progressText = $state('')
  let startTime = 0
  let statusStop = null
  let resultsTimer = null
  let rangesFileName = $state('')

  const data = $derived($cleanData)
  const scanDesc = $derived(useConfig ? $_('clean.descTwoPhase') : $_('clean.descOnePhase'))
  const hasResults = $derived((data?.entries?.length || 0) > 0 || (data?.nearby_entries?.length || 0) > 0)
  const startDisabled = $derived(status === 'running' || (useConfig && !vlessURL.trim()) || (source === 'custom' && !customRanges.trim()))

  const matchFilter = (e) => {
    const q = coloFilter.trim().toLowerCase()
    const maxLat = parseInt(maxLatency) || 0
    return (!q || ((e.colo || '') + ' ' + (e.loc || '')).toLowerCase().includes(q)) &&
      (!maxLat || parseLatency(e.latency) <= maxLat)
  }

  function limitPool(entries) {
    let p = sortEntries(entries.filter(matchFilter), sort.field, sort.dir)
    const oc = parseInt(outCount) || 0
    if (oc > 0 && p.length > oc) p = p.slice(0, oc)
    return p
  }
  const directPool = $derived(limitPool(data?.entries || []))
  const nearbyPool = $derived(limitPool(data?.nearby_entries || []))
  const nearbyAll = $derived(data?.nearby_entries || [])
  const activePool = $derived(list === 'nearby' ? nearbyPool : directPool)
  const isPhase2 = $derived(data?.phase === 'phase2')
  const failReasons = $derived.by(() => {
    const reasons = data?.fail_reasons || {}
    return Object.keys(reasons).sort((a, b) => reasons[b] - reasons[a]).map((k) => ({ k, n: reasons[k] }))
  })
  const failExamples = $derived((data?.failures || []).slice(0, 5))

  function clearTimers() {
    if (statusStop) { statusStop(); statusStop = null }
    clearInterval(resultsTimer); resultsTimer = null
  }

  // ─── Source / ranges ───
  function appendRanges(lines) {
    const existing = new Set(customRanges.split('\n').map((s) => s.trim()).filter(Boolean))
    const fresh = lines.filter((v) => v && !existing.has(v))
    if (!fresh.length) return 0
    const prefix = customRanges && !customRanges.endsWith('\n') ? customRanges + '\n' : customRanges
    customRanges = prefix + fresh.join('\n') + '\n'
    return fresh.length
  }
  function addRangePreset(value) {
    const lines = value === '__cf_v4__' ? CF_V4_RANGES : value === '__cf_v6__' ? CF_V6_RANGES : [value]
    if (appendRanges(lines) === 0) showToast($_('clean.rangeExists'))
  }
  function onRangesFile(ev) {
    const f = ev.currentTarget.files?.[0]
    if (!f) return
    const reader = new FileReader()
    reader.onload = () => {
      appendRanges(String(reader.result || '').split('\n').map((s) => s.trim()).filter(Boolean))
      rangesFileName = f.name
      showToast($_('clean.rangesLoaded'))
    }
    reader.readAsText(f)
    ev.currentTarget.value = ''
  }

  // ─── Ports ───
  function togglePort(p, on) {
    const s = new Set(ports)
    if (on) s.add(p); else s.delete(p)
    ports = [...s]
  }
  function portPreset(mode) {
    if (mode === 'all') ports = [...HTTPS_PORTS, ...HTTP_PORTS]
    else if (mode === 'https') ports = [...HTTPS_PORTS]
    else if (mode === 'http') ports = [...HTTP_PORTS]
    else if (mode === '443') ports = [443]
    else if (mode === 'config') {
      const m = vlessURL.trim().match(/@[^@]+:(\d+)/)
      if (!m) { showToast($_('settings.portConfigNoURL'), true); return }
      ports = [+m[1]]
    }
  }

  // ─── Scan lifecycle ───
  async function startScan() {
    const onePhase = !useConfig
    if (!onePhase && !vlessURL.trim()) { showToast($_('clean.errNoURL'), true); return }
    if (source === 'custom' && !customRanges.trim()) { showToast($_('clean.errNoRanges'), true); return }

    jobId = null
    status = 'running'
    progressPct = 0
    progressText = $_('clean.progressPhase1', { values: { p: 0, t: 0 } })
    selected = new Set()
    list = 'direct'
    cleanData.set(null)

    let depth = parseInt(scanDepth)
    if (scanDepth === '0') { depth = parseInt(customCount) || 500; if (depth < 1) depth = 500 }
    const scanPorts = ports.length ? ports : [443]

    try {
      const resp = await apiJSON('/api/clean-scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          vless_url: onePhase ? '' : vlessURL.trim(),
          count: depth,
          ipv4: ipVersion === '4' || ipVersion === '46',
          ipv6: ipVersion === '6' || ipVersion === '46',
          phase2_count: parseInt(phase2Count),
          one_phase: onePhase,
          nearby_scan: nearby,
          nearby_count: 10,
          phase1_probes: parseInt(phase1Probes),
          phase2_probes: parseInt(phase2Probes),
          timeout_ms: parseInt(timeout1) || 0,
          phase2_timeout_ms: parseInt(timeout2) || 0,
          port_mode: 'custom',
          ports: scanPorts,
          custom_ranges: source === 'custom' ? customRanges.trim() : '',
          stop_after: parseInt(stopAfter) || 0,
        }),
      })
      jobId = resp.id
      startTime = Date.now()
      pollStatus(resp.id)
      pollResults(resp.id)
    } catch (e) {
      showToast($_('clean.errStart', { values: { msg: e.message } }), true)
      status = 'idle'
    }
  }

  function pollStatus(id) {
    if (statusStop) statusStop()
    // Status only — pollResults below keeps fetching cleanData and self-stops
    // once it sees the terminal status, so this handler must not touch it.
    statusStop = subscribeStatus('/api/clean-events/' + id, '/api/clean-status/' + id, {
      onStatus(d) {
        if (d.status === 'running-phase1') {
          const p = d.phase1_progress || 0, tot = d.phase1_total || 1
          progressPct = Math.round((p / tot) * 50)
          progressText = $_('clean.progressPhase1', { values: { p, t: tot } }) +
            scanRateText(p, tot, startTime, (s) => $_('scan.eta', { values: { s } }))
        } else if (d.status === 'running-phase2') {
          const p = d.phase2_progress || 0, tot = d.phase2_total || 1
          progressPct = Math.min(50 + Math.round((p / tot) * 50), 99)
          progressText = $_('clean.progressPhase2', { values: { p, t: tot } })
        } else if (d.status === 'done') {
          progressPct = 100
          progressText = $_('clean.progressDone')
        } else if (d.status === 'cancelled') {
          progressText = $_('clean.progressCancelled')
        }
        if (d.status === 'done' || d.status === 'cancelled') {
          status = d.status
          if (notify) notifyDone($_('notify.title'), $_('notify.cleanBody', { values: { n: data?.entries?.length || 0 } }))
        }
      },
      isDone: (d) => d.status === 'done' || d.status === 'cancelled',
    })
  }

  function pollResults(id) {
    clearInterval(resultsTimer)
    resultsTimer = setInterval(async () => {
      try {
        const d = await apiJSON('/api/clean-results/' + id)
        cleanData.set(d)
        if (d.status === 'done' || d.status === 'cancelled') {
          clearInterval(resultsTimer); resultsTimer = null
          status = d.status
        }
      } catch {
        clearInterval(resultsTimer); resultsTimer = null
      }
    }, 800)
  }

  async function stopScan() {
    if (!jobId) return
    try { await fetch('/api/clean-stop/' + jobId, { method: 'POST' }) } catch {}
  }

  function resetAll() {
    clearTimers()
    jobId = null
    status = 'idle'
    progressPct = 0
    progressText = ''
    selected = new Set()
    list = 'direct'
    cleanData.set(null)
  }

  function onSort(field) { sort = toggleSort(sort, field) }
  function sortArrow() { return sort.dir === 'asc' ? '▲' : '▼' }

  function toggleSelect(ep, on) {
    const s = new Set(selected)
    if (on) s.add(ep); else s.delete(ep)
    selected = s
  }
  function selectAll(on) {
    selected = on ? new Set(activePool.map((e) => e.endpoint)) : new Set()
  }

  // ─── Result actions ───
  function curEntries() {
    return (list === 'nearby' ? data?.nearby_entries : data?.entries) || []
  }
  async function copyAll() {
    let entries = curEntries()
    if (list === 'direct' && jobId) {
      try { entries = (await apiJSON('/api/clean-results/' + jobId)).entries || entries } catch {}
    }
    copyToClipboard(formatEps(entries.map((r) => r.endpoint)))
    showToast($_('copied.clipboard'))
  }
  function download() {
    const entries = curEntries()
    const header = list === 'nearby'
      ? '# Nearby Cloudflare Responding IPs\n# Generated by Cloudflare Scanner\n\n'
      : '# Cloudflare Responding IPs\n# Generated by Cloudflare Scanner\n\n'
    downloadText(list === 'nearby' ? 'nearby_responded_ips.txt' : 'responded_ips.txt',
      header + formatEps(entries.map((r) => r.endpoint)) + '\n')
  }
  function qrAll() {
    const text = formatEps(curEntries().map((r) => r.endpoint))
    if (text) showQR(text)
  }
  function copySelected() {
    if (!selected.size) { showToast($_('clean.errNoSelection')); return }
    copyToClipboard(formatEps([...selected]))
    showToast($_('copied.clipboard'))
  }
  function downloadBlob(blob, filename) {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = filename; a.click()
    URL.revokeObjectURL(url)
  }
  async function exportConfigs() {
    const endpoints = list === 'nearby' ? curEntries().map((e) => e.endpoint) : [...selected]
    if (!endpoints.length) { showToast($_('clean.errNoSelection'), true); return }
    try {
      const resp = await fetch('/api/clean-export', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ vless_url: vlessURL.trim(), endpoints }),
      })
      if (!resp.ok) { const err = await resp.json().catch(() => ({})); throw new Error(err.error || resp.statusText) }
      const blob = await resp.blob()
      downloadBlob(blob, list === 'nearby' ? 'nearby_ips_vless.txt' : 'clean_ips_vless.txt')
      showToast($_('clean.exported', { values: { n: endpoints.length } }))
    } catch (e) {
      showToast($_('clean.errExport', { values: { msg: e.message } }), true)
    }
  }
  function pushToReplacer() {
    const endpoints = list === 'nearby' ? curEntries().map((e) => e.endpoint) : [...selected]
    const cleaned = endpoints.map((s) => (s || '').trim()).filter(Boolean)
    if (!cleaned.length) { showToast($_('clean.errNoSelection')); return }
    pendingProxyEndpoints.set(cleaned)
    replacerCtype.set('proxy')
    activeTab.set('replacer')
    showToast($_('clean.pushedToReplacer', { values: { n: cleaned.length } }))
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
    <th class="sortable" onclick={() => onSort('colo')}>{$_('results.tableColo')}{#if sort.field === 'colo'}<span class="sort-icon">{sortArrow()}</span>{/if}</th>
    <th class="checkbox-cell"></th>
  </tr>
{/snippet}

{#snippet row(e, i, measure)}
  <tr data-index={i} use:measure>
    <td class="num">{i + 1}</td>
    <td><span class="tag" onclick={() => { copyToClipboard(e.endpoint); showToast($_('copied.clipboard')) }} title={$_('results.tableEndpoint')}>{e.endpoint}</span></td>
    <td class={latClass(e.latency)}>{e.latency}</td>
    <td class="colo-cell">
      {#if e.colo}<span class="colo-tag" title={e.loc || ''}>{e.colo}{#if e.loc}<span class="colo-loc">{e.loc}</span>{/if}</span>
      {:else}<span class="colo-empty">—</span>{/if}
    </td>
    <td class="checkbox-cell"><input type="checkbox" checked={selected.has(e.endpoint)} onchange={(ev) => toggleSelect(e.endpoint, ev.currentTarget.checked)} /></td>
  </tr>
{/snippet}

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
          <label class="toggle" title={$_('clean.useConfigTitle')} aria-label="Toggle config usage">
            <input type="checkbox" bind:checked={useConfig} />
            <span class="slider"></span>
          </label>
          <span class="toggle-label">{$_('clean.useConfig')}</span>
        </div>
        <div class:config-fields-disabled={!useConfig}>
          <label for="vlessURL" title={$_('clean.vlessTitle')}>{$_('clean.vlessLabel')}</label>
          <input type="text" id="vlessURL" bind:value={vlessURL} disabled={!useConfig} placeholder={$_('clean.vlessPlaceholder')} title={$_('clean.vlessTitle')} />
        </div>
      </div>
    </div>
  </div>

  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
      <span>{$_('settings.header')}</span>
    </h2>

    <label title={$_('clean.sourceTitle')}>{$_('clean.sourceLabel')}</label>
    <div class="input-method-bar" style="margin-bottom:var(--space-md)">
      <button class:active={source === 'pool'} onclick={() => (source = 'pool')} title={$_('clean.sourceTitle')}>{$_('clean.sourcePool')}</button>
      <button class:active={source === 'custom'} onclick={() => (source = 'custom')} title={$_('clean.sourceTitle')}>{$_('clean.sourceCustom')}</button>
    </div>

    {#if source === 'custom'}
      <div style="margin-bottom:var(--space-md)">
        <div class="preset-bar" style="margin-bottom:8px">
          {#each RANGE_PRESETS as r}
            <button class="preset-btn" onclick={() => addRangePreset(r)}>{r}</button>
          {/each}
          <button class="preset-btn" onclick={() => addRangePreset('__cf_v4__')}>{$_('clean.presetAllV4')}</button>
          <button class="preset-btn" onclick={() => addRangePreset('__cf_v6__')}>{$_('clean.presetAllV6')}</button>
        </div>
        <textarea rows="4" bind:value={customRanges} placeholder={$_('clean.customPlaceholder')}></textarea>
        <label class="file-input-wrap" for="cleanRangesFile" style="margin-top:8px">
          <input type="file" id="cleanRangesFile" accept=".txt,.csv,.list,text/plain" onchange={onRangesFile} />
          <div class="file-label" class:selected={!!rangesFileName}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="width:16px;height:16px;flex-shrink:0"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
            <span>{rangesFileName || $_('clean.rangesFile')}</span>
          </div>
        </label>
        <details class="scan-desc" style="margin-top:8px">
          <summary style="cursor:pointer">{$_('clean.rangesHelpSummary')}</summary>
          <div style="margin-top:6px">{$_('clean.rangesHelp')}</div>
        </details>
      </div>
    {/if}

    <div class="row">
      <div class="col">
        <label title={$_('clean.depthTitle')}>{$_('settings.scanDepth')}</label>
        <div class="preset-bar">
          {#each DEPTHS as d}
            <button class="preset-btn" class:active={scanDepth === d.v} onclick={() => (scanDepth = d.v)}>{$_(d.k)}</button>
          {/each}
        </div>
        {#if scanDepth === '0'}
          <div class="status-slot">
            <input type="text" bind:value={customCount} placeholder={$_('settings.customPlaceholder')} inputmode="numeric" />
          </div>
        {/if}
      </div>
      <div class="col">
        <label for="cleanIPVersion" title={$_('clean.ipTitle')}>{$_('settings.ipVersion')}</label>
        <select id="cleanIPVersion" bind:value={ipVersion} title={$_('clean.ipTitle')}>
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
          <label for="phase1Probes" title={$_('clean.probesTitle')}>{$_('clean.probesLabel')}</label>
          <select id="phase1Probes" bind:value={phase1Probes} title={$_('clean.probesTitle')}>
            {#each ['100', '250', '500', '1000', '2000'] as v}<option value={v}>{v}</option>{/each}
          </select>
        </div>
        <div class="col">
          <label for="phase2Probes" title={$_('clean.phase2Title')}>{$_('clean.phase2ProbesLabel')}</label>
          <select id="phase2Probes" bind:value={phase2Probes} title={$_('clean.phase2Title')}>
            {#each ['5', '12', '25', '50', '100'] as v}<option value={v}>{v}</option>{/each}
          </select>
        </div>
        {#if useConfig}
          <div class="col">
            <label for="cleanPhase2" title={$_('clean.phase2Title')}>{$_('clean.phase2Label')}</label>
            <select id="cleanPhase2" bind:value={phase2Count} title={$_('clean.phase2Title')}>
              {#each ['10', '20', '30', '50'] as v}<option value={v}>{v}</option>{/each}
            </select>
          </div>
        {/if}
      </div>

      <div style="margin:var(--space-sm) 0">
        <label style="display:block;margin-bottom:6px">{$_('settings.portMode')}</label>
        <div class="preset-bar" style="margin-bottom:10px">
          <button class="preset-btn" onclick={() => portPreset('443')}>{$_('settings.portPreset443')}</button>
          <button class="preset-btn" onclick={() => portPreset('https')}>{$_('settings.portPresetHttps')}</button>
          <button class="preset-btn" onclick={() => portPreset('http')}>{$_('settings.portPresetHttp')}</button>
          <button class="preset-btn" onclick={() => portPreset('all')}>{$_('settings.portPresetAll')}</button>
          <button class="preset-btn" onclick={() => portPreset('config')}>{$_('settings.portPresetConfig')}</button>
        </div>
        <div class="port-section-label">{$_('settings.portHttps')}</div>
        <div class="port-grid">
          {#each HTTPS_PORTS as p}
            <label class="port-cb-label"><input type="checkbox" checked={ports.includes(p)} onchange={(e) => togglePort(p, e.currentTarget.checked)} /> {p}</label>
          {/each}
        </div>
        <div class="port-section-label">{$_('settings.portHttp')}</div>
        <div class="port-grid">
          {#each HTTP_PORTS as p}
            <label class="port-cb-label"><input type="checkbox" checked={ports.includes(p)} onchange={(e) => togglePort(p, e.currentTarget.checked)} /> {p}</label>
          {/each}
        </div>
      </div>

      <div class="row">
        <div class="col">
          <div class="toggle-wrap">
            <label class="toggle" title={$_('clean.nearbyTitle')} aria-label="Toggle nearby scan">
              <input type="checkbox" bind:checked={nearby} />
              <span class="slider"></span>
            </label>
            <span class="toggle-label" title={$_('clean.nearbyTitle')}>{$_('clean.nearby')}</span>
          </div>
        </div>
        <div class="col">
          <label for="cleanTimeout" title={$_('clean.timeoutTitle')}>{$_('clean.timeout1Label')}</label>
          <input id="cleanTimeout" type="text" bind:value={timeout1} inputmode="numeric" title={$_('clean.timeoutTitle')} />
        </div>
        {#if useConfig}
          <div class="col">
            <label for="cleanPhase2Timeout" title={$_('clean.timeout2Title')}>{$_('clean.timeout2Label')}</label>
            <input id="cleanPhase2Timeout" type="text" bind:value={timeout2} inputmode="numeric" title={$_('clean.timeout2Title')} />
          </div>
        {/if}
      </div>
      <div class="row">
        <div class="col">
          <label for="cleanStopAfter" title={$_('clean.stopAfterTitle')}>{$_('clean.stopAfter')}</label>
          <input id="cleanStopAfter" type="text" bind:value={stopAfter} inputmode="numeric" title={$_('clean.stopAfterTitle')} />
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
    <button class="btn btn-primary" onclick={startScan} disabled={startDisabled} title={$_('clean.startTitle')}>
      {status === 'running' ? $_('clean.scanning') : $_('clean.start')}
    </button>
    <button class="btn btn-danger" onclick={stopScan} disabled={status !== 'running'} title={$_('clean.stopTitle')}>{$_('buttons.stop')}</button>
    <button class="btn btn-secondary" onclick={startScan} disabled={status === 'running' || !hasResults} title={$_('buttons.rescanTitle')}>{$_('buttons.rescan')}</button>
    <button class="btn btn-secondary" onclick={resetAll} title={$_('buttons.resetTitle')}>{$_('buttons.reset')}</button>
  </div>

  <div class="card" id="cleanResultsCard">
    <div class="section-header">
      <h2>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 3v18h18"/><path d="m19 9-5 5-4-4-3 3"/></svg>
        <span>{$_('results.header')}</span>
      </h2>
      <div style="display:flex;gap:var(--space-md);align-items:center;flex-wrap:wrap">
        <div class="compact-control">
          <label for="cleanColoFilter" title={$_('clean.coloFilterTitle')}>{$_('clean.coloFilter')}</label>
          <input class="compact-input" id="cleanColoFilter" type="text" bind:value={coloFilter} style="width:84px;text-align:left;font-family:var(--font-sans)" placeholder={$_('clean.coloFilterPh')} title={$_('clean.coloFilterTitle')} />
        </div>
        <div class="compact-control">
          <label for="cleanMaxLatency" title={$_('results.maxLatTitle')}>{$_('results.maxLat')}</label>
          <input class="compact-input" id="cleanMaxLatency" type="text" bind:value={maxLatency} title={$_('results.maxLatTitle')} inputmode="numeric" />
        </div>
        <div class="compact-control">
          <label for="cleanOutCount" title={$_('settings.outCountTitle')}>{$_('settings.outCount')}</label>
          <input class="compact-input" id="cleanOutCount" type="text" bind:value={outCount} title={$_('settings.outCountTitle')} inputmode="numeric" />
        </div>
      </div>
    </div>

    {#if status !== 'idle'}
      <div class="progress-wrap active">
        <div class="progress-bar"><div class="progress-fill" class:cancelled={status === 'cancelled'} style="width:{progressPct}%"></div></div>
        <div class="progress-text">{progressText}</div>
      </div>
    {/if}

    {#if nearbyAll.length > 0}
      <div class="btn-bar" style="margin-top:10px;margin-bottom:4px">
        <button class="btn btn-sm method-btn" class:active={list === 'direct'} onclick={() => (list = 'direct')}>{$_('clean.listDirect')}</button>
        <button class="btn btn-sm method-btn" class:active={list === 'nearby'} onclick={() => (list = 'nearby')}>
          {$_('clean.listNearby')} <span style="color:var(--text-secondary);font-size:0.625rem">({nearbyAll.length})</span>
        </button>
      </div>
    {/if}

    {#if hasResults}
      <div class="results-actions results-actions-top" style="margin-top:4px">
        <SplitCopyButton oncopy={copyAll} title={$_('clean.copyAllTitle')} />
        <button class="btn btn-secondary btn-sm" onclick={download} title={$_('results.downloadTitle')}>{$_('results.downloadRaw')}</button>
        <button class="btn btn-secondary btn-sm" onclick={qrAll} title="QR">QR</button>
        {#if isPhase2}
          <button class="btn btn-accent btn-sm" onclick={exportConfigs}>{$_('clean.export')}</button>
        {/if}
        <button class="btn btn-accent btn-sm" onclick={pushToReplacer}>{$_('clean.pushToReplacer')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(true)}>{$_('clean.selectAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => selectAll(false)}>{$_('clean.deselectAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={copySelected} title={$_('clean.copySelectedTitle')}>{$_('clean.copySelected')}</button>
      </div>

      {#if status === 'done' || status === 'cancelled'}
        <div class={status === 'cancelled' ? 'error-msg' : 'success-msg'} style="margin-bottom:6px">
          {#if status === 'cancelled'}{$_('clean.progressCancelled')}
          {:else if list === 'nearby'}{isPhase2 ? $_('clean.foundNearbyPhase2', { values: { n: nearbyPool.length } }) : $_('clean.foundNearby', { values: { n: nearbyPool.length } })}
          {:else if isPhase2}{$_('clean.foundPhase2', { values: { n: data?.total, s: data?.scanned } })}
          {:else}{$_('clean.foundPhase1', { values: { n: data?.total, t: data?.phase1_total } })}{/if}
        </div>
      {/if}

      <VirtualTable items={activePool} getKey={(e) => e.endpoint} colspan={5} {header} {row} />

      {#if isPhase2 && failReasons.length > 0}
        <div class="fail-panel">
          <div class="fail-title">{$_('clean.whyFailed')}</div>
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
    {:else if status === 'done' && isPhase2}
      <div class="empty-state">
        <p>{$_('clean.noResults')}{#if data?.scanned > 0 && data?.phase2_failures > 0} ({data.scanned} {$_('clean.testedAllFailed')}){/if}</p>
      </div>
      {#if failReasons.length > 0}
        <div class="fail-panel">
          <div class="fail-title">{$_('clean.whyFailed')}</div>
          <ul class="fail-list">
            {#each failReasons as r}<li><span class="fail-count">{r.n}×</span> {r.k}</li>{/each}
          </ul>
        </div>
      {/if}
    {/if}
  </div>

  <div class="card">
    <details class="help-panel">
      <summary>{$_('cleanHelp.header')}</summary>
      <p class="desc" style="margin-top:8px">{$_('cleanHelp.intro')}</p>
      <div class="help-list">
        <div>{@html $_('cleanHelp.p1')}</div>
        <div>{@html $_('cleanHelp.p2')}</div>
        <div>{@html $_('cleanHelp.p3')}</div>
        <div>{@html $_('cleanHelp.p4')}</div>
        <div>{@html $_('cleanHelp.p5')}</div>
      </div>
      <p class="desc">{$_('cleanHelp.tip')}</p>
    </details>
  </div>
</div>
