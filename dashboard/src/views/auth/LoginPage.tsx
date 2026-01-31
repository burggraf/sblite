/**
 * Login Page
 * Dashboard authentication
 */

import { useState } from "react"
import { useAuthStore } from "@/stores/authStore"
import { useToast } from "@/hooks"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export function LoginPage() {
  const { login, loading, error } = useAuthStore()
  const { success, error: showError } = useToast()

  const [password, setPassword] = useState("")

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()

    try {
      await login(password)
      success("Login successful", "Welcome back!")
      // Force navigation using window.location
      window.location.href = "/_/tables"
    } catch {
      showError("Login failed", error || "Invalid password. Please try again.")
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Dashboard Login</CardTitle>
        <CardDescription>
          Enter your dashboard password to continue.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter your password"
              disabled={loading}
              autoFocus
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button type="submit" className="w-full" disabled={loading || !password}>
            {loading ? "Logging in..." : "Login"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
