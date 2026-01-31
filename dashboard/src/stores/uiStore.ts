/**
 * UI Store
 * Global UI state management
 */

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type ViewName =
  | 'tables'
  | 'users'
  | 'policies'
  | 'storage'
  | 'storage-policies'
  | 'functions'
  | 'settings'
  | 'logs'
  | 'api-console'
  | 'sql-browser'
  | 'api-docs'
  | 'realtime'
  | 'observability'
  | 'migration'
  | 'mail'

interface UIState {
  // Current view
  currentView: ViewName

  // Theme
  theme: 'light' | 'dark'

  // Sidebar
  sidebarCollapsed: boolean

  // Loading states
  globalLoading: boolean

  // Toast notifications
  toasts: Array<{
    id: string
    type: 'success' | 'error' | 'warning' | 'info'
    title: string
    message?: string
  }>

  // Actions
  setCurrentView: (view: ViewName) => void
  setTheme: (theme: 'light' | 'dark') => void
  toggleTheme: () => void
  setSidebarCollapsed: (collapsed: boolean) => void
  toggleSidebar: () => void
  setGlobalLoading: (loading: boolean) => void
  addToast: (toast: {
    type: 'success' | 'error' | 'warning' | 'info'
    title: string
    message?: string
  }) => void
  removeToast: (id: string) => void
  clearToasts: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      // Initial state
      currentView: 'tables',
      theme: 'dark',
      sidebarCollapsed: false,
      globalLoading: false,
      toasts: [],

      // Set current view
      setCurrentView: (view) => set({ currentView: view }),

      // Set theme
      setTheme: (theme) => set({ theme }),

      // Toggle theme
      toggleTheme: () => set((state) => ({ theme: state.theme === 'light' ? 'dark' : 'light' })),

      // Set sidebar collapsed state
      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),

      // Toggle sidebar
      toggleSidebar: () => set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),

      // Set global loading
      setGlobalLoading: (loading) => set({ globalLoading: loading }),

      // Add toast notification
      addToast: (toast) => {
        const id = Math.random().toString(36).substring(2, 9)
        set((state) => ({
          toasts: [...state.toasts, { id, ...toast }],
        }))
        // Auto-remove after 5 seconds
        setTimeout(() => {
          set((state) => ({
            toasts: state.toasts.filter((t) => t.id !== id),
          }))
        }, 5000)
      },

      // Remove toast
      removeToast: (id) => set((state) => ({
        toasts: state.toasts.filter((t) => t.id !== id),
      })),

      // Clear all toasts
      clearToasts: () => set({ toasts: [] }),
    }),
    {
      name: 'sblite-ui',
      partialize: (state) => ({
        theme: state.theme,
        sidebarCollapsed: state.sidebarCollapsed,
      }),
    }
  )
)

/**
 * Hook to apply theme to document
 */
export function useThemeEffect() {
  const theme = useUIStore((state) => state.theme)

  // Apply theme on mount and when theme changes
  if (typeof document !== 'undefined') {
    const root = document.documentElement
    if (theme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
  }

  return theme
}
