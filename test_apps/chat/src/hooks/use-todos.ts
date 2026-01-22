import { useState, useEffect, useCallback, useRef } from 'react'
import { supabase } from '@/lib/supabase/client'
import { generateId } from '@/lib/utils'
import type { RealtimePostgresChangesPayload } from '@supabase/supabase-js'

export interface Todo {
  id: string
  title: string
  completed: boolean
  author: string
  created_at: string
}

interface UseTodosOptions {
  onEvent?: (event: TodoEvent) => void
}

export interface TodoEvent {
  type: 'INSERT' | 'UPDATE' | 'DELETE'
  todo: Todo | null
  old?: Todo | null
  timestamp: Date
}

export function useTodos({ onEvent }: UseTodosOptions = {}) {
  const [todos, setTodos] = useState<Todo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const channelRef = useRef<ReturnType<typeof supabase.channel> | null>(null)

  // Fetch initial todos
  const fetchTodos = useCallback(async () => {
    setLoading(true)
    setError(null)

    const { data, error: fetchError } = await supabase
      .from('todos')
      .select('*')
      .order('created_at', { ascending: true })

    if (fetchError) {
      setError(fetchError.message)
      setLoading(false)
      return
    }

    setTodos((data || []).map(t => ({
      ...t,
      completed: Boolean(t.completed)
    })))
    setLoading(false)
  }, [])

  // Subscribe to realtime changes
  useEffect(() => {
    fetchTodos()

    const channel = supabase.channel('todos-changes')
      .on(
        'postgres_changes',
        { event: '*', schema: 'public', table: 'todos' },
        (payload: RealtimePostgresChangesPayload<Todo>) => {
          // Supabase protocol uses 'eventType' for the type field
          const eventType = (payload.eventType || (payload as any).type) as 'INSERT' | 'UPDATE' | 'DELETE'
          // Supabase protocol uses 'new'/'old' but raw protocol uses 'record'/'old_record'
          const newRecord = (payload.new || (payload as any).record) as Todo | null
          const oldRecord = (payload.old || (payload as any).old_record) as Todo | null

          // Notify event listener
          onEvent?.({
            type: eventType,
            todo: newRecord ? { ...newRecord, completed: Boolean(newRecord.completed) } : null,
            old: oldRecord ? { ...oldRecord, completed: Boolean(oldRecord.completed) } : null,
            timestamp: new Date()
          })

          // Update local state
          setTodos(current => {
            switch (eventType) {
              case 'INSERT':
                if (newRecord && !current.find(t => t.id === newRecord.id)) {
                  return [...current, { ...newRecord, completed: Boolean(newRecord.completed) }]
                }
                return current
              case 'UPDATE':
                if (newRecord) {
                  return current.map(t =>
                    t.id === newRecord.id
                      ? { ...newRecord, completed: Boolean(newRecord.completed) }
                      : t
                  )
                }
                return current
              case 'DELETE':
                if (oldRecord) {
                  return current.filter(t => t.id !== oldRecord.id)
                }
                return current
              default:
                return current
            }
          })
        }
      )
      .subscribe()

    channelRef.current = channel

    return () => {
      channel.unsubscribe()
    }
  }, [fetchTodos, onEvent])

  // Add a new todo
  const addTodo = useCallback(async (title: string, author: string) => {
    const newTodo: Todo = {
      id: generateId(),
      title,
      completed: false,
      author,
      created_at: new Date().toISOString()
    }

    // Optimistic update
    setTodos(current => [...current, newTodo])

    const { error: insertError } = await supabase
      .from('todos')
      .insert([{
        id: newTodo.id,
        title: newTodo.title,
        completed: 0,
        author: newTodo.author,
        created_at: newTodo.created_at
      }])

    if (insertError) {
      // Rollback on error
      setTodos(current => current.filter(t => t.id !== newTodo.id))
      setError(insertError.message)
      return null
    }

    return newTodo
  }, [])

  // Update a todo
  const updateTodo = useCallback(async (id: string, updates: Partial<Pick<Todo, 'title' | 'completed'>>) => {
    const todoIndex = todos.findIndex(t => t.id === id)
    if (todoIndex === -1) return false

    const oldTodo = todos[todoIndex]
    const updatedTodo = { ...oldTodo, ...updates }

    // Optimistic update
    setTodos(current => current.map(t => t.id === id ? updatedTodo : t))

    const dbUpdates: Record<string, unknown> = {}
    if (updates.title !== undefined) dbUpdates.title = updates.title
    if (updates.completed !== undefined) dbUpdates.completed = updates.completed ? 1 : 0

    const { error: updateError } = await supabase
      .from('todos')
      .update(dbUpdates)
      .eq('id', id)

    if (updateError) {
      // Rollback on error
      setTodos(current => current.map(t => t.id === id ? oldTodo : t))
      setError(updateError.message)
      return false
    }

    return true
  }, [todos])

  // Delete a todo
  const deleteTodo = useCallback(async (id: string) => {
    const todoToDelete = todos.find(t => t.id === id)
    if (!todoToDelete) return false

    // Optimistic update
    setTodos(current => current.filter(t => t.id !== id))

    const { error: deleteError } = await supabase
      .from('todos')
      .delete()
      .eq('id', id)

    if (deleteError) {
      // Rollback on error
      setTodos(current => [...current, todoToDelete].sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      ))
      setError(deleteError.message)
      return false
    }

    return true
  }, [todos])

  return {
    todos,
    loading,
    error,
    addTodo,
    updateTodo,
    deleteTodo,
    refetch: fetchTodos
  }
}
