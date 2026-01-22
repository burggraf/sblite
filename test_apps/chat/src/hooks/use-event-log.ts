import { useState, useCallback } from 'react'

export type EventType =
  | 'postgres_changes'
  | 'presence_join'
  | 'presence_leave'
  | 'presence_sync'
  | 'broadcast_cursor'
  | 'broadcast_reaction'
  | 'connection'
  | 'error'

export interface LogEvent {
  id: string
  type: EventType
  message: string
  data?: unknown
  timestamp: Date
}

const MAX_EVENTS = 100

export function useEventLog() {
  const [events, setEvents] = useState<LogEvent[]>([])

  const addEvent = useCallback((type: EventType, message: string, data?: unknown) => {
    const event: LogEvent = {
      id: crypto.randomUUID(),
      type,
      message,
      data,
      timestamp: new Date()
    }

    setEvents(prev => {
      const next = [event, ...prev]
      if (next.length > MAX_EVENTS) {
        return next.slice(0, MAX_EVENTS)
      }
      return next
    })

    return event
  }, [])

  const clearEvents = useCallback(() => {
    setEvents([])
  }, [])

  const filterByType = useCallback((type: EventType | null) => {
    if (!type) return events
    return events.filter(e => e.type === type)
  }, [events])

  return {
    events,
    addEvent,
    clearEvents,
    filterByType
  }
}
