<script>
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import { toggleLanguage } from '../lib/i18n.js'
  import { theme, toggleTheme } from '../lib/theme.js'
  import { apiJSON } from '../lib/api.js'
  import { appState, initResults, persistResults } from '../lib/stores.svelte.js'
  import EndpointScanner from './EndpointScanner.svelte'
  import IpScanner from './IpScanner.svelte'
  import Replacer from './Replacer.svelte'
  import About from './About.svelte'
  import Toast from './Toast.svelte'
  import QrModal from './QrModal.svelte'

  initResults()

  $effect(() => {
    if (appState.endpointRaw || appState.cleanData || appState.endpointScanning === false || appState.cleanScanning === false) {
      persistResults()
    }
  })

  const epBadge = $derived(appState.endpointRaw?.length || 0)
  const cleanBadge = $derived(appState.cleanData?.entries?.length || 0)
  const repBadge = $derived(appState.replacerGenerated?.length || 0)
  const pageTitle = $derived($_(`tab.${appState.activeTab}`))
  const pageDescription = $derived($_(`page.${appState.activeTab}Desc`))

  import { createMediaQuery } from '../lib/media.svelte.js'

  const host = typeof window !== 'undefined' ? window.location.host : ''
  let version = $state('')
  const isDesktop = createMediaQuery('(min-width: 60rem)')
  const tabOrientation = $derived(isDesktop.matches ? 'vertical' : 'horizontal')

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

<div class="container">
  <main class="app-shell">
    <aside class="app-sidebar">
      <div class="header-brand">
        <div class="header-logo" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path class="logo-cloud" d="M17.5 18H7a4.5 4.5 0 0 1-.7-8.94A6 6 0 0 1 18 9.5a3.75 3.75 0 0 1-.5 8.5Z"/>
            <circle class="logo-signal" cx="12" cy="12.3" r="1.6"/>
            <path class="logo-signal-line" d="M9.7 12.3a2.3 2.3 0 0 1 4.6 0M8 12.3a4 4 0 0 1 8 0" stroke-width="1.1" stroke-linecap="round"/>
          </svg>
        </div>
        <div class="brand-copy">
          <h1>{$_('title')}</h1>
          <p class="subtitle">{$_('brand.tagline')}</p>
        </div>
      </div>

      <div class="sidebar-label">{$_('nav.tools')}</div>
      <div class="tab-bar" role="tablist" aria-label="Scanner tabs" aria-orientation={tabOrientation}>
      <button id="tab-endpoint" class={['tab', { active: appState.activeTab === 'endpoint' }]} role="tab" aria-selected={appState.activeTab === 'endpoint'} aria-controls="endpointTab" tabindex={appState.activeTab === 'endpoint' ? 0 : -1} onclick={() => appState.activeTab = 'endpoint'} onkeydown={(event) => handleTabKeydown(event, 'endpoint')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.endpoint')}</span><span class="tab-label tab-label-short">{$_('tab.endpointShort')}</span>{#if appState.endpointScanning}<span class="tab-scan-dot" title={$_('scan.scanning')}></span>{/if}<span class={['tab-badge', { show: epBadge > 0 }]}>{epBadge || ''}</span>
      </button>
      <button id="tab-clean" class={['tab', { active: appState.activeTab === 'clean' }]} role="tab" aria-selected={appState.activeTab === 'clean'} aria-controls="cleanTab" tabindex={appState.activeTab === 'clean' ? 0 : -1} onclick={() => appState.activeTab = 'clean'} onkeydown={(event) => handleTabKeydown(event, 'clean')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.clean')}</span><span class="tab-label tab-label-short">{$_('tab.cleanShort')}</span>{#if appState.cleanScanning}<span class="tab-scan-dot" title={$_('scan.scanning')}></span>{/if}<span class={['tab-badge', { show: cleanBadge > 0 }]}>{cleanBadge || ''}</span>
      </button>
      <button id="tab-replacer" class={['tab', { active: appState.activeTab === 'replacer' }]} role="tab" aria-selected={appState.activeTab === 'replacer'} aria-controls="replacerTab" tabindex={appState.activeTab === 'replacer' ? 0 : -1} onclick={() => appState.activeTab = 'replacer'} onkeydown={(event) => handleTabKeydown(event, 'replacer')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.replacer')}</span><span class="tab-label tab-label-short">{$_('tab.replacerShort')}</span><span class={['tab-badge', { show: repBadge > 0 }]}>{repBadge || ''}</span>
      </button>
      <button id="tab-about" class={['tab', { active: appState.activeTab === 'about' }]} role="tab" aria-selected={appState.activeTab === 'about'} aria-controls="aboutTab" tabindex={appState.activeTab === 'about' ? 0 : -1} onclick={() => appState.activeTab = 'about'} onkeydown={(event) => handleTabKeydown(event, 'about')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        <span class="tab-label tab-label-full">{$_('tab.about')}</span><span class="tab-label tab-label-short">{$_('tab.aboutShort')}</span>
      </button>
      </div>

      <div class="sidebar-status">
        <div class="sidebar-status-line"><span class="host-dot"></span><span>{$_('status.local')}</span></div>
        <code>{host}</code>
        {#if version}<span class="sidebar-version">v{version.replace(/^v/i, '')}</span>{/if}
      </div>
    </aside>

    <header class="header-row">
      <div class="mobile-brand">
        <span class="mobile-brand-mark" aria-hidden="true"></span>
        <span>{$_('title')}</span>
      </div>
      <div class="page-context">
        <span class="page-eyebrow">{$_('page.workspace')}</span>
        <h2>{pageTitle}</h2>
        <p>{pageDescription}</p>
      </div>
      <div class="header-actions">
        <span class="host-pill" title={$_('about.privacy')}><span class="host-dot"></span><span class="host-text">{$_('status.local')}</span></span>
        <button class="theme-btn" data-theme-toggle onclick={toggleTheme} title={$_('themeToggle')} aria-label={$_('themeToggle')}>
          {#if $theme === 'light'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M6.3 17.7l-1.4 1.4M19.1 4.9l-1.4 1.4"/></svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/></svg>
          {/if}
        </button>
        <button class="lang-btn" onclick={toggleLanguage}>{$_('langBtn')}</button>
      </div>
    </header>

    <div id="endpointTab" class={['workspace-panel', { hidden: appState.activeTab !== 'endpoint' }]} role="tabpanel" aria-labelledby="tab-endpoint"><EndpointScanner /></div>
    <div id="cleanTab" class={['workspace-panel', { hidden: appState.activeTab !== 'clean' }]} role="tabpanel" aria-labelledby="tab-clean"><IpScanner /></div>
    <div id="replacerTab" class={['workspace-panel', { hidden: appState.activeTab !== 'replacer' }]} role="tabpanel" aria-labelledby="tab-replacer"><Replacer /></div>
    <div id="aboutTab" class={['workspace-panel', { hidden: appState.activeTab !== 'about' }]} role="tabpanel" aria-labelledby="tab-about"><About /></div>

  </main>
</div>

<QrModal />
<Toast />
