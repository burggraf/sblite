# sblite Realtime Demo

A React demo app showcasing all three sblite realtime features: **Postgres Changes**, **Broadcast**, and **Presence**.

## Features

### Todos Tab (Postgres Changes + Presence)
- Full CRUD operations on a `todos` table
- Real-time sync via `postgres_changes` subscriptions
- INSERT, UPDATE, DELETE events propagate instantly to all clients
- Presence sidebar shows who's online

### Broadcast Tab (Cursor Tracking)
- Real-time cursor position sharing between clients
- Click to send emoji reactions
- Ephemeral messaging (not persisted to database)

### Status Tab (Event Monitoring)
- Live event log of all realtime events
- Connection status indicator
- Event filtering by type
- Debug and learning tool

## Stack

- **React 19** + **TypeScript**
- **Vite 7** - Build tool
- **Tailwind CSS v4** - Styling
- **shadcn/ui** - UI components
- **@supabase/supabase-js** - Supabase client (connects to sblite)

## Prerequisites

1. **sblite** must be built:
   ```bash
   # From the sblite root directory
   go build -o sblite .
   ```

2. **Node.js 18+** for running the Vite dev server

## Setup

### 1. Initialize sblite database

```bash
./sblite init
```

### 2. Create the todos table

Using the sblite dashboard at `http://localhost:8080/_` or via API:

```bash
curl -X POST http://localhost:8080/admin/v1/tables \
  -H "Authorization: Bearer <service_role_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "todos",
    "columns": [
      {"name": "id", "type": "uuid", "primary_key": true},
      {"name": "title", "type": "text", "nullable": false},
      {"name": "completed", "type": "integer", "default": "0"},
      {"name": "author", "type": "text", "nullable": false},
      {"name": "created_at", "type": "timestamptz", "nullable": false}
    ]
  }'
```

Or create via SQL in the dashboard SQL Browser:

```sql
CREATE TABLE todos (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  completed INTEGER DEFAULT 0,
  author TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

### 3. Install dependencies

```bash
cd test_apps/chat
npm install
```

### 4. Configure environment

Edit `.env.local`:

```bash
VITE_SUPABASE_URL=http://localhost:8080
VITE_SUPABASE_PUBLISHABLE_OR_ANON_KEY=<your_anon_key>
```

Get your anon key from the sblite dashboard at Settings > API Keys.

## Running

### 1. Start sblite with realtime enabled

```bash
./sblite serve --realtime
```

### 2. Start the demo app

```bash
cd test_apps/chat
npm run dev
```

### 3. Open multiple browser tabs

Navigate to `http://localhost:5173` in multiple tabs/windows to test realtime sync.

## How It Works

### Postgres Changes

```typescript
const channel = supabase.channel('todos-changes')
  .on(
    'postgres_changes',
    { event: '*', schema: 'public', table: 'todos' },
    (payload) => {
      // Handle INSERT, UPDATE, DELETE
      console.log('Change:', payload.eventType, payload.new)
    }
  )
  .subscribe()
```

### Presence

```typescript
const channel = supabase.channel('presence-room', {
  config: { presence: { key: uniqueUserId } }
})

channel
  .on('presence', { event: 'sync' }, () => {
    const state = channel.presenceState()
    console.log('Online users:', Object.keys(state))
  })
  .subscribe(async (status) => {
    if (status === 'SUBSCRIBED') {
      await channel.track({ username: 'Alice', online_at: new Date() })
    }
  })
```

### Broadcast

```typescript
const channel = supabase.channel('cursor-room', {
  config: { broadcast: { self: false } }
})

channel
  .on('broadcast', { event: 'cursor' }, ({ payload }) => {
    console.log('Cursor moved:', payload.x, payload.y)
  })
  .subscribe()

// Send cursor position
channel.send({
  type: 'broadcast',
  event: 'cursor',
  payload: { x: 100, y: 200 }
})
```

## Project Structure

```
chat/
├── src/
│   ├── components/
│   │   ├── ui/                    # shadcn components
│   │   ├── tabs/
│   │   │   ├── todos-tab.tsx      # Todo list + presence
│   │   │   ├── broadcast-tab.tsx  # Cursor tracking
│   │   │   └── status-tab.tsx     # Event log
│   │   ├── app-header.tsx         # Username & connection status
│   │   ├── todo-item.tsx          # Single todo with edit/delete
│   │   ├── todo-form.tsx          # Add new todo
│   │   ├── presence-sidebar.tsx   # Online users list
│   │   ├── cursor-canvas.tsx      # Cursor tracking area
│   │   └── event-log.tsx          # Realtime event viewer
│   ├── hooks/
│   │   ├── use-todos.ts           # CRUD + postgres_changes
│   │   ├── use-presence.ts        # Presence tracking
│   │   ├── use-broadcast.ts       # Cursor/reaction broadcast
│   │   └── use-event-log.ts       # Event aggregation
│   ├── lib/
│   │   ├── supabase/
│   │   │   └── client.ts          # Supabase client
│   │   └── utils.ts               # Utilities
│   ├── App.tsx                    # Main app with tabs
│   └── main.tsx                   # Entry point
├── .env.local                     # Environment config
└── package.json
```

## Troubleshooting

### "Table 'todos' does not exist"

Create the todos table using the setup instructions above.

### Connection fails

- Ensure sblite is running with `--realtime` flag
- Check that `VITE_SUPABASE_URL` points to the correct sblite URL
- Verify the anon key is correct

### Changes not syncing

- Open the Status tab to see if events are being received
- Check the browser console for WebSocket errors
- Ensure all clients are connected to the same sblite instance
