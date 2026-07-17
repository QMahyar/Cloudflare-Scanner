<script>
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import { toggleLanguage } from '../lib/i18n.js'
  import { apiJSON } from '../lib/api.js'
  import { activeTab, endpointRaw, cleanData, replacerGenerated, loadResults, beginResultsPersistence, endpointScanning, cleanScanning } from '../lib/stores.js'
  import EndpointScanner from './EndpointScanner.svelte'
  import IpScanner from './IpScanner.svelte'
  import Replacer from './Replacer.svelte'
  import About from './About.svelte'
  import Toast from './Toast.svelte'
  import QrModal from './QrModal.svelte'

  // Restore persisted results once, then begin auto-persisting changes.
  const saved = loadResults()
  if (saved) {
    if (Array.isArray(saved.endpointRaw) && saved.endpointRaw.length) endpointRaw.set(saved.endpointRaw)
    if (saved.cleanData) cleanData.set(saved.cleanData)
  }
  beginResultsPersistence()

  const epBadge = $derived($endpointRaw?.length || 0)
  const cleanBadge = $derived($cleanData?.entries?.length || 0)
  const repBadge = $derived($replacerGenerated?.length || 0)

  // Local-host indicator: the page is served from the scanner's own
  // 127.0.0.1:<port> listener, so window.location.host is the real address.
  const host = typeof window !== 'undefined' ? window.location.host : ''
  let version = $state('')
  let tabOrientation = $state('horizontal')
  const tabOrder = ['endpoint', 'clean', 'replacer', 'about']
  onMount(async () => {
    try { const v = await apiJSON('/api/version'); version = v?.version || '' } catch {}
  })
  onMount(() => {
    const media = window.matchMedia('(min-width: 60rem)')
    const updateOrientation = () => { tabOrientation = media.matches ? 'vertical' : 'horizontal' }
    updateOrientation()
    media.addEventListener('change', updateOrientation)
    return () => media.removeEventListener('change', updateOrientation)
  })

  function handleTabKeydown(event, current) {
    const key = event.key
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(key)) return
    event.preventDefault()
    const currentIndex = tabOrder.indexOf(current)
    let nextIndex = currentIndex
    if (key === 'Home') nextIndex = 0
    else if (key === 'End') nextIndex = tabOrder.length - 1
    else if (key === 'ArrowRight' || key === 'ArrowDown') nextIndex = (currentIndex + 1) % tabOrder.length
    else nextIndex = (currentIndex - 1 + tabOrder.length) % tabOrder.length
    const next = tabOrder[nextIndex]
    activeTab.set(next)
    document.getElementById(`tab-${next}`)?.focus({ preventScroll: true })
  }
</script>

<div class="container">
  <main class="app-shell">

    <div class="header-row">
      <div class="header-brand">
        <div class="header-logo" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path class="logo-cloud" d="M17.5 18H7a4.5 4.5 0 0 1-.7-8.94A6 6 0 0 1 18 9.5a3.75 3.75 0 0 1-.5 8.5Z"/>
            <circle class="logo-signal" cx="12" cy="12.3" r="1.6"/>
            <path class="logo-signal-line" d="M9.7 12.3a2.3 2.3 0 0 1 4.6 0M8 12.3a4 4 0 0 1 8 0" stroke-width="1.1" stroke-linecap="round"/>
          </svg>
        </div>
        <div>
          <h1>{$_('title')}</h1>
          <p class="subtitle">{$_('subtitle')}</p>
        </div>
      </div>
      <div class="header-actions">
        {#if version}<span class="ver-chip" title={$_('about.header')}>{version}</span>{/if}
        <span class="host-pill" title={$_('about.privacy')}>
          <span class="host-dot"></span>
          <span class="host-text">{host}</span>
        </span>
        <button class="lang-btn" onclick={toggleLanguage}>{$_('langBtn')}</button>
      </div>
    </div>

    <div class="tab-bar" role="tablist" aria-label="Scanner tabs" aria-orientation={tabOrientation}>
      <button id="tab-endpoint" class="tab" class:active={$activeTab === 'endpoint'} role="tab" aria-selected={$activeTab === 'endpoint'} aria-controls="endpointTab" tabindex={$activeTab === 'endpoint' ? 0 : -1} onclick={() => activeTab.set('endpoint')} onkeydown={(event) => handleTabKeydown(event, 'endpoint')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.endpoint')}</span><span class="tab-label tab-label-short">{$_('tab.endpointShort')}</span>{#if $endpointScanning}<span class="tab-scan-dot" title={$_('scan.scanning')}></span>{/if}<span class="tab-badge" class:show={epBadge > 0}>{epBadge || ''}</span>
      </button>
      <button id="tab-clean" class="tab" class:active={$activeTab === 'clean'} role="tab" aria-selected={$activeTab === 'clean'} aria-controls="cleanTab" tabindex={$activeTab === 'clean' ? 0 : -1} onclick={() => activeTab.set('clean')} onkeydown={(event) => handleTabKeydown(event, 'clean')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.clean')}</span><span class="tab-label tab-label-short">{$_('tab.cleanShort')}</span>{#if $cleanScanning}<span class="tab-scan-dot" title={$_('scan.scanning')}></span>{/if}<span class="tab-badge" class:show={cleanBadge > 0}>{cleanBadge || ''}</span>
      </button>
      <button id="tab-replacer" class="tab" class:active={$activeTab === 'replacer'} role="tab" aria-selected={$activeTab === 'replacer'} aria-controls="replacerTab" tabindex={$activeTab === 'replacer' ? 0 : -1} onclick={() => activeTab.set('replacer')} onkeydown={(event) => handleTabKeydown(event, 'replacer')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.replacer')}</span><span class="tab-label tab-label-short">{$_('tab.replacerShort')}</span><span class="tab-badge" class:show={repBadge > 0}>{repBadge || ''}</span>
      </button>
      <button id="tab-about" class="tab" class:active={$activeTab === 'about'} role="tab" aria-selected={$activeTab === 'about'} aria-controls="aboutTab" tabindex={$activeTab === 'about' ? 0 : -1} onclick={() => activeTab.set('about')} onkeydown={(event) => handleTabKeydown(event, 'about')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.about')}</span><span class="tab-label tab-label-short">{$_('tab.aboutShort')}</span>
      </button>
    </div>

    <div id="endpointTab" class="workspace-panel" role="tabpanel" aria-labelledby="tab-endpoint" class:hidden={$activeTab !== 'endpoint'}><EndpointScanner /></div>
    <div id="cleanTab" class="workspace-panel" role="tabpanel" aria-labelledby="tab-clean" class:hidden={$activeTab !== 'clean'}><IpScanner /></div>
    <div id="replacerTab" class="workspace-panel" role="tabpanel" aria-labelledby="tab-replacer" class:hidden={$activeTab !== 'replacer'}><Replacer /></div>
    <div id="aboutTab" class="workspace-panel" role="tabpanel" aria-labelledby="tab-about" class:hidden={$activeTab !== 'about'}><About /></div>

  </main>
</div>

<QrModal />
<Toast />
