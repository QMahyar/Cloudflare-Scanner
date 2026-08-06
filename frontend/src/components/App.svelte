<script>
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import { toggleLanguage } from '../lib/i18n.js'
  import { theme, toggleTheme } from '../lib/theme.js'
  import { apiJSON } from '../lib/api.js'
  import { activeTab, endpointRaw, cleanData, replacerGenerated, loadResults, beginResultsPersistence, endpointScanning, cleanScanning } from '../lib/stores.js'
  import EndpointScanner from './EndpointScanner.svelte'
  import IpScanner from './IpScanner.svelte'
  import Replacer from './Replacer.svelte'
  import About from './About.svelte'
  import Toast from './Toast.svelte'
  import QrModal from './QrModal.svelte'

  const saved = loadResults()
  if (saved) {
    if (Array.isArray(saved.endpointRaw) && saved.endpointRaw.length) endpointRaw.set(saved.endpointRaw)
    if (saved.cleanData) cleanData.set(saved.cleanData)
  }
  beginResultsPersistence()

  const epBadge = $derived($endpointRaw?.length || 0)
  const cleanBadge = $derived($cleanData?.entries?.length || 0)
  const repBadge = $derived($replacerGenerated?.length || 0)
  const pageTitle = $derived($_(`tab.${$activeTab}`))

  const host = typeof window !== 'undefined' ? window.location.host : ''
  let version = $state('')
  const tabOrder = ['endpoint', 'clean', 'replacer', 'about']

  onMount(async () => {
    try { const v = await apiJSON('/api/version'); version = v?.version || '' } catch {}
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

<div class="app-layout">
  <aside class="sidebar">
    <div class="sidebar-brand">
      <div class="sidebar-logo" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path class="logo-cloud" d="M17.5 18H7a4.5 4.5 0 0 1-.7-8.94A6 6 0 0 1 18 9.5a3.75 3.75 0 0 1-.5 8.5Z"/>
          <circle class="logo-signal" cx="12" cy="12.3" r="1.6"/>
          <path class="logo-signal-line" d="M9.7 12.3a2.3 2.3 0 0 1 4.6 0M8 12.3a4 4 0 0 1 8 0" stroke-width="1.1" stroke-linecap="round"/>
        </svg>
      </div>
      <span class="sidebar-brand-text">{$_('title')}</span>
    </div>

    <nav class="sidebar-nav" role="tablist" aria-label="Scanner tabs">
      <button id="tab-endpoint" class={['sidebar-tab', { active: $activeTab === 'endpoint' }]} role="tab" aria-selected={$activeTab === 'endpoint'} aria-controls="endpointTab" tabindex={$activeTab === 'endpoint' ? 0 : -1} onclick={() => activeTab.set('endpoint')} onkeydown={(event) => handleTabKeydown(event, 'endpoint')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span>{$_('tab.endpoint')}</span>
        {#if $endpointScanning}<span class="tab-scan-dot"></span>{/if}
        {#if epBadge > 0}<span class="tab-badge">{epBadge}</span>{/if}
      </button>
      <button id="tab-clean" class={['sidebar-tab', { active: $activeTab === 'clean' }]} role="tab" aria-selected={$activeTab === 'clean'} aria-controls="cleanTab" tabindex={$activeTab === 'clean' ? 0 : -1} onclick={() => activeTab.set('clean')} onkeydown={(event) => handleTabKeydown(event, 'clean')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
        <span>{$_('tab.clean')}</span>
        {#if $cleanScanning}<span class="tab-scan-dot"></span>{/if}
        {#if cleanBadge > 0}<span class="tab-badge">{cleanBadge}</span>{/if}
      </button>
      <button id="tab-replacer" class={['sidebar-tab', { active: $activeTab === 'replacer' }]} role="tab" aria-selected={$activeTab === 'replacer'} aria-controls="replacerTab" tabindex={$activeTab === 'replacer' ? 0 : -1} onclick={() => activeTab.set('replacer')} onkeydown={(event) => handleTabKeydown(event, 'replacer')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
        <span>{$_('tab.replacer')}</span>
        {#if repBadge > 0}<span class="tab-badge">{repBadge}</span>{/if}
      </button>
      <button id="tab-about" class={['sidebar-tab', { active: $activeTab === 'about' }]} role="tab" aria-selected={$activeTab === 'about'} aria-controls="aboutTab" tabindex={$activeTab === 'about' ? 0 : -1} onclick={() => activeTab.set('about')} onkeydown={(event) => handleTabKeydown(event, 'about')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        <span>{$_('tab.about')}</span>
      </button>
    </nav>

    <div class="sidebar-footer">
      <div class="sidebar-status">
        <span class="host-dot"></span>
        <code>{host}</code>
      </div>
      {#if version}<span class="sidebar-version">v{version.replace(/^v/i, '')}</span>{/if}
    </div>
  </aside>

  <div class="main-area">
    <header class="main-header">
      <div class="header-left">
        <h1>{pageTitle}</h1>
      </div>
      <div class="header-right">
        <span class="host-pill" title={$_('about.privacy')}><span class="host-dot"></span>{$_('status.local')}</span>
        <button class="icon-btn" data-theme-toggle onclick={toggleTheme} title={$_('themeToggle')} aria-label={$_('themeToggle')}>
          {#if $theme === 'light'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M6.3 17.7l-1.4 1.4M19.1 4.9l-1.4 1.4"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/></svg>
          {/if}
        </button>
        <button class="lang-btn" onclick={toggleLanguage}>{$_('langBtn')}</button>
      </div>
    </header>

    <div class="workspace">
      {#if $activeTab === 'endpoint'}
        <EndpointScanner />
      {:else if $activeTab === 'clean'}
        <IpScanner />
      {:else if $activeTab === 'replacer'}
        <Replacer />
      {:else}
        <About />
      {/if}
    </div>
  </div>
</div>

<QrModal />
<Toast />
