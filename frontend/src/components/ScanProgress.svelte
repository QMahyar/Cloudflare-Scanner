<script>
  import { _ } from 'svelte-i18n'

  // Shared scan feedback: the status pill + live percentage + progress bar,
  // followed by the post-scan summary chips. Purely presentational — the parent
  // owns the scan lifecycle and passes the current snapshot. Styles live in the
  // global app.css (.scan-pill / .scan-summary / .stat-chip), so this component
  // needs no <style> of its own.
  let {
    status,                 // 'idle' | 'running' | 'done' | 'cancelled'
    progressPct = 0,
    progressText = '',
    summary = null,         // { found, scanned, best, elapsed, rate } | null
    runningLabel = null,    // tab-specific "Scanning..." label
  } = $props()
</script>

{#if status !== 'idle'}
  <div class="progress-wrap active">
    <div class="scan-status">
      <span class="scan-pill" class:running={status === 'running'} class:done={status === 'done'} class:cancelled={status === 'cancelled'}>
        <span class="scan-pill-dot"></span>
        {status === 'running' ? (runningLabel || $_('scan.scanning')) : status === 'cancelled' ? $_('status.cancelled') : $_('status.done')}
      </span>
      <span class="progress-pct">{progressPct}%</span>
    </div>
    <div class="progress-bar"><div class="progress-fill" class:cancelled={status === 'cancelled'} style="width:{progressPct}%"></div></div>
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
