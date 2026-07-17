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
  /* Hallmark · component: result charts · genre: modern-minimal · theme: design.md */
  .charts {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-xs);
    margin: var(--space-md) 0 var(--space-lg);
  }
  @media (min-width: 720px) {
    .charts { grid-template-columns: minmax(0, 1.3fr) minmax(0, 0.7fr) minmax(0, 1fr); }
  }
  .chart-card {
    background: var(--color-paper-3);
    border: var(--rule-thin) solid var(--color-rule);
    border-radius: var(--radius-control);
    padding: var(--space-sm);
    min-width: 0;
  }
  .chart-title {
    font-family: var(--font-display);
    font-size: var(--text-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 700;
    color: var(--color-ink-2);
    margin-bottom: var(--space-xs);
  }
  /* Latency histogram */
  .hist { display: flex; align-items: flex-end; gap: 4px; height: 92px; }
  .hist-col { flex: 1; display: flex; flex-direction: column; align-items: center; min-width: 0; }
  .hist-bar-wrap { width: 100%; height: 64px; display: flex; align-items: flex-end; }
  .hist-bar {
    width: 100%;
    border-radius: 2px 2px 0 0;
    background: var(--color-accent);
    box-shadow: none;
    transition: opacity var(--dur-short) var(--ease-out);
  }
  .hist-n { font-family: var(--font-mono); font-size: 0.6875rem; color: var(--color-ink-2); font-variant-numeric: tabular-nums; height: 14px; }
  .hist-x { font-family: var(--font-mono); font-size: 0.625rem; color: var(--color-ink-3); white-space: nowrap; }
  /* Quality mix bar */
  .qbar { display: flex; height: 16px; border-radius: var(--radius-pill); overflow: hidden; background: var(--color-paper-4); }
  .qseg { height: 100%; }
  .q-good { background: var(--color-success); }
  .q-ok { background: var(--color-warning); }
  .q-poor { background: var(--color-danger); }
  .qlegend { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-top: var(--space-xs); font-size: 0.6875rem; color: var(--color-ink-2); }
  .qlegend span { display: inline-flex; align-items: center; gap: 4px; font-variant-numeric: tabular-nums; }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
  /* Colo distribution */
  .colo-rows { display: flex; flex-direction: column; gap: 5px; }
  .colo-row { display: grid; grid-template-columns: 34px 1fr 22px; align-items: center; gap: 6px; }
  .colo-name { font-family: var(--font-mono); font-size: 0.6875rem; font-weight: 700; color: var(--color-accent-hover); }
  .colo-track { height: 7px; border-radius: var(--radius-pill); background: var(--color-paper-4); overflow: hidden; }
  .colo-fill { display: block; height: 100%; border-radius: inherit; background: var(--color-accent); }
  .colo-n { font-family: var(--font-mono); font-size: 0.6875rem; color: var(--color-ink-2); text-align: right; font-variant-numeric: tabular-nums; }
</style>
