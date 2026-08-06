import { writable } from 'svelte/store'

const THEME_KEY = 'cfscanner_theme'

function readInitial() {
  try {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {

  }
  return 'dark'
}

export const theme = writable(readInitial())

export function applyTheme(value) {
  const t = value === 'light' ? 'light' : 'dark'
  if (t === 'light') document.documentElement.setAttribute('data-theme', 'light')
  else document.documentElement.removeAttribute('data-theme')
  try {
    localStorage.setItem(THEME_KEY, t)
  } catch {

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

export function initTheme() {
  applyTheme(readInitial())
}
