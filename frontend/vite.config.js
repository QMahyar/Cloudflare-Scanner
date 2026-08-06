import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

const apiTarget = process.env.VITE_API_TARGET || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [svelte()],
  base: './',
  build: {
    outDir: '../ui/dist',
    emptyOutDir: true,
    target: 'es2020',

    assetsDir: 'assets',
  },
  server: {
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true },
    },
  },
})
