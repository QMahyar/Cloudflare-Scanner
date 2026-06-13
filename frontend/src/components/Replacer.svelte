<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON } from '../lib/api.js'
  import { copyToClipboard, downloadText } from '../lib/clipboard.js'
  import { sortEntries, toggleSort } from '../lib/sort.js'
  import { activateKey } from '../lib/a11y.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'
  import { replacerGenerated } from '../lib/stores.js'
  import { pendingWarpEndpoint, pendingProxyEndpoints, replacerCtype } from '../lib/handoff.js'

  // ─── Config type (proxy share-URLs vs WireGuard .conf) ───
  let ctype = $state('proxy')
  // Consume cross-tab handoff: "Use" on a WARP endpoint switches to warp mode.
  $effect(() => {
    const c = $replacerCtype
    if (c) { ctype = c; replacerCtype.set(null) }
  })

  // ═══════════════════════════ Proxy mode ═══════════════════════════
  let method = $state('url') // url | paste
  let subURL = $state('')
  let rawText = $state('')
  let fetching = $state(false)
  let parsing = $state(false)
  let fetchStatus = $state(null) // {ok, msg}
  let parseStatus = $state(null)

  let configs = $state([])
  let cfgSelected = $state(new Set())
  let cfgSort = $state({ field: 'num', dir: 'asc' })

  let endpointsText = $state('')
  let nameTemplate = $state('')
  let generating = $state(false)
  let genStatus = $state(null)
  let genCount = $state(0)

  // Consume pushed proxy endpoints from the IP scanner.
  $effect(() => {
    const eps = $pendingProxyEndpoints
    if (eps && eps.length) {
      const existing = endpointsText.split('\n').map((s) => s.trim()).filter(Boolean)
      endpointsText = [...new Set(existing.concat(eps))].join('\n')
      pendingProxyEndpoints.set(null)
    }
  })

  const cfgPool = $derived(sortEntries(configs, cfgSort.field, cfgSort.dir))

  function onCfgSort(field) { cfgSort = toggleSort(cfgSort, field) }
  function cfgArrow() { return cfgSort.dir === 'asc' ? '▲' : '▼' }

  function toggleCfg(idx, on) {
    const s = new Set(cfgSelected)
    if (on) s.add(idx); else s.delete(idx)
    cfgSelected = s
  }
  function cfgSelectAll(on) {
    cfgSelected = on ? new Set(configs.map((_, i) => i)) : new Set()
  }

  async function doFetch() {
    if (!subURL.trim()) { fetchStatus = { ok: false, msg: $_('replacer.errNoURL') }; return }
    fetching = true; fetchStatus = null
    try {
      const data = await apiJSON('/api/replacer/fetch', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: subURL.trim() }),
      })
      loadConfigs(data.configs)
      fetchStatus = { ok: true, msg: $_('replacer.fetched', { values: { t: data.total, u: data.unique } }) }
    } catch (e) {
      fetchStatus = { ok: false, msg: $_('replacer.errFetch', { values: { msg: e.message } }) }
    }
    fetching = false
  }

  async function doParse() {
    if (!rawText.trim()) { parseStatus = { ok: false, msg: $_('replacer.errNoRaw') }; return }
    parsing = true; parseStatus = null
    try {
      const data = await apiJSON('/api/replacer/parse', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ raw: rawText.trim() }),
      })
      loadConfigs(data.configs)
      parseStatus = { ok: true, msg: $_('replacer.parsed', { values: { t: data.total, u: data.unique } }) }
    } catch (e) {
      parseStatus = { ok: false, msg: $_('replacer.errParse', { values: { msg: e.message } }) }
    }
    parsing = false
  }

  function loadConfigs(list) {
    configs = list || []
    cfgSelected = new Set(configs.map((_, i) => i)) // all selected by default
  }

  // Live endpoint validation (host:port, IPv4/IPv6, port range).
  function isValidEndpoint(s) {
    let host, port
    if (s[0] === '[') {
      const i = s.indexOf(']:')
      if (i < 0) return false
      host = s.slice(1, i); port = s.slice(i + 2)
      if (host.indexOf(':') < 0) return false
    } else {
      const c = s.lastIndexOf(':')
      if (c < 0 || s.indexOf(':') !== c) return false
      host = s.slice(0, c); port = s.slice(c + 1)
    }
    const p = parseInt(port, 10)
    return !!host && p >= 1 && p <= 65535
  }
  const epHint = $derived.by(() => {
    const lines = endpointsText.split('\n').map((s) => s.trim()).filter(Boolean)
    if (!lines.length) return null
    const seen = new Set()
    let dupes = 0
    const bad = []
    for (const l of lines) {
      if (seen.has(l)) { dupes++; continue }
      seen.add(l)
      if (!isValidEndpoint(l)) bad.push(l)
    }
    const ok = seen.size - bad.length
    let msg = $_('replacer.epValid', { values: { ok, total: lines.length } })
    if (bad.length) msg += ' · ' + $_('replacer.epInvalid', { values: { n: bad.length, ex: bad[0] } })
    if (dupes) msg += ' · ' + $_('replacer.epDupes', { values: { n: dupes } })
    return { warn: bad.length > 0, msg }
  })

  async function generate() {
    const selected = [...cfgSelected].map((i) => configs[i]).filter(Boolean)
    const endpoints = endpointsText.split('\n').map((s) => s.trim()).filter(Boolean)
    if (!selected.length) { genStatus = { ok: false, msg: $_('replacer.errNoConfigs') }; return }
    if (!endpoints.length) { genStatus = { ok: false, msg: $_('replacer.errNoEndpoints') }; return }
    generating = true; genStatus = null
    try {
      const data = await apiJSON('/api/replacer/apply', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ configs: selected, endpoints, name_template: nameTemplate.trim() }),
      })
      replacerGenerated.set(data.urls || [])
      subscription = data.subscription || ''
      genCount = selected.length
      genCountEp = endpoints.length
      genStatus = { ok: true, msg: $_('replacer.generated', { values: { n: data.count } }) }
    } catch (e) {
      genStatus = { ok: false, msg: $_('replacer.errGenerate', { values: { msg: e.message } }) }
    }
    generating = false
  }

  // ─── Generated results ───
  let subscription = $state('')
  let genCountEp = $state(0)
  const generated = $derived($replacerGenerated)

  function subText() {
    return subscription || btoa(unescape(encodeURIComponent(generated.join('\n'))))
  }
  function copyRow(u) { copyToClipboard(u); showToast($_('copied.clipboard')) }
  function rowQR(u) { showQR(u) }
  function copyAllUrls() { copyToClipboard(generated.join('\n')); showToast($_('copied.clipboard')) }
  function copySub() { if (generated.length) { copyToClipboard(subText()); showToast($_('copied.clipboard')) } }
  function downloadUrls() {
    downloadText('replaced_configs.txt', '# Replaced VLESS Configs\n# Generated by Cloudflare Scanner\n\n' + generated.join('\n') + '\n')
  }
  function downloadSub() { if (generated.length) downloadText('subscription.txt', subText()) }

  // ═══════════════════════════ WireGuard mode ═══════════════════════════
  let applyEndpoint = $state('')
  let applyFiles = $state(null)
  let outputDir = $state('')
  let applying = $state(false)
  let applyStatus = $state(null) // {ok, msg}
  let applyResults = $state([])

  // Consume pushed WARP endpoint from the endpoint scanner.
  $effect(() => {
    const ep = $pendingWarpEndpoint
    if (ep) { applyEndpoint = ep; pendingWarpEndpoint.set(null) }
  })

  const applyFileCount = $derived(applyFiles ? applyFiles.length : 0)
  const applyDisabled = $derived(applying || !applyEndpoint.trim() || applyFileCount === 0)
  const goodResults = $derived(applyResults.filter((r) => !r.error))
  const failedResults = $derived(applyResults.filter((r) => r.error))

  async function browseOutput() {
    try {
      const data = await apiJSON('/api/select-output-dir')
      if (data?.path) { outputDir = data.path; return }
      if (data?.cancelled) return
    } catch {}
    showToast($_('apply.browseFallback'), true)
  }

  async function doApply() {
    if (applyDisabled) return
    applying = true; applyStatus = null; applyResults = []
    const fd = new FormData()
    fd.append('endpoint', applyEndpoint.trim())
    fd.append('output_dir', outputDir.trim())
    for (const f of applyFiles) fd.append('configs', f)
    try {
      const data = await apiJSON('/api/apply-endpoint', { method: 'POST', body: fd })
      applyResults = data.results || []
      applyStatus = { ok: true, msg: $_('apply.generated', { values: { s: data.saved, t: data.total } }) }
    } catch (e) {
      applyStatus = { ok: false, msg: e.message }
    }
    applying = false
  }

  function copyApply(r) { if (r.content) { copyToClipboard(r.content); showToast($_('copied.clipboard')) } }
  function applyQR(r) { if (r.content) showQR(r.content) }
  function copyAllApply() {
    const items = goodResults.filter((r) => r.content)
    if (!items.length) return
    copyToClipboard(items.map((r) => '# ' + r.name + '\n' + r.content).join('\n\n'))
    showToast($_('copied.clipboard'))
  }
  function downloadApply() {
    const items = goodResults.filter((r) => r.content)
    if (!items.length) return
    downloadText('modified_warp_configs.txt',
      '# Modified WireGuard Configs\n# Generated by Cloudflare Scanner\n\n' +
      items.map((r) => '# ' + r.name + '\n' + r.content).join('\n\n') + '\n')
  }
</script>

<div class="input-method-bar" style="margin-bottom:var(--space-md)">
  <button class="method-btn" class:active={ctype === 'proxy'} onclick={() => (ctype = 'proxy')} title={$_('replacer.typeProxyTitle')}>{$_('replacer.typeProxy')}</button>
  <button class="method-btn" class:active={ctype === 'warp'} onclick={() => (ctype = 'warp')} title={$_('replacer.typeWarpTitle')}>{$_('replacer.typeWarp')}</button>
</div>

{#if ctype === 'proxy'}
  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
      <span>{$_('replacer.header')}</span>
    </h2>
    <p class="desc">{$_('replacer.desc')}</p>
    <div class="input-method-bar">
      <button class="method-btn" class:active={method === 'url'} onclick={() => (method = 'url')}>{$_('replacer.methodURL')}</button>
      <button class="method-btn" class:active={method === 'paste'} onclick={() => (method = 'paste')}>{$_('replacer.methodPaste')}</button>
    </div>

    {#if method === 'url'}
      <div class="row">
        <div class="col" style="flex:3">
          <label for="replacerURL" title={$_('replacer.urlTitle')}>{$_('replacer.subLabel')}</label>
          <input type="text" id="replacerURL" bind:value={subURL} placeholder={$_('replacer.subPlaceholder')} title={$_('replacer.urlTitle')} onkeydown={(e) => e.key === 'Enter' && doFetch()} />
        </div>
        <div class="col" style="flex:none;min-width:auto">
          <div class="field-label" aria-hidden="true">&nbsp;</div>
          <button class="btn btn-primary" onclick={doFetch} disabled={fetching} title={$_('replacer.fetchTitle')}>{fetching ? $_('replacer.fetching') : $_('replacer.fetch')}</button>
        </div>
      </div>
      {#if fetchStatus}<div class="status-slot"><div class={fetchStatus.ok ? 'success-msg' : 'error-msg'}>{fetchStatus.msg}</div></div>{/if}
    {:else}
      <textarea bind:value={rawText} rows="3" placeholder={$_('replacer.pastePlaceholder')} title={$_('replacer.pasteTitle')}></textarea>
      <div class="status-slot">
        <button class="btn btn-secondary" onclick={doParse} disabled={parsing} title={$_('replacer.parseTitle')}>{parsing ? $_('replacer.parsing') : $_('replacer.parse')}</button>
        {#if parseStatus}<div class="status-slot"><div class={parseStatus.ok ? 'success-msg' : 'error-msg'}>{parseStatus.msg}</div></div>{/if}
      </div>
    {/if}
  </div>

  {#if configs.length > 0}
    <div class="card">
      <h2>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
        <span>{$_('replacer.configsHeader')}</span>
        <span class="count-chip">{configs.length}</span>
      </h2>
      <p class="desc">{$_('replacer.configCount', { values: { n: configs.length } })}</p>
      <div class="results-table-wrap scrollable-list">
        <table class="results-table">
          <thead>
            <tr>
              <th class="checkbox-cell"><input type="checkbox" checked={cfgSelected.size === configs.length} onchange={(e) => cfgSelectAll(e.currentTarget.checked)} /></th>
              <th class="sortable" onclick={() => onCfgSort('num')}>{$_('results.tableNum')}{#if cfgSort.field === 'num'}<span class="sort-icon">{cfgArrow()}</span>{/if}</th>
              <th class="sortable" onclick={() => onCfgSort('address')}>{$_('results.tableEndpoint')}{#if cfgSort.field === 'address'}<span class="sort-icon">{cfgArrow()}</span>{/if}</th>
              <th class="sortable" onclick={() => onCfgSort('port')}>{$_('replacer.tablePort')}{#if cfgSort.field === 'port'}<span class="sort-icon">{cfgArrow()}</span>{/if}</th>
              <th class="sortable" onclick={() => onCfgSort('remark')}>{$_('replacer.tableConfig')}{#if cfgSort.field === 'remark'}<span class="sort-icon">{cfgArrow()}</span>{/if}</th>
            </tr>
          </thead>
          <tbody>
            {#each cfgPool as c, i (c.fingerprint || i)}
              {@const idx = configs.indexOf(c)}
              <tr>
                <td class="checkbox-cell"><input type="checkbox" checked={cfgSelected.has(idx)} onchange={(e) => toggleCfg(idx, e.currentTarget.checked)} /></td>
                <td class="num">{i + 1}</td>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <td><span class="tag" role="button" tabindex="0" onclick={() => { copyToClipboard(c.address); showToast($_('copied.clipboard')) }} use:activateKey={() => { copyToClipboard(c.address); showToast($_('copied.clipboard')) }}>{c.address}</span></td>
                <td>{c.port}</td>
                <td><span class="replacer-config-remark" title={c.remark || ''}>{c.remark || c.protocol + '://' + (c.uuid || '').substring(0, 8) + '…'}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <div class="status-slot">
        <button class="btn btn-secondary btn-sm" onclick={() => cfgSelectAll(true)} title={$_('replacer.configsSelectTitle')}>{$_('clean.selectAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => cfgSelectAll(false)} title={$_('replacer.configsSelectTitle')}>{$_('clean.deselectAll')}</button>
      </div>
    </div>
  {/if}

  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
      <span>{$_('replacer.endpointsHeader')}</span>
    </h2>
    <p class="desc">{$_('replacer.endpointsDesc')}</p>
    <textarea bind:value={endpointsText} rows="4" placeholder={$_('replacer.endpointsPlaceholder')} title={$_('replacer.endpointsTitle')}></textarea>
    {#if epHint}<div class="muted-inline status-slot" style="display:block"><span style="color:{epHint.warn ? 'var(--warning)' : 'var(--success)'}">{epHint.msg}</span></div>{/if}
    <div class="row" style="margin-top:var(--space-sm)">
      <div class="col">
        <label for="replacerNameTemplate" title={$_('replacer.nameTplTitle')}>{$_('replacer.nameTpl')}</label>
        <input type="text" id="replacerNameTemplate" bind:value={nameTemplate} placeholder={$_('replacer.nameTplPh')} title={$_('replacer.nameTplTitle')} />
        <div class="muted-inline status-slot" style="display:block">{$_('replacer.nameTplHelp')}</div>
      </div>
    </div>
    <div class="status-slot">
      <button class="btn btn-accent" onclick={generate} disabled={generating} title={$_('replacer.generateTitle')}>{generating ? $_('replacer.generating') : $_('replacer.generate')}</button>
    </div>
    {#if genStatus}<div class="status-slot"><div class={genStatus.ok ? 'success-msg' : 'error-msg'}>{genStatus.msg}</div></div>{/if}
  </div>

  {#if generated.length > 0}
    <div class="card">
      <h2>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>
        <span>{$_('replacer.resultsHeader')}</span>
        <span class="count-chip">{generated.length}</span>
      </h2>
      <p class="desc">{$_('replacer.resultsCount', { values: { n: generated.length, c: genCount, e: genCountEp } })}</p>
      <div class="results-table-wrap scrollable-list">
        <table class="results-table">
          <thead><tr><th>#</th><th>{$_('replacer.tableConfig')}</th><th></th></tr></thead>
          <tbody>
            {#each generated as u, i (i)}
              <tr>
                <td class="num">{i + 1}</td>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <td class="mono-break"><span class="tag" role="button" tabindex="0" onclick={() => copyRow(u)} use:activateKey={() => copyRow(u)} title={$_('replacer.copyAllTitle')}>{u.length > 100 ? u.substring(0, 100) + '…' : u}</span></td>
                <td style="white-space:nowrap">
                  <button class="btn btn-secondary btn-sm" onclick={() => copyRow(u)} title={$_('replacer.copyAllTitle')}>{$_('buttons.copy')}</button>
                  <button class="btn btn-secondary btn-sm" onclick={() => rowQR(u)}>QR</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <div class="results-actions status-slot">
        <button class="btn btn-secondary btn-sm" onclick={copyAllUrls} title={$_('replacer.copyAllTitle')}>{$_('replacer.copyAll')}</button>
        <button class="btn btn-secondary btn-sm" onclick={copySub} title={$_('replacer.copySubTitle')}>{$_('replacer.copySub')}</button>
        <button class="btn btn-secondary btn-sm" onclick={downloadUrls} title={$_('replacer.downloadTitle')}>{$_('replacer.download')}</button>
        <button class="btn btn-secondary btn-sm" onclick={downloadSub} title={$_('replacer.downloadSubTitle')}>{$_('replacer.downloadSub')}</button>
      </div>
    </div>
  {/if}
{:else}
  <!-- WireGuard / WARP .conf mode -->
  <div class="card">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
      <span>{$_('apply.header')}</span>
    </h2>
    <p class="desc">{$_('apply.desc')}</p>
    <div class="row">
      <div class="col">
        <label for="applyEndpoint" title={$_('apply.endpointTitle')}>{$_('apply.endpointLabel')}</label>
        <input type="text" id="applyEndpoint" bind:value={applyEndpoint} placeholder={$_('apply.endpointPlaceholder')} title={$_('apply.endpointTitle')} />
      </div>
    </div>
    <div class="row">
      <div class="col">
        <label for="applyConfigs" title={$_('apply.configsTitle')}>{$_('apply.configsLabel')}</label>
        <label class="file-input-wrap" for="applyConfigs" title={$_('apply.configsTitle')}>
          <input type="file" id="applyConfigs" accept=".conf,.txt" multiple bind:files={applyFiles} />
          <div class="file-label" class:selected={applyFileCount > 0}>
            {applyFileCount > 0 ? $_('apply.nConfigs', { values: { n: applyFileCount } }) : $_('apply.chooseConfigs')}
          </div>
        </label>
      </div>
    </div>
    <div class="row">
      <div class="col">
        <label for="outputDirPath" title={$_('apply.outputTitle')}>{$_('apply.outputLabel')}</label>
        <div class="output-path-row">
          <input type="text" id="outputDirPath" bind:value={outputDir} placeholder={$_('apply.outputPlaceholder')} title={$_('apply.outputTitle')} />
          <button class="btn btn-secondary" onclick={browseOutput} title={$_('apply.browseTitle')}>{$_('apply.browse')}</button>
        </div>
      </div>
    </div>
    <button class="btn btn-accent" onclick={doApply} disabled={applyDisabled} title={$_('apply.generateTitle')}>{applying ? $_('apply.generating') : $_('apply.generate')}</button>
    {#if applyStatus}
      <div class="status-slot">
        <div class={applyStatus.ok ? 'success-msg' : 'error-msg'}>{applyStatus.msg}</div>
        {#each failedResults as r}<div class="error-msg" style="margin-top:4px">{r.name}: {r.error}</div>{/each}
      </div>
    {/if}
    {#if goodResults.length > 0}
      <div class="status-slot">
        {#each goodResults as r, i (r.name + i)}
          <div class="apply-config-item">
            <div class="apply-config-item-header">
              <span class="apply-config-item-name">{r.name}</span>
              <div class="apply-config-item-actions">
                {#if r.path}<span style="font-size:0.625rem;color:var(--text-secondary)">{r.path}</span>{/if}
                <button class="btn btn-secondary btn-sm" onclick={() => copyApply(r)} title={$_('results.tableEndpoint')}>{$_('results.copyAll')}</button>
                <button class="btn btn-secondary btn-sm" onclick={() => applyQR(r)} title="QR">QR</button>
              </div>
            </div>
            <textarea class="apply-config-content-box" readonly spellcheck="false" onclick={(e) => e.currentTarget.select()}>{r.content}</textarea>
          </div>
        {/each}
        <div class="results-actions">
          <button class="btn btn-secondary btn-sm" onclick={copyAllApply}>{$_('results.copyAll')}</button>
          <button class="btn btn-secondary btn-sm" onclick={downloadApply}>{$_('results.downloadRaw')}</button>
        </div>
      </div>
    {/if}
  </div>
{/if}

<div class="card">
  <details class="help-panel">
    <summary>{$_('repHelp.header')}</summary>
    <p class="desc" style="margin-top:8px">{$_('repHelp.intro')}</p>
    <div class="help-list">
      <div>{@html $_('repHelp.p1')}</div>
      <div>{@html $_('repHelp.p2')}</div>
      <div>{@html $_('repHelp.p3')}</div>
      <div>{@html $_('repHelp.p4')}</div>
    </div>
    <p class="desc">{$_('repHelp.tip')}</p>
  </details>
</div>
