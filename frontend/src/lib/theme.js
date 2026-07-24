import { writable } from 'svelte/store'

// Dark is the default; light is opt-in via [data-theme="light"] on <html>,
// persisted to localStorage. Mirrors the language-toggle pattern in i18n.js.
const THEME_KEY = 'cfscanner_theme'

function readInitial() {
  try {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    /* localStorage unavailable — fall through to default */
  }
  return 'dark'
}

export const theme = writable(readInitial())

// Apply to <html> and persist. Called once at startup (before mount, to avoid a
// flash) and on every toggle.
export function applyTheme(value) {
  const t = value === 'light' ? 'light' : 'dark'
  if (t === 'light') document.documentElement.setAttribute('data-theme', 'light')
  else document.documentElement.removeAttribute('data-theme')
  try {
    localStorage.setItem(THEME_KEY, t)
  } catch {
    /* ignore persistence failure */
  }
}

export function setTheme(value) {
  const t = value === 'light' ? 'light' : 'dark'
  theme.set(t)
  applyTheme(t)
}

export function toggleTheme() {
  let current
  theme.update((v) => {
    current = v === 'light' ? 'dark' : 'light'
    return current
  })
  applyTheme(current)
}

// Apply the persisted theme immediately (call before mount so first paint is
// correct and there's no dark→light flash).
export function initTheme() {
  applyTheme(readInitial())
}
