import './lib/i18n.js' // registers messages, inits locale, wires <html>/<body> dir
import './app.css'
import { mount } from 'svelte'
import { waitLocale } from 'svelte-i18n'
import { initTheme } from './lib/theme.js'
import App from './components/App.svelte'

// Apply the persisted theme before mount so the first paint is correct.
initTheme()

// Wait for the initial locale's messages before mounting so the first paint is
// already translated (no key-flash). esbuild target es2020 has no top-level
// await, so mount inside .then().
waitLocale().then(() => {
  mount(App, { target: document.getElementById('app') })
})
