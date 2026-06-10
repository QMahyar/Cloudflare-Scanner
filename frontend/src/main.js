import './lib/i18n.js' // registers messages, inits locale, wires <html>/<body> dir
import './app.css'
import { mount } from 'svelte'
import { waitLocale } from 'svelte-i18n'
import App from './components/App.svelte'

// Wait for the initial locale's messages before mounting so the first paint is
// already translated (no key-flash). esbuild target es2020 has no top-level
// await, so mount inside .then().
waitLocale().then(() => {
  mount(App, { target: document.getElementById('app') })
})
