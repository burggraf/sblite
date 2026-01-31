/**
 * Auth Context
 * Simple authentication state management using React Context
 */

import { createContext, useContext, useState, useEffect, ReactNode } from 'react'

interface AuthState {
  authenticated: boolean
  needsSetup: boolean
  loading: boolean
}

interface AuthContextValue extends AuthState {
  login: (password: string) => Promise<void>
  setup: (password: string) => Promise<void>
  logout: () => Promise<void>
  checkStatus: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    authenticated: false,
    needsSetup: false,
    loading: true,
  })

  const checkStatus = async () => {
    setState((prev) => ({ ...prev, loading: true }))
    try {
      const res = await fetch('/_/api/auth/status')
      if (res.ok) {
        const data = await res.json()
        setState({
          authenticated: data.authenticated,
          needsSetup: data.needs_setup,
          loading: false,
        })
      } else {
        setState({ authenticated: false, needsSetup: false, loading: false })
      }
    } catch {
      setState({ authenticated: false, needsSetup: false, loading: false })
    }
  }

  const login = async (password: string) => {
    setState((prev) => ({ ...prev, loading: true }))
    try {
      const res = await fetch('/_/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      if (res.ok) {
        setState({ authenticated: true, needsSetup: false, loading: false })
      } else {
        const error = await res.json()
        throw new Error(error.error || 'Login failed')
      }
    } finally {
      setState((prev) => ({ ...prev, loading: false }))
    }
  }

  const setup = async (password: string) => {
    setState((prev) => ({ ...prev, loading: true }))
    try {
      const res = await fetch('/_/api/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      if (res.ok) {
        setState({ authenticated: true, needsSetup: false, loading: false })
      } else {
        const error = await res.json()
        throw new Error(error.error || 'Setup failed')
      }
    } finally {
      setState((prev) => ({ ...prev, loading: false }))
    }
  }

  const logout = async () => {
    setState((prev) => ({ ...prev, loading: true }))
    try {
      await fetch('/_/api/auth/logout', { method: 'POST' })
      setState({ authenticated: false, needsSetup: false, loading: false })
    } finally {
      setState((prev) => ({ ...prev, loading: false }))
    }
  }

  // Check auth status on mount
  useEffect(() => {
    checkStatus()
  }, [])

  return (
    <AuthContext.Provider
      value={{
        ...state,
        login,
        setup,
        logout,
        checkStatus,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
