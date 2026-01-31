import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function DataPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Data Browser</h1>
        <p className="text-muted-foreground">
          View and edit your database records
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Select a table</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Choose a table from the sidebar to view its data.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
