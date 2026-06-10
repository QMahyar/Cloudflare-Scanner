<script>
  import { _ } from 'svelte-i18n'

  const online = [
    { url: 'https://warp-generator.github.io/warp/', name: 'warp-generator.github.io', rec: true, desc: 'help.gen1' },
    { url: 'https://lanrat.github.io/wireguard-warp-generator/', name: 'lanrat.github.io', desc: 'help.gen2' },
    { url: 'https://warp-mirrors.vercel.app/', name: 'warp-mirrors.vercel.app', desc: 'help.gen3' },
    { url: 'https://warp.llimonix.pw/', name: 'warp.llimonix.pw', desc: 'help.gen4' },
    { url: 'https://warpper.me/', name: 'warpper.me', desc: 'help.gen5' },
  ]
  const telegram = [
    { url: 'https://t.me/warp_generator_bot', name: '@warp_generator_bot', desc: 'help.tg1' },
    { url: 'https://t.me/WarpGenerator_bot', name: '@WarpGenerator_bot', desc: 'help.tg2' },
    { url: 'https://github.com/cat228608/Warp-Generator-Bot', name: 'Warp-Generator-Bot', desc: 'help.tg3' },
    { url: 'https://t.me/ConfigWireguard', name: '@ConfigWireguard', desc: 'help.tg4' },
    { url: 'https://t.me/str_bypass', name: '@str_bypass', desc: 'help.tg5' },
    { url: 'https://t.me/findllimonix', name: '@findllimonix', desc: 'help.tg6' },
  ]
  const cli = [
    { url: 'https://github.com/ViRb3/wgcf', name: 'ViRb3/wgcf', desc: 'help.cli1' },
    { url: 'https://github.com/lanrat/wireguard-warp-generator', name: 'lanrat/wireguard-warp-generator', desc: 'help.cli2' },
    { url: 'https://github.com/AnimMouse/wgcf-connector', name: 'AnimMouse/wgcf-connector', desc: 'help.cli3' },
    { url: 'https://gitlab.com/ImMALWARE/bash-warp-generator', name: 'ImMALWARE/bash-warp-generator', desc: 'help.cli4' },
    { url: 'https://github.com/barry-far/V2ray-Config', name: 'barry-far/V2ray-Config', desc: 'help.cli5' },
  ]
</script>

<details class="help-panel">
  <summary>{$_('help.header')}</summary>
  <p class="desc" style="margin-top:8px">{$_('help.desc')}</p>

  <h3 class="help-cat">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
    <span>{$_('help.onlineGen')}</span>
  </h3>
  <div class="link-grid">
    {#each online as l}
      <a class="link-tile" href={l.url} target="_blank" rel="noopener">
        <span class="link-tile-name"><span>{l.name}</span>{#if l.rec}<span class="rec-pill">{$_('help.recommended')}</span>{/if}</span>
        <span class="link-tile-desc">{$_(l.desc)}</span>
      </a>
    {/each}
  </div>

  <h3 class="help-cat">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21.5 4.5 2.5 12l6 2m13-9.5-3 15-7-5.5m10-9.5L8.5 16m0 0V21l3.5-4"/></svg>
    <span>{$_('help.telegram')}</span>
  </h3>
  <div class="link-grid">
    {#each telegram as l}
      <a class="link-tile" href={l.url} target="_blank" rel="noopener">
        <span class="link-tile-name"><span>{l.name}</span></span>
        <span class="link-tile-desc">{$_(l.desc)}</span>
      </a>
    {/each}
  </div>

  <h3 class="help-cat">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
    <span>{$_('help.cliTools')}</span>
  </h3>
  <div class="link-grid">
    {#each cli as l}
      <a class="link-tile" href={l.url} target="_blank" rel="noopener">
        <span class="link-tile-name"><span>{l.name}</span></span>
        <span class="link-tile-desc">{$_(l.desc)}</span>
      </a>
    {/each}
  </div>

  <h3 class="help-cat">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5" y="2" width="14" height="20" rx="2"/><line x1="12" y1="18" x2="12" y2="18"/></svg>
    <span>{$_('help.clientApps')}</span>
  </h3>
  <div class="app-grid">
    <div class="app-card">
      <h3>Amnezia VPN</h3>
      <div><span class="os-tag">Windows</span><span class="os-tag">Android</span><span class="os-tag">iOS</span></div>
      <p>{$_('help.app1')}</p>
      <a class="help-link" href="https://amnezia.org/" target="_blank" rel="noopener">{$_('help.visit')}</a>
    </div>
    <div class="app-card">
      <h3>WireSock</h3>
      <div><span class="os-tag">Windows</span></div>
      <p>{$_('help.app2')}</p>
      <a class="help-link" href="https://www.wiresock.net/" target="_blank" rel="noopener">{$_('help.visit2')}</a>
    </div>
    <div class="app-card">
      <h3>WG Tunnel</h3>
      <div><span class="os-tag">Android</span></div>
      <p>{$_('help.app3')}</p>
      <a class="help-link" href="https://github.com/zaneschepke/wgtunnel" target="_blank" rel="noopener">{$_('help.visit3')}</a>
    </div>
  </div>
</details>
