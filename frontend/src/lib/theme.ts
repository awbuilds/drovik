import { create } from 'zustand'

export type Theme = 'dark' | 'light'

const STORAGE_KEY = 'drovik-theme'

function readInitialTheme(): Theme {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    if (stored === 'dark' || stored === 'light') {
      return stored
    }
    if (window.matchMedia?.('(prefers-color-scheme: light)').matches) {
      return 'light'
    }
  } catch {
    // Storage or matchMedia can fail in hardened browsers; dark is the safe default.
  }
  return 'dark'
}

function applyTheme(theme: Theme) {
  try {
    document.documentElement.dataset.theme = theme
    const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    if (meta) {
      meta.content = theme === 'dark' ? '#07090c' : '#f4f1ea'
    }
  } catch {
    // A failed theme application must never crash the app.
  }
}

interface ThemeStore {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
}

export const useThemeStore = create<ThemeStore>((set, get) => ({
  theme: typeof window === 'undefined' ? 'dark' : readInitialTheme(),
  setTheme: (theme) => {
    applyTheme(theme)
    try {
      window.localStorage.setItem(STORAGE_KEY, theme)
    } catch {
      // Persisting a theme is optional.
    }
    set({ theme })
  },
  toggleTheme: () => {
    const next = get().theme === 'dark' ? 'light' : 'dark'
    get().setTheme(next)
  },
}))

export function initializeTheme() {
  applyTheme(useThemeStore.getState().theme)
}
