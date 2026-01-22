# Realtime Demo App Design

## Overview

A React demo app showcasing all three sblite realtime features: Postgres Changes, Broadcast, and Presence. Located in `test_apps/chat/`.

## Architecture

### Tabbed Interface

1. **Todos Tab** - Database changes + presence
   - Todo list with full CRUD operations via REST API
   - Postgres Changes subscription receives INSERT/UPDATE/DELETE events in realtime
   - Presence sidebar shows online users
   - Any client can insert, edit, or delete todos

2. **Broadcast Tab** - Ephemeral cursor tracking
   - Canvas showing mouse cursor positions of all connected users
   - Each user gets a random color with labeled cursor
   - Demonstrates ephemeral, non-persisted real-time messaging

3. **Status Tab** - Connection monitoring
   - Connection status indicator
   - Live event log of all incoming realtime events
   - Channel subscription list
   - Debugging/learning tool

### Tech Stack

- React 19 + TypeScript + Vite 7
- Tailwind CSS v4 + shadcn/ui
- @supabase/supabase-js client

## Data Model

### Todos Table

```sql
CREATE TABLE todos (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  completed INTEGER DEFAULT 0,
  author TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

**Operations:**
- INSERT - Add new todo
- UPDATE - Toggle completed, edit title
- DELETE - Remove todo

## File Structure

```
chat/
├── src/
│   ├── components/
│   │   ├── ui/                    # shadcn components
│   │   ├── app-header.tsx         # Username input, connection status
│   │   ├── tabs/
│   │   │   ├── todos-tab.tsx      # Todo list + presence sidebar
│   │   │   ├── broadcast-tab.tsx  # Cursor tracking canvas
│   │   │   └── status-tab.tsx     # Event log & debug info
│   │   ├── todo-item.tsx          # Single todo with edit/delete
│   │   ├── todo-form.tsx          # Add new todo input
│   │   ├── presence-sidebar.tsx   # Online users list
│   │   ├── cursor-canvas.tsx      # Remote cursors display
│   │   └── event-log.tsx          # Scrolling event viewer
│   ├── hooks/
│   │   ├── use-todos.ts           # CRUD + postgres_changes subscription
│   │   ├── use-presence.ts        # Presence tracking
│   │   ├── use-broadcast.ts       # Cursor/ephemeral messaging
│   │   └── use-event-log.ts       # Captures all realtime events
│   ├── lib/
│   │   └── supabase/
│   │       └── client.ts          # Supabase client
│   ├── App.tsx                    # Main layout with tabs
│   └── main.tsx                   # Entry point
```

## Hooks API

### useTodos()

```typescript
const { todos, addTodo, updateTodo, deleteTodo, loading } = useTodos()
```

- Fetches initial todos on mount
- Subscribes to `postgres_changes` for realtime updates
- Provides CRUD methods that call REST API

### usePresence(username: string)

```typescript
const { onlineUsers, isConnected } = usePresence(username)
```

- Tracks current user's presence
- Returns list of online users with their metadata

### useBroadcast(username: string)

```typescript
const { cursors, sendCursor } = useBroadcast(username)
```

- Sends local cursor position on mouse move
- Receives and tracks remote cursor positions

### useEventLog()

```typescript
const { events, clearEvents } = useEventLog()
```

- Aggregates all realtime events for display
- Provides clear function for reset

## UI Components

### App Header
- Username input (persisted to localStorage)
- Connection status badge (green/red)
- Tab navigation

### Todos Tab
- Left: Todo list with inline editing
- Right: Presence sidebar with avatars
- Add form at top
- Visual highlight on new/updated items

### Broadcast Tab
- Full canvas area
- Colored cursors with username labels
- Own cursor shown differently

### Status Tab
- Connection state display
- Scrolling event log with timestamps
- Filter by event type
