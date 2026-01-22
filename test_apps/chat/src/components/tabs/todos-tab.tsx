import { useState, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { TodoItem } from '@/components/todo-item'
import { TodoForm } from '@/components/todo-form'
import { PresenceSidebar } from '@/components/presence-sidebar'
import { useTodos, type TodoEvent } from '@/hooks/use-todos'
import { usePresence, type PresenceUser } from '@/hooks/use-presence'
import { Database, ListTodo } from 'lucide-react'

interface TodosTabProps {
  username: string
  onTodoEvent?: (event: TodoEvent) => void
  onPresenceJoin?: (user: PresenceUser) => void
  onPresenceLeave?: (user: PresenceUser) => void
  onPresenceSync?: (users: PresenceUser[]) => void
}

export function TodosTab({
  username,
  onTodoEvent,
  onPresenceJoin,
  onPresenceLeave,
  onPresenceSync
}: TodosTabProps) {
  const [highlightedId, setHighlightedId] = useState<string | null>(null)

  const handleTodoEvent = useCallback((event: TodoEvent) => {
    onTodoEvent?.(event)

    // Highlight new/updated items
    if (event.type === 'INSERT' || event.type === 'UPDATE') {
      const id = event.todo?.id
      if (id) {
        setHighlightedId(id)
        setTimeout(() => setHighlightedId(null), 2000)
      }
    }
  }, [onTodoEvent])

  const { todos, loading, error, addTodo, updateTodo, deleteTodo } = useTodos({
    onEvent: handleTodoEvent
  })

  const { onlineUsers, currentUserId } = usePresence(username, {
    channelName: 'todos-presence',
    onJoin: onPresenceJoin,
    onLeave: onPresenceLeave,
    onSync: onPresenceSync
  })

  const handleAddTodo = useCallback(async (title: string) => {
    await addTodo(title, username)
  }, [addTodo, username])

  const handleToggle = useCallback(async (id: string, completed: boolean) => {
    await updateTodo(id, { completed })
  }, [updateTodo])

  const handleUpdate = useCallback(async (id: string, title: string) => {
    await updateTodo(id, { title })
  }, [updateTodo])

  const handleDelete = useCallback(async (id: string) => {
    await deleteTodo(id)
  }, [deleteTodo])

  const completedCount = todos.filter(t => t.completed).length
  const totalCount = todos.length

  return (
    <div className="flex h-full">
      {/* Main content */}
      <div className="flex-1 flex flex-col p-4 gap-4">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <Database className="h-4 w-4" />
              Postgres Changes Demo
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground mb-4">
              Add, edit, or delete todos. Changes sync in realtime via <code className="bg-muted px-1 rounded">postgres_changes</code> subscriptions.
              Open multiple browser tabs to see the magic!
            </p>
            <TodoForm onAdd={handleAddTodo} disabled={!username} />
          </CardContent>
        </Card>

        <Card className="flex-1 flex flex-col min-h-0">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <ListTodo className="h-4 w-4" />
              Todos
              <span className="text-sm font-normal text-muted-foreground ml-auto">
                {completedCount}/{totalCount} completed
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="flex-1 min-h-0 pb-4">
            {loading ? (
              <div className="flex items-center justify-center h-full">
                <p className="text-muted-foreground">Loading todos...</p>
              </div>
            ) : error ? (
              <div className="flex items-center justify-center h-full">
                <p className="text-destructive">{error}</p>
              </div>
            ) : todos.length === 0 ? (
              <div className="flex items-center justify-center h-full">
                <p className="text-muted-foreground">No todos yet. Add one above!</p>
              </div>
            ) : (
              <ScrollArea className="h-full pr-2">
                <div className="space-y-2">
                  {todos.map((todo) => (
                    <TodoItem
                      key={todo.id}
                      todo={todo}
                      onToggle={handleToggle}
                      onUpdate={handleUpdate}
                      onDelete={handleDelete}
                      isHighlighted={highlightedId === todo.id}
                    />
                  ))}
                </div>
              </ScrollArea>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Presence sidebar */}
      <PresenceSidebar users={onlineUsers} currentUserId={currentUserId} />
    </div>
  )
}
