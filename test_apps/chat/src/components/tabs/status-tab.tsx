import { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { EventLog } from '@/components/event-log'
import type { LogEvent, EventType } from '@/hooks/use-event-log'
import { Activity, Server, Wifi, WifiOff } from 'lucide-react'

interface StatusTabProps {
  events: LogEvent[]
  onClearEvents: () => void
  isConnected: boolean
}

export function StatusTab({ events, onClearEvents, isConnected }: StatusTabProps) {
  const [filter, setFilter] = useState<EventType | null>(null)

  // Count events by type
  const eventCounts = events.reduce((acc, event) => {
    acc[event.type] = (acc[event.type] || 0) + 1
    return acc
  }, {} as Record<EventType, number>)

  return (
    <div className="h-full p-4 flex flex-col gap-4">
      {/* Connection & Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Server className="h-4 w-4" />
              Connection
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {isConnected ? (
                <>
                  <Wifi className="h-5 w-5 text-green-500" />
                  <span className="text-lg font-semibold text-green-500">Connected</span>
                </>
              ) : (
                <>
                  <WifiOff className="h-5 w-5 text-red-500" />
                  <span className="text-lg font-semibold text-red-500">Disconnected</span>
                </>
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              WebSocket to sblite realtime server
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Activity className="h-4 w-4" />
              Events Received
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{events.length}</span>
            <p className="text-xs text-muted-foreground mt-1">
              Total realtime events captured
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Event Breakdown</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-1">
              {eventCounts.postgres_changes && (
                <Badge variant="outline" className="text-xs">
                  DB: {eventCounts.postgres_changes}
                </Badge>
              )}
              {eventCounts.presence_join && (
                <Badge variant="outline" className="text-xs">
                  Join: {eventCounts.presence_join}
                </Badge>
              )}
              {eventCounts.presence_leave && (
                <Badge variant="outline" className="text-xs">
                  Leave: {eventCounts.presence_leave}
                </Badge>
              )}
              {eventCounts.broadcast_cursor && (
                <Badge variant="outline" className="text-xs">
                  Cursor: {eventCounts.broadcast_cursor}
                </Badge>
              )}
              {eventCounts.broadcast_reaction && (
                <Badge variant="outline" className="text-xs">
                  Reaction: {eventCounts.broadcast_reaction}
                </Badge>
              )}
              {Object.keys(eventCounts).length === 0 && (
                <span className="text-xs text-muted-foreground">No events yet</span>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Event Log */}
      <Card className="flex-1 min-h-0 flex flex-col">
        <CardContent className="flex-1 min-h-0 p-0">
          <EventLog
            events={events}
            onClear={onClearEvents}
            filter={filter}
            onFilterChange={setFilter}
          />
        </CardContent>
      </Card>
    </div>
  )
}
