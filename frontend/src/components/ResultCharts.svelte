<script>
  import { _ } from 'svelte-i18n'
  import { parseLatency } from '../lib/sort.js'

  // Pure-SVG result visualizations — no chart dependency, styled with the global
  // design tokens. Shared by both scanner tabs. Renders nothing until there are
  // results. `showColo` lets the WARP tab hide the colo chart it has no data for.
  let { entries = [], showColo = true } = $props()

  const LAT_BUCKETS = [
    { max: 50, label: '<50' },
    { max: 100, label: '50–100' },
    { max: 150, label: '100–150' },
    { max: 200, label: '150–200' },
    { max: 300, label: '200–300' },
    { max: 500, label: '300–500' },
    { max: Infinity, label: '500+' },
  ]

  const latHist = $derived.by(() => {
    const counts = LAT_BUCKETS.map(() => 0)
    for (const e of entries) {
      const v = parseLatency(e.latency)
      if (!isFinite(v)) continue
      for (let i = 0; i < LAT_BUCKETS.length; i++) {
        if (v < LAT_BUCKETS[i].max) { counts[i]++; break }
      }
    }
    const max = Math.max(1, ...counts)
    return LAT_BUCKETS.map((b, i) => ({ label: b.label, n: counts[i], pct: counts[i] / max }))
  })

  const coloBars = $derived.by(() => {
    if (!showColo) return []
    const m = new Map()
    for (const e of entries) {
      const c = (e.colo || '').trim()
      if (!c) continue
      m.set(c, (m.get(c) || 0) + 1)
    }
    const arr = [...m.entries()].map(([colo, n]) => ({ colo, n }))
    arr.sort((a, b) => b.n - a.n)
    const top = arr.slice(0, 8)
    const max = Math.max(1, ...top.map((x) => x.n))
    return top.map((x) => ({ ...x, pct: x.n / max }))
  })

  // Quality mix: how the result set splits into good / ok / poor by score.
  const quality = $derived.by(() => {
    let good = 0, ok = 0, poor = 0, scored = 0
    for (const e of entries) {
      const s = Number(e.score)
      if (!isFinite(s) || s <= 0) continue
      scored++
      if (s >= 75) good++
      else if (s >= 50) ok++
      else poor++
    }
    return { good, ok, poor, scored }
  })

  const hasColo = $derived(coloBars.length > 0)
  const show = $derived(entries.length >= 3)
</script>

{#if show}
  <div class="charts">
    <div class="chart-card">
      <div class="chart-title">{$_('charts.latency')}</div>
      <div class="hist">
        {#each latHist as b}
          <div class="hist-col" title={`${b.label} ms · ${b.n}`}>
            <div class="hist-bar-wrap">
              <div class="hist-bar" style="height:{Math.max(3, b.pct * 100)}%"></div>
            </div>
            <div class="hist-n">{b.n || ''}</div>
            <div class="hist-x">{b.label}</div>
          </div>
        {/each}
      </div>
    </div>

    {#if quality.scored > 0}
      <div class="chart-card">
        <div class="chart-title">{$_('charts.quality')}</div>
        <div class="qbar">
          {#if quality.good}<div class="qseg q-good" style="flex:{quality.good}" title={`${$_('charts.good')}: ${quality.good}`}></div>{/if}
          {#if quality.ok}<div class="qseg q-ok" style="flex:{quality.ok}" title={`${$_('charts.ok')}: ${quality.ok}`}></div>{/if}
          {#if quality.poor}<div class="qseg q-poor" style="flex:{quality.poor}" title={`${$_('charts.poor')}: ${quality.poor}`}></div>{/if}
        </div>
        <div class="qlegend">
          <span><i class="dot q-good"></i>{$_('charts.good')} {quality.good}</span>
          <span><i class="dot q-ok"></i>{$_('charts.ok')} {quality.ok}</span>
          <span><i class="dot q-poor"></i>{$_('charts.poor')} {quality.poor}</span>
        </div>
      </div>
    {/if}

    {#if hasColo}
      <div class="chart-card">
        <div class="chart-title">{$_('charts.colo')}</div>
        <div class="colo-rows">
          {#each coloBars as c}
            <div class="colo-row">
              <span class="colo-name">{c.colo}</span>
              <span class="colo-track"><span class="colo-fill" style="width:{Math.max(4, c.pct * 100)}%"></span></span>
              <span class="colo-n">{c.n}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .charts {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--space-sm);
    margin: var(--space-sm) 0 var(--space-md);
  }
  @media (min-width: 720px) {
    .charts { grid-template-columns: 1.3fr 0.7fr 1fr; }
  }
  .chart-card {
    background: var(--bg-input);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    padding: 10px 12px;
    min-width: 0;
  }
  .chart-title {
    font-size: 0.625rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 700;
    color: var(--text-tertiary);
    margin-bottom: 8px;
  }
  /* Latency histogram */
  .hist { display: flex; align-items: flex-end; gap: 4px; height: 92px; }
  .hist-col { flex: 1; display: flex; flex-direction: column; align-items: center; min-width: 0; }
  .hist-bar-wrap { width: 100%; height: 64px; display: flex; align-items: flex-end; }
  .hist-bar {
    width: 100%;
    border-radius: 3px 3px 0 0;
    background: linear-gradient(180deg, var(--accent-2), var(--accent));
    box-shadow: 0 0 8px var(--accent-glow);
    transition: height 0.3s var(--ease);
  }
  .hist-n { font-size: 0.625rem; color: var(--text-secondary); font-variant-numeric: tabular-nums; height: 12px; }
  .hist-x { font-size: 0.5rem; color: var(--text-tertiary); white-space: nowrap; transform: scale(0.92); }
  /* Quality mix bar */
  .qbar { display: flex; height: 16px; border-radius: var(--radius-full); overflow: hidden; background: rgba(255,255,255,0.05); }
  .qseg { height: 100%; }
  .q-good { background: var(--success); }
  .q-ok { background: var(--warning); }
  .q-poor { background: var(--danger); }
  .qlegend { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; font-size: 0.625rem; color: var(--text-secondary); }
  .qlegend span { display: inline-flex; align-items: center; gap: 4px; font-variant-numeric: tabular-nums; }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
  /* Colo distribution */
  .colo-rows { display: flex; flex-direction: column; gap: 5px; }
  .colo-row { display: grid; grid-template-columns: 34px 1fr 22px; align-items: center; gap: 6px; }
  .colo-name { font-family: var(--font-mono); font-size: 0.625rem; font-weight: 700; color: var(--accent-2); }
  .colo-track { height: 7px; border-radius: var(--radius-full); background: rgba(255,255,255,0.06); overflow: hidden; }
  .colo-fill { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--accent), var(--accent-2)); }
  .colo-n { font-size: 0.625rem; color: var(--text-secondary); text-align: right; font-variant-numeric: tabular-nums; }
</style>
