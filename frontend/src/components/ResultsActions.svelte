<script>
  import { _ } from 'svelte-i18n'
  import { copyWithPorts, setCopyMode } from '../lib/copymode.js'

  let {
    onCopyAll,
    onDownload,
    onCSV,
    onJSON,
    onQR,
    onSelectAll,
    onDeselectAll,
    onCopySelected,
    extra = undefined,
  } = $props()
</script>

<div class="results-actions results-actions-top">
  <button class="btn btn-secondary btn-sm" onclick={onCopyAll} title={$_('results.copyAllTitle')}>{$_('results.copyAll')}</button>
  <select class="copy-mode-select" value={$copyWithPorts ? 'port' : 'ip'} onchange={(e) => setCopyMode(e.currentTarget.value === 'port')} title={$_('copy.menuTitle')} aria-label={$_('copy.menuTitle')}>
    <option value="port">{$_('copy.withPort')}</option>
    <option value="ip">{$_('copy.ipOnly')}</option>
  </select>
  <button class="btn btn-secondary btn-sm" onclick={onDownload} title={$_('results.downloadTitle')}>{$_('results.downloadRaw')}</button>
  <button class="btn btn-secondary btn-sm" onclick={onCSV} title={$_('results.exportCsvTitle')}>{$_('results.exportCsv')}</button>
  <button class="btn btn-secondary btn-sm" onclick={onJSON} title={$_('results.exportJsonTitle')}>{$_('results.exportJson')}</button>
  <button class="btn btn-secondary btn-sm" onclick={onQR} title="QR">QR</button>
  {#if extra}{@render extra()}{/if}
  <button class="btn btn-secondary btn-sm" onclick={onSelectAll}>{$_('results.selectAll')}</button>
  <button class="btn btn-secondary btn-sm" onclick={onDeselectAll}>{$_('results.deselectAll')}</button>
  <button class="btn btn-secondary btn-sm" onclick={onCopySelected} title={$_('results.copySelectedTitle')}>{$_('results.copySelected')}</button>
</div>
