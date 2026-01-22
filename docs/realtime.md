# sblite Realtime

sblite provides a Supabase-compatible Realtime API for WebSocket-based communication. The API implements the Phoenix Protocol v1.0.0 and is fully compatible with `@supabase/supabase-js`.

## Overview

- **100% Supabase API compatible** - Works with the official Supabase JavaScript client
- **Three core features** - Broadcast, Presence, and Postgres Changes
- **Phoenix Protocol** - JSON-based WebSocket messaging
- **Channel-based** - Organize communication by topics/channels
- **Low latency** - Sub-second message delivery

## Quick Start

### Enable Realtime

Start sblite with the `--realtime` flag:

```bash
./sblite serve --realtime
```

### Connect with supabase-js

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  'http://localhost:8080',
  'your-anon-key'
)

// Subscribe to a channel
const channel = supabase.channel('room-1')

channel
  .on('broadcast', { event: 'cursor-move' }, (payload) => {
    console.log('Cursor moved:', payload)
  })
  .subscribe()

// Send a broadcast
channel.send({
  type: 'broadcast',
  event: 'cursor-move',
  payload: { x: 100, y: 200 }
})
```

## Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--realtime` | - | `false` | Enable realtime WebSocket support |

The WebSocket endpoint is available at:

```
ws://localhost:8080/realtime/v1/websocket?apikey=<your-api-key>
```

## Broadcast

Broadcast enables low-latency messaging between connected clients. Messages are ephemeral and not persisted to the database.

### Use Cases

- Cursor positions in collaborative apps
- Typing indicators
- Game state synchronization
- Live reactions

### Code Example

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Subscribe to broadcasts
const channel = supabase.channel('room-1', {
  config: {
    broadcast: { self: false }  // Don't receive your own broadcasts
  }
})

channel
  .on('broadcast', { event: 'message' }, (payload) => {
    console.log('Received:', payload.payload)
  })
  .subscribe()

// Send a broadcast
channel.send({
  type: 'broadcast',
  event: 'message',
  payload: { text: 'Hello, world!' }
})
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `self` | boolean | `false` | Receive your own broadcast messages |
| `ack` | boolean | `false` | Receive acknowledgment when message is sent |

### Example: Cursor Tracking

```typescript
const channel = supabase.channel('whiteboard', {
  config: {
    broadcast: { self: false }
  }
})

channel
  .on('broadcast', { event: 'cursor' }, ({ payload }) => {
    updateRemoteCursor(payload.userId, payload.x, payload.y)
  })
  .subscribe()

// Track local cursor
document.addEventListener('mousemove', (e) => {
  channel.send({
    type: 'broadcast',
    event: 'cursor',
    payload: { userId: currentUser.id, x: e.clientX, y: e.clientY }
  })
})
```

## Presence

Presence tracks and synchronizes shared state across connected clients. It's ideal for showing who's online and their current status.

### Use Cases

- Online/offline indicators
- Who's viewing a document
- Multiplayer game lobbies
- Active users in a chat room

### Code Example

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

const channel = supabase.channel('room-1', {
  config: {
    presence: { key: currentUser.id }
  }
})

// Track presence state changes
channel
  .on('presence', { event: 'sync' }, () => {
    const state = channel.presenceState()
    console.log('Online users:', Object.keys(state))
  })
  .on('presence', { event: 'join' }, ({ key, newPresences }) => {
    console.log('User joined:', key, newPresences)
  })
  .on('presence', { event: 'leave' }, ({ key, leftPresences }) => {
    console.log('User left:', key, leftPresences)
  })
  .subscribe(async (status) => {
    if (status === 'SUBSCRIBED') {
      // Track your presence
      await channel.track({
        online_at: new Date().toISOString(),
        status: 'online',
        name: currentUser.name
      })
    }
  })
```

### Presence Events

| Event | Description |
|-------|-------------|
| `sync` | Full presence state synchronized |
| `join` | User(s) joined the channel |
| `leave` | User(s) left the channel |

### Example: Typing Indicator

```typescript
const channel = supabase.channel('chat-room', {
  config: {
    presence: { key: currentUser.id }
  }
})

channel
  .on('presence', { event: 'sync' }, () => {
    const state = channel.presenceState()
    const typing = Object.values(state)
      .flat()
      .filter(p => p.is_typing)
      .map(p => p.name)

    if (typing.length > 0) {
      showTypingIndicator(typing.join(', ') + ' is typing...')
    } else {
      hideTypingIndicator()
    }
  })
  .subscribe()

// Update typing status
let typingTimeout
inputElement.addEventListener('input', () => {
  channel.track({ is_typing: true, name: currentUser.name })

  clearTimeout(typingTimeout)
  typingTimeout = setTimeout(() => {
    channel.track({ is_typing: false, name: currentUser.name })
  }, 2000)
})
```

## Postgres Changes

Postgres Changes delivers real-time notifications when rows are inserted, updated, or deleted from your tables.

### Use Cases

- Live dashboards
- Real-time feeds
- Collaborative editing
- Cache invalidation

### Code Example

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

const channel = supabase.channel('db-changes')

channel
  .on(
    'postgres_changes',
    { event: '*', schema: 'public', table: 'messages' },
    (payload) => {
      console.log('Change:', payload.eventType, payload.new)
    }
  )
  .subscribe()
```

### Event Types

| Event | Description |
|-------|-------------|
| `INSERT` | New row inserted |
| `UPDATE` | Existing row updated |
| `DELETE` | Row deleted |
| `*` | All events (wildcard) |

### Filtering

Filter events by column values using PostgREST-style operators:

```typescript
// Only receive events where status = 'published'
channel
  .on(
    'postgres_changes',
    {
      event: 'INSERT',
      schema: 'public',
      table: 'posts',
      filter: 'status=eq.published'
    },
    (payload) => {
      console.log('New published post:', payload.new)
    }
  )
  .subscribe()
```

### Supported Filter Operators

| Operator | Example | Description |
|----------|---------|-------------|
| `eq` | `status=eq.active` | Equal to |
| `neq` | `status=neq.deleted` | Not equal to |
| `gt` | `age=gt.21` | Greater than |
| `gte` | `age=gte.21` | Greater than or equal |
| `lt` | `price=lt.100` | Less than |
| `lte` | `price=lte.100` | Less than or equal |
| `in` | `id=in.(1,2,3)` | In list |

### Example: Live Chat

```typescript
const channel = supabase.channel('chat-updates')

channel
  .on(
    'postgres_changes',
    { event: 'INSERT', schema: 'public', table: 'messages' },
    (payload) => {
      appendMessage(payload.new)
    }
  )
  .on(
    'postgres_changes',
    { event: 'UPDATE', schema: 'public', table: 'messages' },
    (payload) => {
      updateMessage(payload.new.id, payload.new)
    }
  )
  .on(
    'postgres_changes',
    { event: 'DELETE', schema: 'public', table: 'messages' },
    (payload) => {
      removeMessage(payload.old.id)
    }
  )
  .subscribe()
```

### Example: User-Specific Changes

```typescript
// Only receive changes for the current user's data
channel
  .on(
    'postgres_changes',
    {
      event: '*',
      schema: 'public',
      table: 'notifications',
      filter: `user_id=eq.${currentUser.id}`
    },
    (payload) => {
      showNotification(payload.new)
    }
  )
  .subscribe()
```

## Multiple Subscriptions

A single channel can have multiple subscriptions:

```typescript
const channel = supabase.channel('app-updates')

channel
  // Broadcast: cursor movements
  .on('broadcast', { event: 'cursor' }, handleCursor)

  // Presence: user status
  .on('presence', { event: 'sync' }, handlePresenceSync)

  // Postgres Changes: messages table
  .on(
    'postgres_changes',
    { event: 'INSERT', schema: 'public', table: 'messages' },
    handleNewMessage
  )

  // Postgres Changes: notifications table
  .on(
    'postgres_changes',
    { event: '*', schema: 'public', table: 'notifications' },
    handleNotification
  )
  .subscribe()
```

## Dashboard Monitoring

Monitor realtime connections via the dashboard API:

```bash
curl http://localhost:8080/_/api/realtime/stats \
  -H "Cookie: session=your-session-token"
```

Response:

```json
{
  "connections": 5,
  "channels": 3,
  "channel_details": [
    {
      "topic": "realtime:room-1",
      "subscribers": 3,
      "has_presence": true
    },
    {
      "topic": "realtime:db-changes",
      "subscribers": 2,
      "has_presence": false
    }
  ]
}
```

## WebSocket Protocol Details

sblite implements the Phoenix Protocol v1.0.0 for WebSocket communication.

### Message Format

All messages are JSON with this structure:

```json
{
  "event": "phx_join",
  "topic": "realtime:room-1",
  "payload": { ... },
  "ref": "1",
  "join_ref": "1"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Event type (see below) |
| `topic` | string | Channel topic (e.g., `realtime:room-1`) |
| `payload` | object | Event-specific data |
| `ref` | string | Message reference for correlation |
| `join_ref` | string | Join reference for channel association |

### Client Events

| Event | Description |
|-------|-------------|
| `phx_join` | Join a channel |
| `phx_leave` | Leave a channel |
| `heartbeat` | Keep connection alive (sent every 25s) |
| `broadcast` | Send a broadcast message |
| `presence` | Presence track/untrack |
| `access_token` | Refresh JWT token |

### Server Events

| Event | Description |
|-------|-------------|
| `phx_reply` | Response to client event |
| `phx_close` | Channel closed |
| `phx_error` | Error occurred |
| `system` | System notification |
| `broadcast` | Broadcast message received |
| `postgres_changes` | Database change notification |
| `presence_state` | Full presence snapshot |
| `presence_diff` | Presence changes (joins/leaves) |

### Connection Lifecycle

1. Client connects to WebSocket with API key
2. Client sends `phx_join` to join channels
3. Server responds with `phx_reply`
4. Client sends `heartbeat` every 25 seconds
5. Server sends data events as they occur
6. Client sends `phx_leave` to leave channels

### Example Raw WebSocket Usage

```javascript
const ws = new WebSocket('ws://localhost:8080/realtime/v1/websocket?apikey=your-anon-key')

ws.onopen = () => {
  // Join a channel
  ws.send(JSON.stringify({
    event: 'phx_join',
    topic: 'realtime:room-1',
    payload: {
      config: {
        broadcast: { self: false }
      }
    },
    ref: '1',
    join_ref: '1'
  }))
}

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data)
  console.log('Received:', msg)
}

// Send heartbeat every 25 seconds
setInterval(() => {
  ws.send(JSON.stringify({
    event: 'heartbeat',
    topic: 'phoenix',
    payload: {},
    ref: String(Date.now())
  }))
}, 25000)
```

## Limitations

Current implementation limitations compared to Supabase:

| Feature | Status | Notes |
|---------|--------|-------|
| Broadcast | Supported | Full support |
| Presence | Supported | Full support |
| Postgres Changes | Supported | Triggered by REST API changes |
| Private channels | Supported | Requires JWT authentication |
| RLS integration | Partial | Supported for private channels |
| Multiplexing | Supported | Multiple channels per connection |
| Binary messages | Not supported | JSON only |
| Rate limiting | Not implemented | No connection/message limits |
| Clustering | Not supported | Single-node only |

### Postgres Changes Limitations

- Changes are detected via REST API hooks, not database triggers
- Changes made directly to SQLite (bypassing the REST API) will not trigger events
- No support for listening to changes in system tables

## Migration to Supabase

sblite Realtime is designed for seamless migration to Supabase:

### 1. Code Compatibility

Your application code using `@supabase/supabase-js` will work without changes:

```typescript
// This code works with both sblite and Supabase
const channel = supabase.channel('my-channel')
  .on('broadcast', { event: 'message' }, handler)
  .subscribe()
```

### 2. Migration Steps

1. **Export your data** using sblite's export commands
2. **Import to Supabase** using their import tools
3. **Update connection URL** to point to Supabase
4. **Enable Realtime** in Supabase dashboard for your tables

### 3. Supabase-Specific Features

After migration, you'll gain access to:

- Database-level change detection (not REST-only)
- Connection rate limiting
- Horizontal scaling
- Binary message support
- Advanced RLS integration

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| Invalid API key | Wrong or missing API key | Check `apikey` query parameter |
| Connection timeout | No heartbeat received | Ensure heartbeat is sent every 25s |
| Invalid token | Malformed or expired JWT | Refresh access token |
| Unauthorized | Private channel without auth | Provide valid JWT in join payload |

### Example Error Handling

```typescript
const channel = supabase.channel('room-1')

channel.subscribe((status, err) => {
  if (status === 'SUBSCRIBED') {
    console.log('Connected!')
  } else if (status === 'CHANNEL_ERROR') {
    console.error('Channel error:', err)
  } else if (status === 'TIMED_OUT') {
    console.error('Connection timed out')
  } else if (status === 'CLOSED') {
    console.log('Connection closed')
  }
})
```

## Best Practices

### 1. Use Meaningful Channel Names

```typescript
// Good: descriptive and scoped
supabase.channel('project:123:comments')
supabase.channel('user:456:notifications')

// Avoid: generic names
supabase.channel('channel1')
```

### 2. Filter Postgres Changes

```typescript
// Good: filter to reduce traffic
channel.on(
  'postgres_changes',
  { event: 'INSERT', schema: 'public', table: 'posts', filter: 'published=eq.true' },
  handler
)

// Avoid: subscribing to all changes without filters
channel.on(
  'postgres_changes',
  { event: '*', schema: 'public', table: 'posts' },
  handler
)
```

### 3. Clean Up Subscriptions

```typescript
// Always unsubscribe when done
const channel = supabase.channel('room-1')
channel.subscribe()

// Later...
supabase.removeChannel(channel)
```

### 4. Handle Reconnection

```typescript
const channel = supabase.channel('room-1')

channel.subscribe((status) => {
  if (status === 'SUBSCRIBED') {
    // Re-sync state after reconnection
    fetchLatestData()
  }
})
```
