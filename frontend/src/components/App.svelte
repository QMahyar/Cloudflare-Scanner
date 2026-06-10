<script>
  // Phase 0 placeholder — proves the Vite build + Go embed + asset serving work
  // end to end. Replaced by the real shell in Phase 1.
  let ok = $state(true)
</script>

<main style="font-family: system-ui, sans-serif; color: #e8eaed; background: #08090c; min-height: 100vh; display: grid; place-items: center; gap: 1rem;">
  <h1 style="color: #f6821f;">Cloudflare Scanner</h1>
  <p>Vite + Svelte 5 shell loaded from the embedded bundle. {ok ? '✓' : ''}</p>
</main>
