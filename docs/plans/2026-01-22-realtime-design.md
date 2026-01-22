# sblite Realtime Design

**Date:** 2026-01-22
**Status:** Approved
**Author:** Claude + markb

## Overview

This document describes the design for sblite Realtime, a Supabase-compatible WebSocket implementation providing real-time features for sblite applications. The goal is full compatibility with `@supabase/supabase-js` so applications can migrate to Supabase without code changes.

## Features

sblite Realtime implements all three Supabase Realtime features:

1. **Broadcast** - Low-latency ephemeral messages between clients
2. **Presence** - Track and synchronize user online state
3. **Postgres Changes** - Database change notifications (CDC)

## Architecture

### Package Structure

```
internal/realtime/
├── realtime.go      # Service orchestration, public API
├── hub.go           # Connection hub, channel management
├── conn.go          # WebSocket connection wrapper
├── channel.go       # Channel state, subscriptions
├── broadcast.go     # Broadcast message handling
├── presence.go      # Presence state (CRDT-like)
├── postgres.go      # Postgres Changes subscription logic
├── filter.go        # Filter evaluation for postgres_changes
├── protocol.go      # Phoenix protocol message types
└── handler.go       # HTTP handler for WebSocket upgrade
```

### Data Flow

1. Client connects to `/realtime/v1/websocket?apikey=<key>`
2. Server upgrades to WebSocket, creates `Conn` wrapper
3. Client sends `phx_join` to join channels with config
4. Server validates JWT/RLS, creates `Channel` with subscriptions
5. For Postgres Changes: REST handlers call `hub.Broadcast()` on mutations
6. Hub evaluates filters, routes events to matching subscriptions
7. Heartbeat every 25s keeps connection alive

### Integration Points

- `Server` struct gains `realtimeService *realtime.Service`
- REST handlers call `realtimeService.NotifyChange(table, event, old, new)`
- Dashboard gets `/_/api/realtime/stats` endpoint for monitoring

## Protocol Specification

### Phoenix Protocol v1.0.0

All messages are JSON text frames with this structure:

```go
type Message struct {
    Event   string         `json:"event"`
    Topic   string         `json:"topic"`
    Payload map[string]any `json:"payload"`
    Ref     string         `json:"ref"`
    JoinRef string         `json:"join_ref,omitempty"`
}
```

### Client → Server Events

| Event | Topic | Payload | Description |
|-------|-------|---------|-------------|
| `phx_join` | `realtime:{channel}` | `{config, access_token}` | Join channel |
| `phx_leave` | `realtime:{channel}` | `{}` | Leave channel |
| `heartbeat` | `phoenix` | `{}` | Keep-alive (25s interval) |
| `access_token` | `realtime:{channel}` | `{access_token}` | Refresh JWT |
| `broadcast` | `realtime:{channel}` | `{type, event, payload}` | Send broadcast |
| `presence` | `realtime:{channel}` | `{type, event, payload}` | Presence update |

### Server → Client Events

| Event | Description |
|-------|-------------|
| `phx_reply` | Acknowledgment: `{status: "ok"/"error", response}` |
| `phx_close` | Channel closed |
| `phx_error` | Error occurred |
| `system` | Subscription status notification |
| `broadcast` | Broadcast message received |
| `postgres_changes` | Database change event |
| `presence_state` | Full presence snapshot |
| `presence_diff` | Presence joins/leaves |

### Join Configuration

```go
type JoinConfig struct {
    Broadcast       BroadcastConfig       `json:"broadcast"`
    Presence        PresenceConfig        `json:"presence"`
    PostgresChanges []PostgresChangesSub  `json:"postgres_changes"`
    Private         bool                  `json:"private"`
}

type BroadcastConfig struct {
    Ack  bool `json:"ack"`   // wait for server ack
    Self bool `json:"self"`  // receive own broadcasts
}

type PresenceConfig struct {
    Key string `json:"key"`  // presence key (e.g., user ID)
}

type PostgresChangesSub struct {
    Event  string `json:"event"`  // INSERT, UPDATE, DELETE, *
    Schema string `json:"schema"` // "public"
    Table  string `json:"table"`  // table name or "*"
    Filter string `json:"filter"` // e.g., "user_id=eq.123"
}
```

## Core Components

### Hub (Central Coordinator)

```go
type Hub struct {
    mu          sync.RWMutex
    connections map[string]*Conn          // connID -> Conn
    channels    map[string]*Channel       // topic -> Channel
    db          *sql.DB
    rlsService  *rls.Service
    jwtSecret   string
}
```

Responsibilities:
- Track all active WebSocket connections
- Manage channel lifecycle (create on first join, cleanup when empty)
- Route broadcast messages to appropriate channels
- Receive database change notifications from REST handlers
- Provide stats for dashboard monitoring

### Connection (Per-Client State)

```go
type Conn struct {
    id        string
    ws        *websocket.Conn
    hub       *Hub
    channels  map[string]*ChannelSub  // topic -> subscription
    claims    *jwt.MapClaims          // parsed from access_token
    send      chan []byte             // outbound message queue
    done      chan struct{}
    lastPing  time.Time
}
```

Each connection:
- Has unique ID (UUID)
- Maintains its own channel subscriptions
- Has a buffered send channel (prevents slow clients blocking)
- Tracks JWT claims for authorization
- Monitors heartbeat for timeout (30s without heartbeat = disconnect)

### Concurrency Model

- Hub uses RWMutex for connection/channel maps
- Each Conn has dedicated read and write goroutines
- Channel operations are serialized through Hub
- No locks held during WebSocket I/O

### Channel (Shared State per Topic)

```go
type Channel struct {
    topic       string
    private     bool
    mu          sync.RWMutex
    subscribers map[string]*ChannelSub  // connID -> subscription
    presence    *PresenceState          // nil if presence disabled
}

type ChannelSub struct {
    conn            *Conn
    joinRef         string
    broadcastConfig BroadcastConfig
    presenceConfig  PresenceConfig
    pgChanges       []PostgresChangesSub
}
```

### Channel Join Flow

1. Client sends `phx_join` with config and access_token
2. Server validates JWT signature and expiry
3. If `private: true`, check RLS authorization:
   - Query `_rls_policies` for `realtime.messages` table
   - Evaluate SELECT policy with user's claims
   - Reject if policy denies access
4. Create/get Channel, add ChannelSub
5. If presence enabled, broadcast `presence_state` to new subscriber
6. Send `phx_reply` with `{status: "ok"}`
7. Send `system` message confirming postgres_changes subscriptions

## Postgres Changes

### Change Event Structure

```go
type ChangeEvent struct {
    Schema          string         `json:"schema"`
    Table           string         `json:"table"`
    CommitTimestamp string         `json:"commit_timestamp"`
    EventType       string         `json:"eventType"`  // INSERT, UPDATE, DELETE
    New             map[string]any `json:"new"`
    Old             map[string]any `json:"old"`
    Errors          []string       `json:"errors"`
}
```

### REST Handler Integration

Modify `HandleInsert`, `HandleUpdate`, `HandleDelete` to call the realtime service after successful mutations:

```go
// In HandleInsert, after successful insert:
if s.realtimeService != nil {
    s.realtimeService.NotifyChange("public", table, "INSERT", nil, insertedRow)
}

// In HandleUpdate, after successful update:
if s.realtimeService != nil {
    s.realtimeService.NotifyChange("public", table, "UPDATE", oldRow, newRow)
}

// In HandleDelete, after successful delete:
if s.realtimeService != nil {
    s.realtimeService.NotifyChange("public", table, "DELETE", deletedRow, nil)
}
```

### Filter Evaluation

When a change occurs, the Hub evaluates each subscriber's filters:

```go
func (h *Hub) matchesFilter(sub PostgresChangesSub, event ChangeEvent) bool {
    // Check event type (* matches all)
    if sub.Event != "*" && sub.Event != event.EventType {
        return false
    }
    // Check schema
    if sub.Schema != "*" && sub.Schema != event.Schema {
        return false
    }
    // Check table
    if sub.Table != "*" && sub.Table != event.Table {
        return false
    }
    // Evaluate row filter (e.g., "user_id=eq.123")
    if sub.Filter != "" {
        return evaluateFilter(sub.Filter, event.New, event.Old)
    }
    return true
}
```

**Supported Filter Operators:** `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `in`

Reuses existing `rest.ParseFilter` logic.

### RLS for Change Events

Before sending a change event to a subscriber, verify they have SELECT permission on the affected row using existing RLS infrastructure. This prevents leaking data the user shouldn't see.

## Broadcast

### Message Flow

```
Client A                Hub                    Client B, C
   |                     |                         |
   |-- broadcast msg --> |                         |
   |                     |-- evaluate self flag -->|
   |                     |-- send to B, C -------->|
   |<-- phx_reply (ack)--|                         |
```

### Payload Structure

```json
// Client sends:
{
    "event": "broadcast",
    "topic": "realtime:room:123",
    "payload": {
        "type": "broadcast",
        "event": "cursor-move",
        "payload": {"x": 100, "y": 200}
    },
    "ref": "5"
}

// Server forwards to other subscribers:
{
    "event": "broadcast",
    "topic": "realtime:room:123",
    "payload": {
        "type": "broadcast",
        "event": "cursor-move",
        "payload": {"x": 100, "y": 200}
    },
    "ref": null
}
```

### Acknowledgment

When `broadcast.ack` is configured, the sender receives a `phx_reply` after the message is queued to all recipients. This confirms the server received and queued the message, not that recipients received it (matches Supabase behavior - no delivery guarantee).

## Presence

### State Structure

```go
type PresenceState struct {
    mu    sync.RWMutex
    state map[string][]PresenceMeta  // presenceKey -> list of metas
}

type PresenceMeta struct {
    ConnID  string         `json:"-"`
    PhxRef  string         `json:"phx_ref"`
    Payload map[string]any // user metadata spread into object
}
```

### Operations

**Track** - Client announces their presence:
```json
{"event": "presence", "payload": {"type": "presence", "event": "track", "payload": {"user_id": "123", "status": "online"}}}
```

**Untrack** - Automatic on channel leave or connection close.

### Events to Clients

```json
// presence_state - Full snapshot on join
{
    "event": "presence_state",
    "topic": "realtime:room:123",
    "payload": {
        "user-123": [{"phx_ref": "abc", "user_id": "123", "status": "online"}],
        "user-456": [{"phx_ref": "def", "user_id": "456", "status": "away"}]
    }
}

// presence_diff - Incremental updates
{
    "event": "presence_diff",
    "topic": "realtime:room:123",
    "payload": {
        "joins": {"user-789": [{"phx_ref": "ghi", "status": "online"}]},
        "leaves": {"user-123": [{"phx_ref": "abc"}]}
    }
}
```

### Presence Key

The `presence.key` in join config determines how presences are grouped. Typically the user ID, allowing multiple connections per user (tabs/devices) to be tracked together.

## HTTP Handler

### WebSocket Endpoint

```go
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 1. Validate API key from query param
    apiKey := r.URL.Query().Get("apikey")
    if !s.validateAPIKey(apiKey) {
        http.Error(w, "Invalid API key", http.StatusUnauthorized)
        return
    }

    // 2. Upgrade to WebSocket
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    // 3. Create connection and register with hub
    conn := s.hub.NewConn(ws)
    go conn.ReadPump()
    go conn.WritePump()
}
```

### Route Registration

```go
// In server.go setupRoutes():
if s.realtimeService != nil {
    s.router.Get("/realtime/v1/websocket", s.realtimeService.HandleWebSocket)
}
```

## Dashboard Integration

### Stats Endpoint

```go
// GET /_/api/realtime/stats
type RealtimeStats struct {
    Connections    int            `json:"connections"`
    Channels       int            `json:"channels"`
    ChannelDetails []ChannelStats `json:"channel_details"`
}

type ChannelStats struct {
    Topic       string `json:"topic"`
    Subscribers int    `json:"subscribers"`
    HasPresence bool   `json:"has_presence"`
}
```

### Dashboard UI

Add a "Realtime" section to the dashboard sidebar showing:
- Total active connections count
- Total channels count
- Table listing channels with subscriber counts
- Auto-refresh every 5 seconds

## E2E Tests

### Test Structure

```
e2e/tests/realtime/
├── connection.test.ts      # Connect, disconnect, heartbeat, timeout
├── broadcast.test.ts       # Send/receive broadcasts, self flag, ack
├── presence.test.ts        # Track, untrack, sync, join/leave events
├── postgres-changes.test.ts # INSERT/UPDATE/DELETE events, filters
├── channels.test.ts        # Join, leave, private channels, RLS
└── protocol.test.ts        # Message format, error handling, refs
```

### Test Cases (~40-50 tests)

**Connection (6 tests):**
- Connect with valid API key
- Reject invalid API key
- Heartbeat keeps connection alive
- Timeout after missed heartbeats
- Multiple concurrent connections
- Graceful disconnect

**Broadcast (8 tests):**
- Send and receive broadcast
- Self flag false (don't receive own)
- Self flag true (receive own)
- Ack flag returns confirmation
- Multiple subscribers receive message
- Broadcast to specific channel only
- Custom event names
- Large payload handling

**Presence (10 tests):**
- Track presence on join
- Receive presence_state on join
- Receive presence_diff on peer join
- Receive presence_diff on peer leave
- Multiple presences same key (tabs)
- Automatic untrack on disconnect
- Custom presence metadata
- Presence sync event
- Presence key grouping
- Empty presence state

**Postgres Changes (12 tests):**
- Receive INSERT events
- Receive UPDATE events with old/new
- Receive DELETE events
- Filter by event type
- Filter by table name
- Filter by column value (eq, neq, gt, etc.)
- Wildcard table subscription
- Multiple subscriptions same channel
- RLS filters change events
- No event for unauthorized rows
- Schema filtering
- Unsubscribe stops events

**Channels (8 tests):**
- Join public channel
- Join private channel with JWT
- Reject private channel without JWT
- Leave channel
- RLS policy enforcement
- Multiple channels per connection
- Channel cleanup when empty
- Access token refresh

## Implementation Phases

### Phase 1: Core Infrastructure
- Create `internal/realtime` package structure
- Implement protocol types and message parsing
- Implement Hub with connection tracking
- Implement Conn with read/write pumps
- WebSocket handler and route registration
- Basic heartbeat handling

### Phase 2: Channels & Broadcast
- Channel join/leave logic
- JWT validation for private channels
- Broadcast send/receive
- Self and ack configuration options
- Basic E2E tests for broadcast

### Phase 3: Presence
- PresenceState data structure
- Track/untrack operations
- presence_state on join
- presence_diff broadcasts
- Cleanup on disconnect
- E2E tests for presence

### Phase 4: Postgres Changes
- ChangeEvent structure
- REST handler integration (NotifyChange calls)
- Filter parsing and evaluation
- RLS enforcement for change events
- Subscription matching logic
- E2E tests for postgres_changes

### Phase 5: Dashboard & Polish
- Stats endpoint implementation
- Dashboard UI for monitoring
- Documentation (docs/realtime.md)
- Update README.md
- Remaining E2E tests
- Error handling edge cases

## Dependencies

- `github.com/gorilla/websocket` - WebSocket implementation

## Design Decisions Summary

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| Features | All three (Broadcast, Presence, Postgres Changes) | Full Supabase compatibility |
| WebSocket lib | gorilla/websocket | Mature, well-tested, widely used |
| Change detection | Hook into REST handlers | Simple, reliable, sufficient for REST API usage |
| Channel auth | Full RLS integration | Matches Supabase security model |
| Filter evaluation | Server-side | Reduces bandwidth, matches Supabase |
| Protocol version | Phoenix v1.0.0 (JSON only) | Sufficient for supabase-js compatibility |
| Dashboard | Basic monitoring UI | Useful for debugging without complexity |
