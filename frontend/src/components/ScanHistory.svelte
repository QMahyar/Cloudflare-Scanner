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
  .hist-actions { display: flex; justify-content: flex-end; margin: 4px 0 8px; }
  .hist-list { display: flex; flex-direction: column; gap: 6px; }
  .hist-row {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 8px;
    padding: 7px 10px;
    background: var(--bg-card);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
  }
  .hist-label { font-weight: 600; color: var(--text-heading); }
  .hist-meta { color: var(--text-secondary); font-variant-numeric: tabular-nums; }
  .hist-time { margin-inline-start: auto; color: var(--text-tertiary); font-size: 0.6875rem; white-space: nowrap; }
</style>
