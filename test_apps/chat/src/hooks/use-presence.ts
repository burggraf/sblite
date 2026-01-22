import { useState, useEffect, useCallback, useRef } from 'react'
import { supabase } from '@/lib/supabase/client'
import type { RealtimeChannel } from '@supabase/supabase-js'

export interface PresenceUser {
  id: string
  username: string
  online_at: string
  color: string
}

interface UsePresenceOptions {
  channelName?: string
  onJoin?: (user: PresenceUser) => void
  onLeave?: (user: PresenceUser) => void
  onSync?: (users: PresenceUser[]) => void
}

function generateColor(seed: string): string {
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = seed.charCodeAt(i) + ((hash << 5) - hash)
  }
  const hue = Math.abs(hash % 360)
  return `hsl(${hue}, 70%, 50%)`
}

export function usePresence(username: string, options: UsePresenceOptions = {}) {
  const {
    channelName = 'presence-room',
    onJoin,
    onLeave,
    onSync
  } = options

  const [onlineUsers, setOnlineUsers] = useState<PresenceUser[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const channelRef = useRef<RealtimeChannel | null>(null)
  const userIdRef = useRef<string>(crypto.randomUUID())

  const parsePresenceState = useCallback((state: Record<string, unknown[]>) => {
    const users: PresenceUser[] = []
    for (const presences of Object.values(state)) {
      for (const presence of presences) {
        const p = presence as Record<string, unknown>
        users.push({
          id: p.id as string,
          username: p.username as string,
          online_at: p.online_at as string,
          color: p.color as string
        })
      }
    }
    return users
  }, [])

  useEffect(() => {
    if (!username) return

    const userId = userIdRef.current
    const userColor = generateColor(username + userId)

    const channel = supabase.channel(channelName, {
      config: {
        presence: { key: userId }
      }
    })

    channel
      .on('presence', { event: 'sync' }, () => {
        const state = channel.presenceState()
        const users = parsePresenceState(state)
        setOnlineUsers(users)
        onSync?.(users)
      })
      .on('presence', { event: 'join' }, ({ newPresences }) => {
        for (const presence of newPresences) {
          const p = presence as Record<string, unknown>
          onJoin?.({
            id: p.id as string,
            username: p.username as string,
            online_at: p.online_at as string,
            color: p.color as string
          })
        }
      })
      .on('presence', { event: 'leave' }, ({ leftPresences }) => {
        for (const presence of leftPresences) {
          const p = presence as Record<string, unknown>
          onLeave?.({
            id: p.id as string,
            username: p.username as string,
            online_at: p.online_at as string,
            color: p.color as string
          })
        }
      })
      .subscribe(async (status) => {
        if (status === 'SUBSCRIBED') {
          setIsConnected(true)
          await channel.track({
            id: userId,
            username,
            online_at: new Date().toISOString(),
            color: userColor
          })
        } else if (status === 'CLOSED' || status === 'CHANNEL_ERROR') {
          setIsConnected(false)
        }
      })

    channelRef.current = channel

    return () => {
      channel.unsubscribe()
      channelRef.current = null
    }
  }, [username, channelName, onJoin, onLeave, onSync, parsePresenceState])

  const updatePresence = useCallback(async (data: Partial<PresenceUser>) => {
    if (!channelRef.current) return

    const userId = userIdRef.current
    const userColor = generateColor(username + userId)

    await channelRef.current.track({
      id: userId,
      username,
      online_at: new Date().toISOString(),
      color: userColor,
      ...data
    })
  }, [username])

  return {
    onlineUsers,
    isConnected,
    updatePresence,
    currentUserId: userIdRef.current
  }
}
