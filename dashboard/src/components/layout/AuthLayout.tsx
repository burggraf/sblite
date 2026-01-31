/**
 * Auth Layout
 * Centered card layout for authentication pages
 */

import { Link, Outlet } from "react-router-dom"

export function AuthLayout() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4">
      <div className="w-full max-w-md">
        <div className="mb-8 text-center">
          <Link to="/">
            <div className="inline-flex items-center gap-2">
              <div className="h-8 w-8 rounded-lg bg-primary flex items-center justify-center">
                <span className="text-primary-foreground font-bold text-sm">SB</span>
              </div>
              <span className="text-xl font-bold">sblite Dashboard</span>
            </div>
          </Link>
        </div>
        <Outlet />
      </div>
    </div>
  )
}
