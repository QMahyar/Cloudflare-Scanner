<script>
  import { _ } from 'svelte-i18n'

  /** When true, render body only (parent provides pane chrome / summary). */
  let { dense = false } = $props()

  // Curated Cloudflare Worker / edge-tunnel projects for building configs the
  // Replacer then rewires onto clean IPs. Static links only — no network.
  const projects = [
    {
      url: 'https://github.com/bia-pain-bache/BPB-Worker-Panel',
      name: 'BPB Worker Panel',
      desc: 'replacer.res.bpb',
      rec: true,
    },
    {
      url: 'https://github.com/zizifn/edgetunnel',
      name: 'edgetunnel',
      desc: 'replacer.res.edgetunnel',
    },
    {
      url: 'https://github.com/3Kmfi6HP/EDtunnel',
      name: 'EDtunnel',
      desc: 'replacer.res.edtunnel',
    },
    {
      url: 'https://github.com/cmliu/edgetunnel',
      name: 'cmliu/edgetunnel',
      desc: 'replacer.res.cmliu',
    },
  ]
</script>

{#snippet body()}
  <p class="desc help-intro">{$_('replacer.res.intro')}</p>
  <div class="link-grid">
    {#each projects as l (l.url)}
      <a class="link-tile" href={l.url} target="_blank" rel="noopener">
        <span class="link-tile-name">
          <span>{l.name}</span>
          {#if l.rec}<span class="rec-pill">{$_('help.recommended')}</span>{/if}
        </span>
        <span class="link-tile-desc">{$_(l.desc)}</span>
      </a>
    {/each}
  </div>
{/snippet}

{#if dense}
  <div class="help-panel help-panel-dense">
    {@render body()}
  </div>
{:else}
  <details class="help-panel">
    <summary>{$_('replacer.res.header')}</summary>
    {@render body()}
  </details>
{/if}
