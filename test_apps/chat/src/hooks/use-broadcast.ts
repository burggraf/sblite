import { useState, useEffect, useCallback, useRef } from 'react'
import { supabase } from '@/lib/supabase/client'
import type { RealtimeChannel } from '@supabase/supabase-js'

export interface CursorPosition {
  id: string
  username: string
  x: number
  y: number
  color: string
  lastUpdate: number
}

interface UseBroadcastOptions {
  channelName?: string
  onCursor?: (cursor: CursorPosition) => void
  onReaction?: (reaction: ReactionEvent) => void
}

export interface ReactionEvent {
  id: string
  username: string
  emoji: string
  x: number
  y: number
  color: string
}

function generateColor(seed: string): string {
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = seed.charCodeAt(i) + ((hash << 5) - hash)
  }
  const hue = Math.abs(hash % 360)
  return `hsl(${hue}, 70%, 50%)`
}

const CURSOR_TIMEOUT = 5000 // Remove cursor after 5 seconds of inactivity

export function useBroadcast(username: string, options: UseBroadcastOptions = {}) {
  const {
    channelName = 'broadcast-room',
    onCursor,
    onReaction
  } = options

  const [cursors, setCursors] = useState<Map<string, CursorPosition>>(new Map())
  const [reactions, setReactions] = useState<ReactionEvent[]>([])
  const channelRef = useRef<RealtimeChannel | null>(null)
  const userIdRef = useRef<string>(crypto.randomUUID())
  const cleanupIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!username) return

    const channel = supabase.channel(channelName, {
      config: {
        broadcast: { self: false }
      }
    })

    channel
      .on('broadcast', { event: 'cursor' }, ({ payload }) => {
        const cursor: CursorPosition = {
          id: payload.id,
          username: payload.username,
          x: payload.x,
          y: payload.y,
          color: payload.color,
          lastUpdate: Date.now()
        }

        setCursors(prev => {
          const next = new Map(prev)
          next.set(cursor.id, cursor)
          return next
        })

        onCursor?.(cursor)
      })
      .on('broadcast', { event: 'reaction' }, ({ payload }) => {
        const reaction: ReactionEvent = {
          id: payload.id,
          username: payload.username,
          emoji: payload.emoji,
          x: payload.x,
          y: payload.y,
          color: payload.color
        }

        setReactions(prev => [...prev, reaction])
        onReaction?.(reaction)

        // Remove reaction after animation
        setTimeout(() => {
          setReactions(prev => prev.filter(r => r !== reaction))
        }, 2000)
      })
      .subscribe()

    channelRef.current = channel

    // Cleanup stale cursors periodically
    cleanupIntervalRef.current = setInterval(() => {
      const now = Date.now()
      setCursors(prev => {
        const next = new Map(prev)
        for (const [id, cursor] of next) {
          if (now - cursor.lastUpdate > CURSOR_TIMEOUT) {
            next.delete(id)
          }
        }
        return next
      })
    }, 1000)

    return () => {
      channel.unsubscribe()
      channelRef.current = null
      if (cleanupIntervalRef.current) {
        clearInterval(cleanupIntervalRef.current)
      }
    }
  }, [username, channelName, onCursor, onReaction])

  const sendCursor = useCallback((x: number, y: number) => {
    if (!channelRef.current) return

    const userId = userIdRef.current
    const userColor = generateColor(username + userId)

    channelRef.current.send({
      type: 'broadcast',
      event: 'cursor',
      payload: {
        id: userId,
        username,
        x,
        y,
        color: userColor
      }
    })
  }, [username])

  const sendReaction = useCallback((emoji: string, x: number, y: number) => {
    if (!channelRef.current) return

    const userId = userIdRef.current
    const userColor = generateColor(username + userId)

    channelRef.current.send({
      type: 'broadcast',
      event: 'reaction',
      payload: {
        id: crypto.randomUUID(),
        username,
        emoji,
        x,
        y,
        color: userColor
      }
    })
  }, [username])

  return {
    cursors: Array.from(cursors.values()),
    reactions,
    sendCursor,
    sendReaction,
    currentUserId: userIdRef.current
  }
}
