<script>
  import { _ } from 'svelte-i18n'

  let {
    status,
    progressPct = 0,
    progressText = '',
    summary = null,
    runningLabel = null,
  } = $props()
</script>

{#if status !== 'idle'}
  <div class="progress-wrap active" aria-live="polite">
    <div class="scan-status">
      <span class={['scan-pill', {
        running: status === 'running',
        done: status === 'done',
        cancelled: status === 'cancelled',
      }]}>
        <span class="scan-pill-dot"></span>
        {status === 'running' ? (runningLabel || $_('scan.scanning')) : status === 'cancelled' ? $_('status.cancelled') : $_('status.done')}
      </span>
      <span class="progress-pct">{progressPct}%</span>
    </div>
    <div class="progress-bar" role="progressbar" aria-label={runningLabel || $_('scan.scanning')} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(progressPct)}>
      <div class={['progress-fill', { cancelled: status === 'cancelled' }]} style:width="{progressPct}%"></div>
    </div>
    <div class="progress-text">{progressText}</div>
  </div>
{/if}

{#if summary}
  <div class="scan-summary">
    <span class="stat-chip stat-found"><span class="stat-num">{summary.found}</span><span class="stat-label">{$_('summary.found')}</span></span>
    <span class="stat-chip"><span class="stat-num">{summary.scanned}</span><span class="stat-label">{$_('summary.scanned')}</span></span>
    {#if summary.best != null}<span class="stat-chip"><span class="stat-num">{summary.best}<span class="stat-unit">ms</span></span><span class="stat-label">{$_('summary.best')}</span></span>{/if}
    {#if summary.elapsed > 0}<span class="stat-chip"><span class="stat-num">{summary.elapsed.toFixed(1)}<span class="stat-unit">s</span></span><span class="stat-label">{$_('summary.elapsed')}</span></span>{/if}
    {#if summary.rate > 0}<span class="stat-chip"><span class="stat-num">{summary.rate}<span class="stat-unit">/s</span></span><span class="stat-label">{$_('summary.rate')}</span></span>{/if}
  </div>
{/if}
