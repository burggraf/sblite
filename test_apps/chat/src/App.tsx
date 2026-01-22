import { useState, useCallback } from 'react'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { AppHeader } from '@/components/app-header'
import { TodosTab } from '@/components/tabs/todos-tab'
import { BroadcastTab } from '@/components/tabs/broadcast-tab'
import { StatusTab } from '@/components/tabs/status-tab'
import { useEventLog } from '@/hooks/use-event-log'
import type { TodoEvent } from '@/hooks/use-todos'
import type { PresenceUser } from '@/hooks/use-presence'
import type { CursorPosition, ReactionEvent } from '@/hooks/use-broadcast'
import { Database, Radio, Activity } from 'lucide-react'

function generateRandomUsername(): string {
  const adjectives = ['Happy', 'Swift', 'Bright', 'Cool', 'Calm']
  const nouns = ['Panda', 'Eagle', 'Tiger', 'Dolphin', 'Fox']
  const adj = adjectives[Math.floor(Math.random() * adjectives.length)]
  const noun = nouns[Math.floor(Math.random() * nouns.length)]
  const num = Math.floor(Math.random() * 100)
  return `${adj}${noun}${num}`
}

export function App() {
  const [username, setUsername] = useState(generateRandomUsername)
  const [activeTab, setActiveTab] = useState('todos')
  const [isConnected, setIsConnected] = useState(false)
  const { events, addEvent, clearEvents } = useEventLog()

  // Event handlers for logging
  const handleTodoEvent = useCallback((event: TodoEvent) => {
    const action = event.type.toLowerCase()
    const title = event.todo?.title || event.old?.title || 'unknown'
    addEvent('postgres_changes', `${action}: "${title}"`, event)
  }, [addEvent])

  const handlePresenceJoin = useCallback((user: PresenceUser) => {
    addEvent('presence_join', `${user.username} joined`, user)
    setIsConnected(true)
  }, [addEvent])

  const handlePresenceLeave = useCallback((user: PresenceUser) => {
    addEvent('presence_leave', `${user.username} left`, user)
  }, [addEvent])

  const handlePresenceSync = useCallback((users: PresenceUser[]) => {
    addEvent('presence_sync', `${users.length} user(s) online`, users)
    setIsConnected(true)
  }, [addEvent])

  const handleCursor = useCallback((cursor: CursorPosition) => {
    // Only log occasionally to avoid spam
    if (Math.random() < 0.1) {
      addEvent('broadcast_cursor', `${cursor.username} moved cursor`, cursor)
    }
  }, [addEvent])

  const handleReaction = useCallback((reaction: ReactionEvent) => {
    addEvent('broadcast_reaction', `${reaction.username} sent ${reaction.emoji}`, reaction)
  }, [addEvent])

  return (
    <div className="h-screen w-screen flex flex-col bg-background">
      <AppHeader
        username={username}
        onUsernameChange={setUsername}
        isConnected={isConnected}
      />

      <Tabs value={activeTab} onValueChange={setActiveTab} className="flex-1 flex flex-col min-h-0">
        <div className="border-b px-4">
          <TabsList className="h-12">
            <TabsTrigger value="todos" className="gap-2">
              <Database className="h-4 w-4" />
              Todos
            </TabsTrigger>
            <TabsTrigger value="broadcast" className="gap-2">
              <Radio className="h-4 w-4" />
              Broadcast
            </TabsTrigger>
            <TabsTrigger value="status" className="gap-2">
              <Activity className="h-4 w-4" />
              Status
            </TabsTrigger>
          </TabsList>
        </div>

        <div className="flex-1 min-h-0">
          <TabsContent value="todos" className="h-full m-0">
            <TodosTab
              username={username}
              onTodoEvent={handleTodoEvent}
              onPresenceJoin={handlePresenceJoin}
              onPresenceLeave={handlePresenceLeave}
              onPresenceSync={handlePresenceSync}
            />
          </TabsContent>

          <TabsContent value="broadcast" className="h-full m-0">
            <BroadcastTab
              username={username}
              onCursor={handleCursor}
              onReaction={handleReaction}
            />
          </TabsContent>

          <TabsContent value="status" className="h-full m-0">
            <StatusTab
              events={events}
              onClearEvents={clearEvents}
              isConnected={isConnected}
            />
          </TabsContent>
        </div>
      </Tabs>
    </div>
  )
}

export default App
