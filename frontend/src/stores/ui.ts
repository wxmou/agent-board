import { reactive, watch } from 'vue'

const THEME_KEY = 'agent-board-theme'
const SIDEBAR_KEY = 'agent-board-sidebar'

function preferredTheme(): 'dark' | 'light' {
  try {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'dark' || saved === 'light') return saved
  } catch {
    // ignore
  }
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

export const ui = reactive({
  theme: preferredTheme() as 'dark' | 'light',
  sidebarCollapsed: (() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) === '1'
    } catch {
      return false
    }
  })(),
  mobileOpen: false,
})

export function applyTheme() {
  document.documentElement.dataset.theme = ui.theme
  document.documentElement.style.colorScheme = ui.theme
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', ui.theme === 'dark' ? '#020617' : '#f4f7f9')
}

export function toggleTheme() {
  ui.theme = ui.theme === 'dark' ? 'light' : 'dark'
}

export function toggleSidebar() {
  ui.sidebarCollapsed = !ui.sidebarCollapsed
}

export function openMobileSidebar() {
  ui.mobileOpen = true
}

export function closeMobileSidebar() {
  ui.mobileOpen = false
}

watch(
  () => ui.theme,
  (value) => {
    applyTheme()
    try {
      localStorage.setItem(THEME_KEY, value)
    } catch {
      // ignore
    }
  },
  { immediate: true },
)

watch(
  () => ui.sidebarCollapsed,
  (value) => {
    try {
      localStorage.setItem(SIDEBAR_KEY, value ? '1' : '0')
    } catch {
      // ignore
    }
  },
)
