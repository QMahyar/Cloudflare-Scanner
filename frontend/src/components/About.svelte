<script>
  import { _ } from 'svelte-i18n'
  import { apiJSON } from '../lib/api.js'
  import { copyToClipboard } from '../lib/clipboard.js'
  import { showToast } from '../lib/toast.js'
  import { showQR } from '../lib/modal.js'

  // Receiving addresses. The TRON address also receives USDT-TRC20 (recommended);
  // the EVM address is the same across all EVM chains; TON receives native USDT.
  const DONATE = {
    trx: 'TD2QrQFpW9QkUzhhH6X8QQEg12uH8wcQGg',
    ton: 'UQCOVixrzJdJ1pGus4zY7oTRpXs8D8mFv-8L6r0kYP5AQi68',
    evm: '0x6b5FC86D71C47b225785BFC6C8b329D180678B1e',
    btc: 'bc1qyuj82t3tlnxv7j7hq5ks98kmx9cmm72vx3jwta',
  }
  const COINS = [
    { kind: 'trx', badge: 'TRON (TRC-20)', hint: 'about.donateRecommended', note: 'about.noteTrx' },
    { kind: 'ton', badge: 'TON', note: 'about.noteTon' },
    { kind: 'evm', badge: 'EVM (Ethereum · BSC · Polygon · L2s)', note: 'about.noteEvm' },
    { kind: 'btc', badge: 'Bitcoin (BTC)', note: 'about.noteBtc' },
  ]

  let version = $state('—')
  let repoURL = $state('https://github.com/')
  let checking = $state(false)
  let updateStatus = $state(null) // {ok, html-less: msg, url?, isUpdate}

  $effect(() => {
    apiJSON('/api/version').then((d) => {
      if (d.version) version = 'v' + d.version.replace(/^v/i, '')
      if (d.repo_url) repoURL = d.repo_url
    }).catch(() => {})
  })

  function copyAddr(kind) {
    copyToClipboard(DONATE[kind])
    showToast($_('copied.clipboard'))
  }
  function addrQR(kind) { showQR(DONATE[kind]) }

  async function checkUpdate() {
    checking = true; updateStatus = null
    try {
      const d = await apiJSON('/api/update-check')
      if (d.update_available) {
        updateStatus = { ok: true, isUpdate: true, msg: $_('about.updateAvailable', { values: { v: d.latest } }), url: d.url || repoURL }
      } else {
        updateStatus = { ok: true, isUpdate: false, msg: $_('about.upToDate', { values: { v: d.current } }) }
      }
    } catch (e) {
      updateStatus = { ok: false, msg: $_('about.updateFailed', { values: { msg: e.message } }) }
    }
    checking = false
  }
</script>

<div class="card">
  <h2>
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
    <span>{$_('about.header')}</span>
  </h2>
  <p class="desc">{$_('about.desc')}</p>
  <div class="about-meta">
    <span class="about-version">{version}</span>
    <a class="help-link" href={repoURL} target="_blank" rel="noopener">{$_('about.viewSource')}</a>
  </div>
  <div class="status-slot">
    <button class="btn btn-secondary btn-sm" onclick={checkUpdate} disabled={checking} title={$_('about.checkUpdateTitle')}>{checking ? $_('about.checking') : $_('about.checkUpdate')}</button>
    {#if updateStatus}
      <div class="status-slot">
        <div class={updateStatus.ok ? 'success-msg' : 'error-msg'}>
          {updateStatus.msg}
          {#if updateStatus.isUpdate}<a class="help-link" href={updateStatus.url} target="_blank" rel="noopener">{$_('about.download')}</a>{/if}
        </div>
      </div>
    {/if}
  </div>
  <p class="muted-inline" style="margin-top:var(--space-sm);display:block">{$_('about.privacy')}</p>
</div>

<div class="card">
  <h2>
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>
    <span>{$_('about.donateHeader')}</span>
  </h2>
  <p class="desc">{$_('about.donateDesc')}</p>

  {#each COINS as c}
    <div class="donate-row">
      <div class="donate-coin">
        <span class="donate-badge">{c.badge}</span>
        {#if c.hint}<span class="donate-hint">{$_(c.hint)}</span>{/if}
      </div>
      <div class="donate-addr-row">
        <code class="donate-addr">{DONATE[c.kind]}</code>
        <button class="btn btn-secondary btn-sm" onclick={() => copyAddr(c.kind)}>{$_('buttons.copy')}</button>
        <button class="btn btn-secondary btn-sm" onclick={() => addrQR(c.kind)}>QR</button>
      </div>
      <p class="donate-note">{@html $_(c.note)}</p>
    </div>
  {/each}

  <p class="muted-inline" style="margin-top:var(--space-sm);display:block">{$_('about.credits')}</p>
</div>
