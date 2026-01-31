import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function UsersPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Users</h1>
        <p className="text-muted-foreground">
          Manage user accounts and authentication
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>User Management</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            View, create, and manage user accounts.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
