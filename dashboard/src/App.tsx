import { BrowserRouter, Routes, Route, Navigate, Outlet } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import { useAuthStore } from "@/stores/authStore"
import { useThemeEffect } from "@/hooks"
import { Toaster } from "sonner"
import type { ReactNode } from "react"

// Layout components
import { Layout } from "@/components/layout/Layout"
import { AuthLayout } from "@/components/layout/AuthLayout"

// Page components
import { SetupPage } from "@/views/auth/SetupPage"
import { LoginPage } from "@/views/auth/LoginPage"
import { TablesPage } from "@/views/tables/TablesPage"

// Create a client for React Query
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000, // 30 seconds
      retry: 1,
    },
  },
})

// Auth Guard component
function AuthGuard({ children }: { children?: ReactNode }) {
  const { authenticated, needsSetup, loading } = useAuthStore()

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (needsSetup) {
    return <Navigate to="/setup" replace />
  }

  if (!authenticated) {
    return <Navigate to="/login" replace />
  }

  // Render children if provided, otherwise Outlet for nested routes
  return children ? <>{children}</> : <Outlet />
}

// Main App component
function App() {
  // Apply theme to document
  useThemeEffect()

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename="/_/">
        <Routes>
          {/* Auth routes */}
          <Route element={<AuthLayout />}>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="/login" element={<LoginPage />} />
          </Route>

          {/* Main app routes */}
          <Route path="/tables" element={<TablesPage />} />
          <Route path="/users" element={<AuthGuard><Layout><div className="p-6">Users View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/policies" element={<AuthGuard><Layout><div className="p-6">Policies View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/storage" element={<AuthGuard><Layout><div className="p-6">Storage View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/storage-policies" element={<AuthGuard><Layout><div className="p-6">Storage Policies View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/functions" element={<AuthGuard><Layout><div className="p-6">Functions View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/settings" element={<AuthGuard><Layout><div className="p-6">Settings View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/logs" element={<AuthGuard><Layout><div className="p-6">Logs View - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/api-console" element={<AuthGuard><Layout><div className="p-6">API Console - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/sql-browser" element={<AuthGuard><Layout><div className="p-6">SQL Browser - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/api-docs" element={<AuthGuard><Layout><div className="p-6">API Docs - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/realtime" element={<AuthGuard><Layout><div className="p-6">Realtime - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/observability" element={<AuthGuard><Layout><div className="p-6">Observability - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/migration" element={<AuthGuard><Layout><div className="p-6">Migration - Coming Soon</div></Layout></AuthGuard>} />
          <Route path="/mail" element={<AuthGuard><Layout><div className="p-6">Mail View - Coming Soon</div></Layout></AuthGuard>} />
        </Routes>
      </BrowserRouter>
      <Toaster />
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  )
}

export default App
