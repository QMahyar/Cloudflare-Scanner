import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The built bundle is embedded into the Go binary via //go:embed all:ui/dist.
// base:'./' makes asset URLs relative so they resolve correctly when the
// server serves index.html from '/'. The build output lands in ../ui/dist,
// which is committed to git (no Node needed for `go build`).
// `go run .` binds to an OS-assigned random port (printed at startup) — set
// VITE_API_TARGET to that address to proxy /api/* during `npm run dev`, e.g.
//   VITE_API_TARGET=http://127.0.0.1:51824 npm run dev
const apiTarget = process.env.VITE_API_TARGET || 'http://127.0.0.1:8080'

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
  server: {
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true },
    },
  },
})
