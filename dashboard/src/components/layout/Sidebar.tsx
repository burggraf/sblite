/**
 * Sidebar Navigation
 */

import { Link, useLocation } from "react-router-dom"
import { useTheme } from "@/contexts/ThemeContext"
import { cn } from "@/lib/utils"

interface NavItem {
  path: string
  label: string
  icon: string
}

const NAV_ITEMS: NavItem[] = [
  { path: "/tables", label: "Tables", icon: "📊" },
  { path: "/users", label: "Users", icon: "👥" },
  { path: "/policies", label: "Policies", icon: "🔒" },
  { path: "/storage", label: "Storage", icon: "📁" },
  { path: "/storage-policies", label: "Storage Policies", icon: "📋" },
  { path: "/functions", label: "Functions", icon: "⚡" },
  { path: "/settings", label: "Settings", icon: "⚙️" },
  { path: "/logs", label: "Logs", icon: "📝" },
  { path: "/api-console", label: "API Console", icon: "🖥️" },
  { path: "/sql-browser", label: "SQL Browser", icon: "🔍" },
  { path: "/api-docs", label: "API Docs", icon: "📚" },
  { path: "/realtime", label: "Realtime", icon: "🔄" },
  { path: "/observability", label: "Observability", icon: "📈" },
  { path: "/migration", label: "Migration", icon: "🚀" },
  { path: "/mail", label: "Mail", icon: "✉️" },
]

interface SidebarProps {
  collapsed: boolean
  onToggle: () => void
}

export function Sidebar({ collapsed, onToggle }: SidebarProps) {
  const location = useLocation()
  const { theme, toggleTheme } = useTheme()

  return (
    <aside
      className={cn(
        "flex flex-col border-r border-border bg-muted/30 transition-all duration-300",
        collapsed ? "w-16" : "w-64"
      )}
    >
      {/* Header */}
      <div className="flex h-14 items-center justify-between border-b border-border px-4">
        {!collapsed && (
          <Link to="/tables" className="flex items-center gap-2 font-semibold">
            <span className="text-lg">SB</span>
            <span className="text-sm">sblite Dashboard</span>
          </Link>
        )}
        {collapsed && (
          <Link to="/tables" className="flex items-center justify-center">
            <span className="text-lg font-semibold">SB</span>
          </Link>
        )}
        <button
          onClick={onToggle}
          className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          aria-label="Toggle sidebar"
        >
          {collapsed ? "→" : "←"}
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4">
        <ul className="space-y-1 px-2">
          {NAV_ITEMS.map((item) => {
            const isActive = location.pathname === item.path
            return (
              <li key={item.path}>
                <Link
                  to={item.path}
                  className={cn(
                    "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                    isActive
                      ? "bg-primary text-primary-foreground font-medium"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  )}
                  title={collapsed ? item.label : undefined}
                >
                  <span className="text-base" aria-hidden="true">
                    {item.icon}
                  </span>
                  {!collapsed && <span>{item.label}</span>}
                </Link>
              </li>
            )
          })}
        </ul>
      </nav>

      {/* Theme Toggle */}
      <div className="border-t border-border p-4">
        <button
          onClick={toggleTheme}
          className={cn(
            "flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors text-muted-foreground hover:bg-muted hover:text-foreground"
          )}
          title={collapsed ? (theme === "dark" ? "Light mode" : "Dark mode") : undefined}
        >
          <span className="text-base" aria-hidden="true">
            {theme === "dark" ? "☀️" : "🌙"}
          </span>
          {!collapsed && (
            <span>{theme === "dark" ? "Light Mode" : "Dark Mode"}</span>
          )}
        </button>
      </div>
    </aside>
  )
}
