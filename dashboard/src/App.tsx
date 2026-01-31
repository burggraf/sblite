import { BrowserRouter, Routes, Route, Navigate, Outlet } from "react-router-dom"
import { AuthProvider, useAuth } from "@/contexts/AuthContext"
import { ThemeProvider } from "@/contexts/ThemeContext"
import { Toaster } from "sonner"

// Layout components
import { LayoutOutlet } from "@/components/layout/Layout"
import { AuthLayout } from "@/components/layout/AuthLayout"

// Page components
import { SetupPage } from "@/views/auth/SetupPage"
import { LoginPage } from "@/views/auth/LoginPage"
import { TablesPage } from "@/views/tables/TablesPage"
import { UsersPage } from "@/views/users/UsersPage"

// Auth Guard component
function AuthGuard() {
  const { authenticated, needsSetup, loading } = useAuth()

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

  return <Outlet />
}

// Main App component
function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <BrowserRouter basename="/_/">
          <Routes>
          {/* Auth routes */}
          <Route element={<AuthLayout />}>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="/login" element={<LoginPage />} />
          </Route>

          {/* Main app routes with sidebar layout */}
          <Route element={<AuthGuard />}>
            <Route element={<LayoutOutlet />}>
              <Route path="/tables" element={<TablesPage />} />
              <Route path="/users" element={<UsersPage />} />
              <Route path="/policies" element={<div className="p-6">Policies View - Coming Soon</div>} />
              <Route path="/storage" element={<div className="p-6">Storage View - Coming Soon</div>} />
              <Route path="/storage-policies" element={<div className="p-6">Storage Policies View - Coming Soon</div>} />
              <Route path="/functions" element={<div className="p-6">Functions View - Coming Soon</div>} />
              <Route path="/settings" element={<div className="p-6">Settings View - Coming Soon</div>} />
              <Route path="/logs" element={<div className="p-6">Logs View - Coming Soon</div>} />
              <Route path="/api-console" element={<div className="p-6">API Console - Coming Soon</div>} />
              <Route path="/sql-browser" element={<div className="p-6">SQL Browser - Coming Soon</div>} />
              <Route path="/api-docs" element={<div className="p-6">API Docs - Coming Soon</div>} />
              <Route path="/realtime" element={<div className="p-6">Realtime - Coming Soon</div>} />
              <Route path="/observability" element={<div className="p-6">Observability - Coming Soon</div>} />
              <Route path="/migration" element={<div className="p-6">Migration - Coming Soon</div>} />
              <Route path="/mail" element={<div className="p-6">Mail View - Coming Soon</div>} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
      </AuthProvider>
      <Toaster />
    </ThemeProvider>
  )
}

export default App
