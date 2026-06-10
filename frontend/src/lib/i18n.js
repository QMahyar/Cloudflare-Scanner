import { addMessages, init, locale, _ } from 'svelte-i18n'
import { get } from 'svelte/store'
import en from '../locales/en.json'
import fa from '../locales/fa.json'

// Messages are bundled (not lazy-loaded) so $_ resolves synchronously with no
// loading flash — a faithful match for the original TR-object lookup.
addMessages('en', en)
addMessages('fa', fa)

const LANG_KEY = 'cfscanner_lang'
const initialLocale = localStorage.getItem(LANG_KEY) || 'en'

init({ fallbackLocale: 'en', initialLocale })

// Mirror the original setLang side effects: set <html> lang/dir AND <body> dir.
// The original only set <html>, but the stylesheet targets `body[dir="rtl"]`, so
// we set both to make every verbatim-copied RTL rule apply.
locale.subscribe((code) => {
  if (!code) return
  const dir = code === 'fa' ? 'rtl' : 'ltr'
  document.documentElement.lang = code
  document.documentElement.dir = dir
  if (document.body) document.body.dir = dir
  try {
    const title = get(_)('title')
    if (title && title !== 'title') document.title = title
  } catch {
    /* format store not ready yet — title stays as the static default */
  }
})

export function setLanguage(code) {
  localStorage.setItem(LANG_KEY, code)
  locale.set(code)
}

export function toggleLanguage() {
  setLanguage(get(locale) === 'fa' ? 'en' : 'fa')
}
