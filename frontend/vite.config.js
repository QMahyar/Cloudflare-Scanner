import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The built bundle is embedded into the Go binary via //go:embed all:ui/dist.
// base:'./' makes asset URLs relative so they resolve correctly when the
// server serves index.html from '/'. The build output lands in ../ui/dist,
// which is committed to git (no Node needed for `go build`).
export default defineConfig({
  plugins: [svelte()],
  base: './',
  build: {
    outDir: '../ui/dist',
    emptyOutDir: true,
    target: 'es2020',
    // Stable asset folder so the Go side can register one route: GET /assets/.
    assetsDir: 'assets',
  },
})
