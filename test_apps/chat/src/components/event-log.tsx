import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Trash2 } from 'lucide-react'
import { formatTime, cn } from '@/lib/utils'
import type { LogEvent, EventType } from '@/hooks/use-event-log'

interface EventLogProps {
  events: LogEvent[]
  onClear: () => void
  filter?: EventType | null
  onFilterChange?: (filter: EventType | null) => void
}

const EVENT_COLORS: Record<EventType, string> = {
  postgres_changes: 'bg-blue-500',
  presence_join: 'bg-green-500',
  presence_leave: 'bg-orange-500',
  presence_sync: 'bg-purple-500',
  broadcast_cursor: 'bg-cyan-500',
  broadcast_reaction: 'bg-pink-500',
  connection: 'bg-gray-500',
  error: 'bg-red-500'
}

const EVENT_LABELS: Record<EventType, string> = {
  postgres_changes: 'DB',
  presence_join: 'Join',
  presence_leave: 'Leave',
  presence_sync: 'Sync',
  broadcast_cursor: 'Cursor',
  broadcast_reaction: 'Reaction',
  connection: 'Conn',
  error: 'Error'
}

export function EventLog({ events, onClear, filter, onFilterChange }: EventLogProps) {
  const filteredEvents = filter ? events.filter(e => e.type === filter) : events
  const eventTypes = Object.keys(EVENT_COLORS) as EventType[]

  return (
    <div className="flex flex-col h-full">
      {/* Header with filters */}
      <div className="p-3 border-b space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm font-medium">Event Log</span>
          <Button size="sm" variant="ghost" onClick={onClear} className="h-7">
            <Trash2 className="h-3 w-3 mr-1" />
            Clear
          </Button>
        </div>

        {onFilterChange && (
          <div className="flex flex-wrap gap-1">
            <Badge
              variant={filter === null ? 'default' : 'outline'}
              className="cursor-pointer text-xs"
              onClick={() => onFilterChange(null)}
            >
              All
            </Badge>
            {eventTypes.map((type) => (
              <Badge
                key={type}
                variant={filter === type ? 'default' : 'outline'}
                className="cursor-pointer text-xs"
                onClick={() => onFilterChange(type)}
              >
                {EVENT_LABELS[type]}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {/* Event list */}
      <ScrollArea className="flex-1">
        <div className="p-2 space-y-1">
          {filteredEvents.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              No events yet. Interact with the app to see realtime events.
            </p>
          ) : (
            filteredEvents.map((event) => (
              <EventItem key={event.id} event={event} />
            ))
          )}
        </div>
      </ScrollArea>
    </div>
  )
}

function EventItem({ event }: { event: LogEvent }) {
  return (
    <div className="flex items-start gap-2 p-2 rounded-md bg-muted/50 text-xs font-mono">
      <span className="text-muted-foreground shrink-0">
        {formatTime(event.timestamp)}
      </span>
      <div
        className={cn(
          "shrink-0 px-1.5 py-0.5 rounded text-white text-[10px] font-medium",
          EVENT_COLORS[event.type]
        )}
      >
        {EVENT_LABELS[event.type]}
      </div>
      <span className="flex-1 break-all">{event.message}</span>
    </div>
  )
}
