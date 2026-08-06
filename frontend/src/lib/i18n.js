import { addMessages, init, locale, _ } from 'svelte-i18n'
import { get } from 'svelte/store'
import en from '../locales/en.json'
import fa from '../locales/fa.json'

addMessages('en', en)
addMessages('fa', fa)

const LANG_KEY = 'cfscanner_lang'
const initialLocale = localStorage.getItem(LANG_KEY) || 'en'

init({ fallbackLocale: 'en', initialLocale })

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

  }
})

export function setLanguage(code) {
  localStorage.setItem(LANG_KEY, code)
  locale.set(code)
}

export function toggleLanguage() {
  setLanguage(get(locale) === 'fa' ? 'en' : 'fa')
}
