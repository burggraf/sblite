/**
 * Auth Store
 * Authentication and session state management
 */

import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { authApi } from '@/lib/api-client'

interface AuthState {
  // State
  authenticated: boolean
  needsSetup: boolean
  loading: boolean
  error: string | null

  // Actions
  checkStatus: () => Promise<void>
  setup: (password: string) => Promise<void>
  login: (password: string) => Promise<void>
  logout: () => Promise<void>
  clearError: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      // Initial state
      authenticated: false,
      needsSetup: false,
      loading: false,
      error: null,

      // Check authentication status
      checkStatus: async () => {
        set({ loading: true, error: null })
        try {
          const status = await authApi.getStatus()
          set({
            authenticated: status.authenticated,
            needsSetup: status.needs_setup,
            loading: false,
          })
        } catch (error) {
          set({
            error: error instanceof Error ? error.message : 'Failed to check auth status',
            loading: false,
          })
        }
      },

      // Setup initial password
      setup: async (password: string) => {
        set({ loading: true, error: null })
        try {
          await authApi.setup(password)
          set({
            authenticated: true,
            needsSetup: false,
            loading: false,
          })
        } catch (error) {
          set({
            error: error instanceof Error ? error.message : 'Setup failed',
            loading: false,
          })
          throw error
        }
      },

      // Login
      login: async (password: string) => {
        set({ loading: true, error: null })
        try {
          await authApi.login(password)
          set({
            authenticated: true,
            loading: false,
          })
        } catch (error) {
          set({
            error: error instanceof Error ? error.message : 'Login failed',
            loading: false,
          })
          throw error
        }
      },

      // Logout
      logout: async () => {
        set({ loading: true, error: null })
        try {
          await authApi.logout()
          set({
            authenticated: false,
            loading: false,
          })
        } catch (error) {
          set({
            error: error instanceof Error ? error.message : 'Logout failed',
            loading: false,
          })
        }
      },

      // Clear error
      clearError: () => set({ error: null }),
    }),
    {
      name: 'sblite-auth',
      // Only persist authenticated state
      partialize: (state) => ({ authenticated: state.authenticated }),
    }
  )
)
