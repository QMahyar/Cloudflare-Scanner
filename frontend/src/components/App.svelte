<script>
  import { _ } from 'svelte-i18n'
  import { toggleLanguage } from '../lib/i18n.js'
  import EndpointScanner from './EndpointScanner.svelte'
  import IpScanner from './IpScanner.svelte'
  import Replacer from './Replacer.svelte'
  import About from './About.svelte'
  import Toast from './Toast.svelte'
  import QrModal from './QrModal.svelte'

  // Tabs stay mounted and are hidden via .hidden (not unmounted) so a running
  // scan's polling and results survive switching tabs — matching the original.
  let tab = $state('endpoint')
</script>

<div class="noise-overlay"></div>
<div class="container">
  <main class="app-shell">

    <div class="header-row">
      <div class="header-brand">
        <div class="header-logo" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M17.5 18H7a4.5 4.5 0 0 1-.7-8.94A6 6 0 0 1 18 9.5a3.75 3.75 0 0 1-.5 8.5Z" fill="#fff" fill-opacity="0.95"/>
            <circle cx="12" cy="12.3" r="1.6" fill="#f6821f"/>
            <path d="M9.7 12.3a2.3 2.3 0 0 1 4.6 0M8 12.3a4 4 0 0 1 8 0" stroke="#f6821f" stroke-width="1.1" stroke-linecap="round"/>
          </svg>
        </div>
        <div>
          <h1>{$_('title')}</h1>
          <p class="subtitle">{$_('subtitle')}</p>
        </div>
      </div>
      <button class="lang-btn" onclick={toggleLanguage}>{$_('langBtn')}</button>
    </div>

    <div class="tab-bar" role="tablist" aria-label="Scanner tabs">
      <button class="tab" class:active={tab === 'endpoint'} role="tab" aria-selected={tab === 'endpoint'} onclick={() => (tab = 'endpoint')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span>{$_('tab.endpoint')}</span><span class="tab-badge"></span>
      </button>
      <button class="tab" class:active={tab === 'clean'} role="tab" aria-selected={tab === 'clean'} onclick={() => (tab = 'clean')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
        <span>{$_('tab.clean')}</span><span class="tab-badge"></span>
      </button>
      <button class="tab" class:active={tab === 'replacer'} role="tab" aria-selected={tab === 'replacer'} onclick={() => (tab = 'replacer')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
        <span>{$_('tab.replacer')}</span><span class="tab-badge"></span>
      </button>
      <button class="tab" class:active={tab === 'about'} role="tab" aria-selected={tab === 'about'} onclick={() => (tab = 'about')}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        <span>{$_('tab.about')}</span>
      </button>
    </div>

    <div id="endpointTab" role="tabpanel" class:hidden={tab !== 'endpoint'}><EndpointScanner /></div>
    <div id="cleanTab" role="tabpanel" class:hidden={tab !== 'clean'}><IpScanner /></div>
    <div id="replacerTab" role="tabpanel" class:hidden={tab !== 'replacer'}><Replacer /></div>
    <div id="aboutTab" role="tabpanel" class:hidden={tab !== 'about'}><About /></div>

  </main>
</div>

<QrModal />
<Toast />
