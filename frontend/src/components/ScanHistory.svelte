<script>
  import { _ } from 'svelte-i18n'
  import { scanHistory, clearHistory } from '../lib/stores.js'

  // Lightweight, local-only log of finished scans (summaries, not result sets).
  // Filtered to the host tab so each scanner shows its own runs.
  let { tab } = $props()
  const items = $derived(($scanHistory || []).filter((h) => !tab || h.tab === tab))

  function fmtTime(ts) {
    try { return new Date(ts).toLocaleString() } catch { return '' }
  }
</script>

{#if items.length}
  <div class="card">
    <details class="help-panel">
      <summary>{$_('history.header')} <span class="count-chip">{items.length}</span></summary>
      <div class="hist-actions">
        <button class="btn btn-ghost btn-sm" onclick={clearHistory}>{$_('history.clear')}</button>
      </div>
      <div class="hist-list">
        {#each items as h}
          <div class="hist-row">
            <span class="hist-label">{h.label}</span>
            <span class="hist-meta">
              {$_('history.found', { values: { n: h.found } })}
              {$_('history.scanned', { values: { n: h.scanned } })}
              {#if h.best != null} · {$_('history.best', { values: { n: h.best } })}{/if}
              {#if h.topScore} · {$_('history.score', { values: { n: h.topScore } })}{/if}
            </span>
            <span class="hist-time">{fmtTime(h.ts)}</span>
          </div>
        {/each}
      </div>
    </details>
  </div>
{/if}

<style>
  /* Hallmark · component: scan history · genre: modern-minimal · theme: design.md */
  .hist-actions { display: flex; justify-content: flex-end; margin: var(--space-2xs) 0 var(--space-xs); }
  .hist-list { display: flex; flex-direction: column; gap: var(--space-2xs); }
  .hist-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-2xs);
    padding: var(--space-xs) var(--space-sm);
    background: var(--color-paper-3);
    border-block-start: var(--rule-thin) solid var(--color-rule);
    font-size: var(--text-xs);
  }
  .hist-label { font-family: var(--font-display); font-weight: 700; color: var(--color-ink); }
  .hist-meta { color: var(--color-ink-2); font-variant-numeric: tabular-nums; }
  .hist-time { color: var(--color-ink-3); font-family: var(--font-mono); font-size: 0.6875rem; white-space: nowrap; }
  @media (min-width: 40rem) {
    .hist-row { grid-template-columns: minmax(7rem, auto) minmax(0, 1fr) auto; align-items: baseline; gap: var(--space-sm); }
    .hist-time { margin-inline-start: auto; }
  }
</style>
