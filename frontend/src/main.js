import './lib/i18n.js'
import './app.css'
import { mount } from 'svelte'
import { waitLocale } from 'svelte-i18n'
import { initTheme } from './lib/theme.js'
import App from './components/App.svelte'

initTheme()

waitLocale().then(() => {
  mount(App, { target: document.getElementById('app') })
})
