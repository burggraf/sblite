# Realtime Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Supabase-compatible Realtime WebSocket server with Broadcast, Presence, and Postgres Changes features.

**Architecture:** WebSocket server using gorilla/websocket with Phoenix Protocol v1.0.0. Hub manages connections and channels, REST handlers emit change events. Full RLS integration for authorization.

**Tech Stack:** Go, gorilla/websocket, Chi router, existing RLS/JWT infrastructure

---

## Phase 1: Core Infrastructure

### Task 1.1: Add gorilla/websocket dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add dependency**

Run: `go get github.com/gorilla/websocket`

**Step 2: Verify dependency added**

Run: `grep gorilla go.mod`
Expected: `github.com/gorilla/websocket v1.x.x`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add gorilla/websocket for realtime support"
```

---

### Task 1.2: Create protocol types

**Files:**
- Create: `internal/realtime/protocol.go`
- Create: `internal/realtime/protocol_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/protocol_test.go
package realtime

import (
	"encoding/json"
	"testing"
)

func TestMessageMarshal(t *testing.T) {
	msg := Message{
		Event:   "phx_join",
		Topic:   "realtime:test",
		Payload: map[string]any{"key": "value"},
		Ref:     "1",
		JoinRef: "1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Event != msg.Event {
		t.Errorf("event mismatch: got %s, want %s", decoded.Event, msg.Event)
	}
	if decoded.Topic != msg.Topic {
		t.Errorf("topic mismatch: got %s, want %s", decoded.Topic, msg.Topic)
	}
	if decoded.Ref != msg.Ref {
		t.Errorf("ref mismatch: got %s, want %s", decoded.Ref, msg.Ref)
	}
}

func TestParseJoinConfig(t *testing.T) {
	payload := map[string]any{
		"config": map[string]any{
			"broadcast": map[string]any{
				"ack":  true,
				"self": false,
			},
			"presence": map[string]any{
				"key": "user-123",
			},
			"postgres_changes": []any{
				map[string]any{
					"event":  "INSERT",
					"schema": "public",
					"table":  "messages",
				},
			},
			"private": true,
		},
		"access_token": "test-token",
	}

	config, token, err := ParseJoinPayload(payload)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if !config.Broadcast.Ack {
		t.Error("broadcast.ack should be true")
	}
	if config.Broadcast.Self {
		t.Error("broadcast.self should be false")
	}
	if config.Presence.Key != "user-123" {
		t.Errorf("presence.key mismatch: got %s", config.Presence.Key)
	}
	if len(config.PostgresChanges) != 1 {
		t.Fatalf("postgres_changes length mismatch: got %d", len(config.PostgresChanges))
	}
	if config.PostgresChanges[0].Event != "INSERT" {
		t.Errorf("postgres_changes[0].event mismatch: got %s", config.PostgresChanges[0].Event)
	}
	if !config.Private {
		t.Error("private should be true")
	}
	if token != "test-token" {
		t.Errorf("token mismatch: got %s", token)
	}
}

func TestReplyMessage(t *testing.T) {
	reply := NewReply("realtime:test", "1", "1", "ok", map[string]any{"msg": "joined"})

	if reply.Event != "phx_reply" {
		t.Errorf("event should be phx_reply, got %s", reply.Event)
	}

	response, ok := reply.Payload["response"].(map[string]any)
	if !ok {
		t.Fatal("response should be map")
	}
	if response["msg"] != "joined" {
		t.Errorf("response.msg mismatch: got %v", response["msg"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write the implementation**

```go
// internal/realtime/protocol.go
package realtime

import (
	"encoding/json"
	"fmt"
)

// Phoenix Protocol v1.0.0 message format
type Message struct {
	Event   string         `json:"event"`
	Topic   string         `json:"topic"`
	Payload map[string]any `json:"payload"`
	Ref     string         `json:"ref"`
	JoinRef string         `json:"join_ref,omitempty"`
}

// Client events
const (
	EventJoin        = "phx_join"
	EventLeave       = "phx_leave"
	EventHeartbeat   = "heartbeat"
	EventAccessToken = "access_token"
	EventBroadcast   = "broadcast"
	EventPresence    = "presence"
)

// Server events
const (
	EventReply         = "phx_reply"
	EventClose         = "phx_close"
	EventError         = "phx_error"
	EventSystem        = "system"
	EventPostgres      = "postgres_changes"
	EventPresenceState = "presence_state"
	EventPresenceDiff  = "presence_diff"
)

// Phoenix topic for heartbeats
const TopicPhoenix = "phoenix"

// JoinConfig holds channel join configuration
type JoinConfig struct {
	Broadcast       BroadcastConfig      `json:"broadcast"`
	Presence        PresenceConfig       `json:"presence"`
	PostgresChanges []PostgresChangeSub  `json:"postgres_changes"`
	Private         bool                 `json:"private"`
}

// BroadcastConfig holds broadcast options
type BroadcastConfig struct {
	Ack  bool `json:"ack"`  // wait for server ack
	Self bool `json:"self"` // receive own broadcasts
}

// PresenceConfig holds presence options
type PresenceConfig struct {
	Key string `json:"key"` // presence key (e.g., user ID)
}

// PostgresChangeSub holds a postgres_changes subscription
type PostgresChangeSub struct {
	Event  string `json:"event"`  // INSERT, UPDATE, DELETE, *
	Schema string `json:"schema"` // "public"
	Table  string `json:"table"`  // table name or "*"
	Filter string `json:"filter"` // e.g., "user_id=eq.123"
	ID     int    `json:"id"`     // subscription ID (assigned by server)
}

// ChangeEvent represents a database change
type ChangeEvent struct {
	Schema          string         `json:"schema"`
	Table           string         `json:"table"`
	CommitTimestamp string         `json:"commit_timestamp"`
	EventType       string         `json:"eventType"` // INSERT, UPDATE, DELETE
	New             map[string]any `json:"new"`
	Old             map[string]any `json:"old"`
	Errors          []string       `json:"errors"`
}

// ParseJoinPayload extracts JoinConfig and access_token from phx_join payload
func ParseJoinPayload(payload map[string]any) (*JoinConfig, string, error) {
	config := &JoinConfig{}

	// Extract access_token
	token, _ := payload["access_token"].(string)

	// Extract config
	configMap, ok := payload["config"].(map[string]any)
	if !ok {
		// Config is optional, return defaults
		return config, token, nil
	}

	// Parse broadcast config
	if bc, ok := configMap["broadcast"].(map[string]any); ok {
		if ack, ok := bc["ack"].(bool); ok {
			config.Broadcast.Ack = ack
		}
		if self, ok := bc["self"].(bool); ok {
			config.Broadcast.Self = self
		}
	}

	// Parse presence config
	if pc, ok := configMap["presence"].(map[string]any); ok {
		if key, ok := pc["key"].(string); ok {
			config.Presence.Key = key
		}
	}

	// Parse postgres_changes
	if pgc, ok := configMap["postgres_changes"].([]any); ok {
		for _, item := range pgc {
			if sub, ok := item.(map[string]any); ok {
				pgSub := PostgresChangeSub{}
				if event, ok := sub["event"].(string); ok {
					pgSub.Event = event
				}
				if schema, ok := sub["schema"].(string); ok {
					pgSub.Schema = schema
				}
				if table, ok := sub["table"].(string); ok {
					pgSub.Table = table
				}
				if filter, ok := sub["filter"].(string); ok {
					pgSub.Filter = filter
				}
				config.PostgresChanges = append(config.PostgresChanges, pgSub)
			}
		}
	}

	// Parse private flag
	if private, ok := configMap["private"].(bool); ok {
		config.Private = private
	}

	return config, token, nil
}

// NewReply creates a phx_reply message
func NewReply(topic, joinRef, ref, status string, response map[string]any) *Message {
	return &Message{
		Event:   EventReply,
		Topic:   topic,
		JoinRef: joinRef,
		Ref:     ref,
		Payload: map[string]any{
			"status":   status,
			"response": response,
		},
	}
}

// NewSystemMessage creates a system message for subscription status
func NewSystemMessage(topic, joinRef string, status, message, extension string) *Message {
	return &Message{
		Event:   EventSystem,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: map[string]any{
			"status":    status,
			"message":   message,
			"extension": extension,
		},
	}
}

// NewBroadcastMessage creates a broadcast message
func NewBroadcastMessage(topic string, event string, payload map[string]any) *Message {
	return &Message{
		Event: EventBroadcast,
		Topic: topic,
		Payload: map[string]any{
			"type":    "broadcast",
			"event":   event,
			"payload": payload,
		},
	}
}

// NewPostgresChangeMessage creates a postgres_changes message
func NewPostgresChangeMessage(topic, joinRef string, ids []int, event ChangeEvent) *Message {
	return &Message{
		Event:   EventPostgres,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: map[string]any{
			"ids":  ids,
			"data": event,
		},
	}
}

// NewPresenceStateMessage creates a presence_state message
func NewPresenceStateMessage(topic, joinRef string, state map[string][]map[string]any) *Message {
	return &Message{
		Event:   EventPresenceState,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: state,
	}
}

// NewPresenceDiffMessage creates a presence_diff message
func NewPresenceDiffMessage(topic, joinRef string, joins, leaves map[string][]map[string]any) *Message {
	return &Message{
		Event:   EventPresenceDiff,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: map[string]any{
			"joins":  joins,
			"leaves": leaves,
		},
	}
}

// Encode serializes a message to JSON bytes
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage parses JSON bytes into a Message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("invalid message format: %w", err)
	}
	return &msg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/protocol.go internal/realtime/protocol_test.go
git commit -m "feat(realtime): add Phoenix protocol message types"
```

---

### Task 1.3: Create Hub structure

**Files:**
- Create: `internal/realtime/hub.go`
- Create: `internal/realtime/hub_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/hub_test.go
package realtime

import (
	"testing"
)

func TestNewHub(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")
	if hub == nil {
		t.Fatal("hub should not be nil")
	}
	if hub.connections == nil {
		t.Error("connections map should be initialized")
	}
	if hub.channels == nil {
		t.Error("channels map should be initialized")
	}
}

func TestHubStats(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	stats := hub.Stats()
	if stats.Connections != 0 {
		t.Errorf("expected 0 connections, got %d", stats.Connections)
	}
	if stats.Channels != 0 {
		t.Errorf("expected 0 channels, got %d", stats.Channels)
	}
}

func TestHubGetOrCreateChannel(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	// First call creates channel
	ch1 := hub.getOrCreateChannel("realtime:test", false)
	if ch1 == nil {
		t.Fatal("channel should be created")
	}
	if ch1.topic != "realtime:test" {
		t.Errorf("topic mismatch: got %s", ch1.topic)
	}

	// Second call returns same channel
	ch2 := hub.getOrCreateChannel("realtime:test", false)
	if ch1 != ch2 {
		t.Error("should return same channel instance")
	}

	stats := hub.Stats()
	if stats.Channels != 1 {
		t.Errorf("expected 1 channel, got %d", stats.Channels)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v -run TestHub`
Expected: FAIL (Hub not defined)

**Step 3: Write the implementation**

```go
// internal/realtime/hub.go
package realtime

import (
	"database/sql"
	"sync"

	"github.com/markb/sblite/internal/rls"
)

// Hub manages all WebSocket connections and channels
type Hub struct {
	mu          sync.RWMutex
	connections map[string]*Conn    // connID -> Conn
	channels    map[string]*Channel // topic -> Channel

	db         *sql.DB
	rlsService *rls.Service
	jwtSecret  string
}

// HubStats contains realtime statistics
type HubStats struct {
	Connections    int            `json:"connections"`
	Channels       int            `json:"channels"`
	ChannelDetails []ChannelStats `json:"channel_details"`
}

// ChannelStats contains per-channel statistics
type ChannelStats struct {
	Topic       string `json:"topic"`
	Subscribers int    `json:"subscribers"`
	HasPresence bool   `json:"has_presence"`
}

// NewHub creates a new Hub
func NewHub(db *sql.DB, rlsService *rls.Service, jwtSecret string) *Hub {
	return &Hub{
		connections: make(map[string]*Conn),
		channels:    make(map[string]*Channel),
		db:          db,
		rlsService:  rlsService,
		jwtSecret:   jwtSecret,
	}
}

// Stats returns current realtime statistics
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HubStats{
		Connections:    len(h.connections),
		Channels:       len(h.channels),
		ChannelDetails: make([]ChannelStats, 0, len(h.channels)),
	}

	for _, ch := range h.channels {
		ch.mu.RLock()
		stats.ChannelDetails = append(stats.ChannelDetails, ChannelStats{
			Topic:       ch.topic,
			Subscribers: len(ch.subscribers),
			HasPresence: ch.presence != nil,
		})
		ch.mu.RUnlock()
	}

	return stats
}

// registerConn adds a connection to the hub
func (h *Hub) registerConn(conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[conn.id] = conn
}

// unregisterConn removes a connection from the hub and all channels
func (h *Hub) unregisterConn(conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.connections, conn.id)

	// Remove from all channels
	for topic, ch := range h.channels {
		ch.mu.Lock()
		if _, ok := ch.subscribers[conn.id]; ok {
			delete(ch.subscribers, conn.id)
			// Clean up empty channels
			if len(ch.subscribers) == 0 {
				delete(h.channels, topic)
			}
		}
		ch.mu.Unlock()
	}
}

// getOrCreateChannel gets or creates a channel by topic
func (h *Hub) getOrCreateChannel(topic string, private bool) *Channel {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.channels[topic]; ok {
		return ch
	}

	ch := &Channel{
		topic:       topic,
		private:     private,
		subscribers: make(map[string]*ChannelSub),
	}
	h.channels[topic] = ch
	return ch
}

// getChannel returns a channel by topic, or nil if not found
func (h *Hub) getChannel(topic string) *Channel {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.channels[topic]
}

// removeChannelIfEmpty removes a channel if it has no subscribers
func (h *Hub) removeChannelIfEmpty(topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.channels[topic]; ok {
		ch.mu.RLock()
		empty := len(ch.subscribers) == 0
		ch.mu.RUnlock()
		if empty {
			delete(h.channels, topic)
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v -run TestHub`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/hub.go internal/realtime/hub_test.go
git commit -m "feat(realtime): add Hub for connection and channel management"
```

---

### Task 1.4: Create Channel structure

**Files:**
- Create: `internal/realtime/channel.go`
- Create: `internal/realtime/channel_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/channel_test.go
package realtime

import (
	"testing"
)

func TestChannelAddSubscriber(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub := &ChannelSub{
		joinRef: "1",
		broadcastConfig: BroadcastConfig{
			Ack:  true,
			Self: false,
		},
	}

	ch.addSubscriber("conn-1", sub)

	if len(ch.subscribers) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(ch.subscribers))
	}

	if ch.subscribers["conn-1"] != sub {
		t.Error("subscriber not added correctly")
	}
}

func TestChannelRemoveSubscriber(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub := &ChannelSub{joinRef: "1"}
	ch.addSubscriber("conn-1", sub)
	ch.removeSubscriber("conn-1")

	if len(ch.subscribers) != 0 {
		t.Errorf("expected 0 subscribers, got %d", len(ch.subscribers))
	}
}

func TestChannelGetSubscribers(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub1 := &ChannelSub{joinRef: "1"}
	sub2 := &ChannelSub{joinRef: "2"}
	ch.addSubscriber("conn-1", sub1)
	ch.addSubscriber("conn-2", sub2)

	subs := ch.getSubscribers()
	if len(subs) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(subs))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v -run TestChannel`
Expected: FAIL (Channel methods not defined)

**Step 3: Write the implementation**

```go
// internal/realtime/channel.go
package realtime

import (
	"sync"
)

// Channel represents a realtime channel with subscribers
type Channel struct {
	topic       string
	private     bool
	mu          sync.RWMutex
	subscribers map[string]*ChannelSub // connID -> subscription
	presence    *PresenceState         // nil if presence not enabled
}

// ChannelSub represents a connection's subscription to a channel
type ChannelSub struct {
	conn            *Conn
	joinRef         string
	broadcastConfig BroadcastConfig
	presenceConfig  PresenceConfig
	pgChanges       []PostgresChangeSub
}

// addSubscriber adds a subscription to the channel
func (ch *Channel) addSubscriber(connID string, sub *ChannelSub) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.subscribers[connID] = sub
}

// removeSubscriber removes a subscription from the channel
func (ch *Channel) removeSubscriber(connID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.subscribers, connID)
}

// getSubscriber returns a subscriber by connection ID
func (ch *Channel) getSubscriber(connID string) *ChannelSub {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.subscribers[connID]
}

// getSubscribers returns all subscribers (snapshot)
func (ch *Channel) getSubscribers() []*ChannelSub {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	subs := make([]*ChannelSub, 0, len(ch.subscribers))
	for _, sub := range ch.subscribers {
		subs = append(subs, sub)
	}
	return subs
}

// isEmpty returns true if the channel has no subscribers
func (ch *Channel) isEmpty() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.subscribers) == 0
}

// enablePresence initializes presence state for the channel
func (ch *Channel) enablePresence() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.presence == nil {
		ch.presence = NewPresenceState()
	}
}

// getPresence returns the presence state (may be nil)
func (ch *Channel) getPresence() *PresenceState {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.presence
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v -run TestChannel`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/channel.go internal/realtime/channel_test.go
git commit -m "feat(realtime): add Channel subscription management"
```

---

### Task 1.5: Create Connection structure with read/write pumps

**Files:**
- Create: `internal/realtime/conn.go`
- Create: `internal/realtime/conn_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/conn_test.go
package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewConnID(t *testing.T) {
	// Test that connection IDs are valid UUIDs
	id := uuid.New().String()
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("generated ID is not a valid UUID: %s", id)
	}
}

func TestConnSendChannel(t *testing.T) {
	// Test that send channel is buffered
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Should not block for buffered sends
	msg := []byte(`{"event":"test"}`)
	select {
	case conn.send <- msg:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("send channel should not block")
	}
}

func TestConnClose(t *testing.T) {
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Close should be idempotent
	conn.Close()
	conn.Close() // Should not panic
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v -run TestConn`
Expected: FAIL (sendBufferSize not defined)

**Step 3: Write the implementation**

```go
// internal/realtime/conn.go
package realtime

import (
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/markb/sblite/internal/log"
)

const (
	// Send buffer size for outbound messages
	sendBufferSize = 256

	// Time allowed to write a message
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message
	pongWait = 30 * time.Second

	// Send pings with this period (must be less than pongWait)
	pingPeriod = 25 * time.Second

	// Maximum message size
	maxMessageSize = 512 * 1024 // 512KB
)

// Conn represents a WebSocket connection
type Conn struct {
	id       string
	ws       *websocket.Conn
	hub      *Hub
	mu       sync.Mutex
	channels map[string]*ChannelSub // topic -> subscription
	claims   jwt.MapClaims         // parsed from access_token
	send     chan []byte            // outbound message queue
	done     chan struct{}          // closed when connection ends
	closeOnce sync.Once
}

// NewConn creates a new connection
func (h *Hub) NewConn(ws *websocket.Conn) *Conn {
	conn := &Conn{
		id:       uuid.New().String(),
		ws:       ws,
		hub:      h,
		channels: make(map[string]*ChannelSub),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
	}
	h.registerConn(conn)
	return conn
}

// ID returns the connection ID
func (c *Conn) ID() string {
	return c.id
}

// Send queues a message for sending
func (c *Conn) Send(msg *Message) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}
	select {
	case c.send <- data:
		return nil
	case <-c.done:
		return nil // Connection closed
	default:
		// Buffer full, drop message
		log.Warn("realtime: send buffer full, dropping message", "conn_id", c.id)
		return nil
	}
}

// Close closes the connection
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.ws != nil {
			c.ws.Close()
		}
		c.hub.unregisterConn(c)
	})
}

// ReadPump reads messages from the WebSocket connection
func (c *Conn) ReadPump() {
	defer c.Close()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debug("realtime: read error", "conn_id", c.id, "error", err.Error())
			}
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil {
			log.Debug("realtime: invalid message", "conn_id", c.id, "error", err.Error())
			continue
		}

		c.handleMessage(msg)
	}
}

// WritePump writes messages to the WebSocket connection
func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// handleMessage routes incoming messages to appropriate handlers
func (c *Conn) handleMessage(msg *Message) {
	switch msg.Event {
	case EventHeartbeat:
		c.handleHeartbeat(msg)
	case EventJoin:
		c.handleJoin(msg)
	case EventLeave:
		c.handleLeave(msg)
	case EventBroadcast:
		c.handleBroadcast(msg)
	case EventPresence:
		c.handlePresence(msg)
	case EventAccessToken:
		c.handleAccessToken(msg)
	default:
		log.Debug("realtime: unknown event", "conn_id", c.id, "event", msg.Event)
	}
}

// handleHeartbeat responds to heartbeat messages
func (c *Conn) handleHeartbeat(msg *Message) {
	reply := NewReply(TopicPhoenix, "", msg.Ref, "ok", map[string]any{})
	c.Send(reply)
}

// handleJoin handles channel join requests
func (c *Conn) handleJoin(msg *Message) {
	config, token, err := ParseJoinPayload(msg.Payload)
	if err != nil {
		c.sendError(msg.Topic, msg.JoinRef, msg.Ref, "invalid_payload", err.Error())
		return
	}

	// Validate JWT if provided
	if token != "" {
		claims, err := c.hub.validateToken(token)
		if err != nil {
			c.sendError(msg.Topic, msg.JoinRef, msg.Ref, "invalid_token", err.Error())
			return
		}
		c.claims = claims
	}

	// Check authorization for private channels
	if config.Private && c.claims == nil {
		c.sendError(msg.Topic, msg.JoinRef, msg.Ref, "unauthorized", "private channel requires authentication")
		return
	}

	// Get or create channel
	ch := c.hub.getOrCreateChannel(msg.Topic, config.Private)

	// Assign subscription IDs to postgres_changes
	for i := range config.PostgresChanges {
		config.PostgresChanges[i].ID = i + 1
	}

	// Create subscription
	sub := &ChannelSub{
		conn:            c,
		joinRef:         msg.JoinRef,
		broadcastConfig: config.Broadcast,
		presenceConfig:  config.Presence,
		pgChanges:       config.PostgresChanges,
	}

	// Enable presence if configured
	if config.Presence.Key != "" {
		ch.enablePresence()
	}

	// Add subscriber
	ch.addSubscriber(c.id, sub)

	// Track channel subscription on connection
	c.mu.Lock()
	c.channels[msg.Topic] = sub
	c.mu.Unlock()

	// Send join reply
	reply := NewReply(msg.Topic, msg.JoinRef, msg.Ref, "ok", map[string]any{})
	c.Send(reply)

	// Send system messages for postgres_changes subscriptions
	for _, pgSub := range config.PostgresChanges {
		sysMsg := NewSystemMessage(msg.Topic, msg.JoinRef, "ok",
			"Subscribed to PostgreSQL", "postgres_changes")
		sysMsg.Payload["subscription_id"] = pgSub.ID
		c.Send(sysMsg)
	}

	// Send presence state if enabled
	if presence := ch.getPresence(); presence != nil {
		state := presence.GetState()
		stateMsg := NewPresenceStateMessage(msg.Topic, msg.JoinRef, state)
		c.Send(stateMsg)
	}
}

// handleLeave handles channel leave requests
func (c *Conn) handleLeave(msg *Message) {
	c.mu.Lock()
	sub, ok := c.channels[msg.Topic]
	if ok {
		delete(c.channels, msg.Topic)
	}
	c.mu.Unlock()

	if !ok {
		c.sendError(msg.Topic, "", msg.Ref, "not_joined", "not subscribed to channel")
		return
	}

	// Handle presence leave
	ch := c.hub.getChannel(msg.Topic)
	if ch != nil {
		if presence := ch.getPresence(); presence != nil && sub.presenceConfig.Key != "" {
			leaves := presence.Untrack(sub.presenceConfig.Key, c.id)
			if len(leaves) > 0 {
				diff := NewPresenceDiffMessage(msg.Topic, sub.joinRef,
					map[string][]map[string]any{},
					map[string][]map[string]any{sub.presenceConfig.Key: leaves})
				c.broadcastToChannel(ch, diff, "")
			}
		}

		ch.removeSubscriber(c.id)
		c.hub.removeChannelIfEmpty(msg.Topic)
	}

	reply := NewReply(msg.Topic, "", msg.Ref, "ok", map[string]any{})
	c.Send(reply)
}

// handleBroadcast handles broadcast messages
func (c *Conn) handleBroadcast(msg *Message) {
	c.mu.Lock()
	sub, ok := c.channels[msg.Topic]
	c.mu.Unlock()

	if !ok {
		return
	}

	ch := c.hub.getChannel(msg.Topic)
	if ch == nil {
		return
	}

	// Extract broadcast payload
	payload, _ := msg.Payload["payload"].(map[string]any)
	event, _ := msg.Payload["event"].(string)

	broadcastMsg := NewBroadcastMessage(msg.Topic, event, payload)

	// Broadcast to channel, respecting self flag
	excludeID := ""
	if !sub.broadcastConfig.Self {
		excludeID = c.id
	}
	c.broadcastToChannel(ch, broadcastMsg, excludeID)

	// Send ack if configured
	if sub.broadcastConfig.Ack {
		reply := NewReply(msg.Topic, sub.joinRef, msg.Ref, "ok", map[string]any{})
		c.Send(reply)
	}
}

// handlePresence handles presence messages
func (c *Conn) handlePresence(msg *Message) {
	c.mu.Lock()
	sub, ok := c.channels[msg.Topic]
	c.mu.Unlock()

	if !ok || sub.presenceConfig.Key == "" {
		return
	}

	ch := c.hub.getChannel(msg.Topic)
	if ch == nil {
		return
	}

	presence := ch.getPresence()
	if presence == nil {
		return
	}

	eventType, _ := msg.Payload["event"].(string)
	payload, _ := msg.Payload["payload"].(map[string]any)

	switch eventType {
	case "track":
		meta := presence.Track(sub.presenceConfig.Key, c.id, payload)
		joins := map[string][]map[string]any{sub.presenceConfig.Key: {meta}}
		diff := NewPresenceDiffMessage(msg.Topic, sub.joinRef, joins, map[string][]map[string]any{})
		c.broadcastToChannel(ch, diff, "")
	}
}

// handleAccessToken refreshes the connection's JWT
func (c *Conn) handleAccessToken(msg *Message) {
	token, _ := msg.Payload["access_token"].(string)
	if token == "" {
		return
	}

	claims, err := c.hub.validateToken(token)
	if err != nil {
		log.Debug("realtime: invalid access_token refresh", "conn_id", c.id, "error", err.Error())
		return
	}
	c.claims = claims
}

// broadcastToChannel sends a message to all channel subscribers
func (c *Conn) broadcastToChannel(ch *Channel, msg *Message, excludeConnID string) {
	for _, sub := range ch.getSubscribers() {
		if sub.conn.id == excludeConnID {
			continue
		}
		sub.conn.Send(msg)
	}
}

// sendError sends an error reply
func (c *Conn) sendError(topic, joinRef, ref, code, message string) {
	reply := NewReply(topic, joinRef, ref, "error", map[string]any{
		"code":    code,
		"message": message,
	})
	c.Send(reply)
}

// validateToken validates a JWT and returns claims
func (h *Hub) validateToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v -run TestConn`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/conn.go internal/realtime/conn_test.go
git commit -m "feat(realtime): add WebSocket connection with read/write pumps"
```

---

### Task 1.6: Create Presence state management

**Files:**
- Create: `internal/realtime/presence.go`
- Create: `internal/realtime/presence_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/presence_test.go
package realtime

import (
	"testing"
)

func TestPresenceTrack(t *testing.T) {
	ps := NewPresenceState()

	meta := ps.Track("user-1", "conn-1", map[string]any{"status": "online"})

	if meta["status"] != "online" {
		t.Errorf("status mismatch: got %v", meta["status"])
	}
	if meta["phx_ref"] == "" {
		t.Error("phx_ref should be set")
	}

	state := ps.GetState()
	if len(state["user-1"]) != 1 {
		t.Errorf("expected 1 presence for user-1, got %d", len(state["user-1"]))
	}
}

func TestPresenceMultipleConnections(t *testing.T) {
	ps := NewPresenceState()

	ps.Track("user-1", "conn-1", map[string]any{"device": "desktop"})
	ps.Track("user-1", "conn-2", map[string]any{"device": "mobile"})

	state := ps.GetState()
	if len(state["user-1"]) != 2 {
		t.Errorf("expected 2 presences for user-1, got %d", len(state["user-1"]))
	}
}

func TestPresenceUntrack(t *testing.T) {
	ps := NewPresenceState()

	ps.Track("user-1", "conn-1", map[string]any{"status": "online"})
	leaves := ps.Untrack("user-1", "conn-1")

	if len(leaves) != 1 {
		t.Errorf("expected 1 leave, got %d", len(leaves))
	}

	state := ps.GetState()
	if len(state["user-1"]) != 0 {
		t.Errorf("expected 0 presences for user-1 after untrack, got %d", len(state["user-1"]))
	}
}

func TestPresenceUntrackNonexistent(t *testing.T) {
	ps := NewPresenceState()

	leaves := ps.Untrack("user-1", "conn-1")
	if len(leaves) != 0 {
		t.Errorf("expected 0 leaves for nonexistent, got %d", len(leaves))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v -run TestPresence`
Expected: FAIL (NewPresenceState not defined)

**Step 3: Write the implementation**

```go
// internal/realtime/presence.go
package realtime

import (
	"sync"

	"github.com/google/uuid"
)

// PresenceState tracks presence for a channel
type PresenceState struct {
	mu    sync.RWMutex
	state map[string][]presenceMeta // presenceKey -> list of metas
}

// presenceMeta stores metadata for a single presence entry
type presenceMeta struct {
	connID  string
	phxRef  string
	payload map[string]any
}

// NewPresenceState creates a new presence state
func NewPresenceState() *PresenceState {
	return &PresenceState{
		state: make(map[string][]presenceMeta),
	}
}

// Track adds or updates presence for a key/connection
func (ps *PresenceState) Track(key, connID string, payload map[string]any) map[string]any {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	phxRef := uuid.New().String()[:8]

	meta := presenceMeta{
		connID:  connID,
		phxRef:  phxRef,
		payload: payload,
	}

	// Check if this connection already has presence for this key
	metas := ps.state[key]
	found := false
	for i, m := range metas {
		if m.connID == connID {
			metas[i] = meta
			found = true
			break
		}
	}

	if !found {
		ps.state[key] = append(metas, meta)
	}

	// Return the meta as a map for broadcasting
	return ps.metaToMap(meta)
}

// Untrack removes presence for a key/connection
func (ps *PresenceState) Untrack(key, connID string) []map[string]any {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	metas := ps.state[key]
	var leaves []map[string]any
	var remaining []presenceMeta

	for _, m := range metas {
		if m.connID == connID {
			leaves = append(leaves, ps.metaToMap(m))
		} else {
			remaining = append(remaining, m)
		}
	}

	if len(remaining) == 0 {
		delete(ps.state, key)
	} else {
		ps.state[key] = remaining
	}

	return leaves
}

// UntrackConn removes all presences for a connection
func (ps *PresenceState) UntrackConn(connID string) map[string][]map[string]any {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	leaves := make(map[string][]map[string]any)

	for key, metas := range ps.state {
		var remaining []presenceMeta
		for _, m := range metas {
			if m.connID == connID {
				if leaves[key] == nil {
					leaves[key] = []map[string]any{}
				}
				leaves[key] = append(leaves[key], ps.metaToMap(m))
			} else {
				remaining = append(remaining, m)
			}
		}

		if len(remaining) == 0 {
			delete(ps.state, key)
		} else {
			ps.state[key] = remaining
		}
	}

	return leaves
}

// GetState returns the full presence state for broadcasting
func (ps *PresenceState) GetState() map[string][]map[string]any {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make(map[string][]map[string]any)
	for key, metas := range ps.state {
		result[key] = make([]map[string]any, len(metas))
		for i, m := range metas {
			result[key][i] = ps.metaToMap(m)
		}
	}
	return result
}

// metaToMap converts internal meta to broadcast format
func (ps *PresenceState) metaToMap(m presenceMeta) map[string]any {
	result := make(map[string]any)
	for k, v := range m.payload {
		result[k] = v
	}
	result["phx_ref"] = m.phxRef
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v -run TestPresence`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/presence.go internal/realtime/presence_test.go
git commit -m "feat(realtime): add Presence state tracking"
```

---

### Task 1.7: Create Service and HTTP handler

**Files:**
- Create: `internal/realtime/realtime.go`
- Create: `internal/realtime/handler.go`

**Step 1: Write the service file**

```go
// internal/realtime/realtime.go
package realtime

import (
	"database/sql"

	"github.com/markb/sblite/internal/rls"
)

// Service provides realtime functionality
type Service struct {
	hub       *Hub
	db        *sql.DB
	jwtSecret string
	anonKey   string
	serviceKey string
}

// Config holds realtime configuration
type Config struct {
	JWTSecret  string
	AnonKey    string
	ServiceKey string
}

// NewService creates a new realtime service
func NewService(db *sql.DB, rlsService *rls.Service, cfg Config) *Service {
	return &Service{
		hub:        NewHub(db, rlsService, cfg.JWTSecret),
		db:         db,
		jwtSecret:  cfg.JWTSecret,
		anonKey:    cfg.AnonKey,
		serviceKey: cfg.ServiceKey,
	}
}

// Hub returns the connection hub
func (s *Service) Hub() *Hub {
	return s.hub
}

// Stats returns realtime statistics
func (s *Service) Stats() HubStats {
	return s.hub.Stats()
}

// NotifyChange broadcasts a database change to subscribers
func (s *Service) NotifyChange(schema, table, eventType string, oldRow, newRow map[string]any) {
	s.hub.broadcastChange(schema, table, eventType, oldRow, newRow)
}
```

**Step 2: Write the handler file**

```go
// internal/realtime/handler.go
package realtime

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/markb/sblite/internal/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (CORS handled elsewhere)
	},
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		apiKey = r.Header.Get("apikey")
	}

	if !s.validateAPIKey(apiKey) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("realtime: upgrade failed", "error", err.Error())
		return
	}

	// Create connection and start pumps
	conn := s.hub.NewConn(ws)
	log.Debug("realtime: new connection", "conn_id", conn.ID())

	go conn.WritePump()
	go conn.ReadPump()
}

// validateAPIKey checks if the API key is valid
func (s *Service) validateAPIKey(key string) bool {
	return key == s.anonKey || key == s.serviceKey
}
```

**Step 3: Add broadcastChange to Hub**

Add to `internal/realtime/hub.go`:

```go
// broadcastChange sends a change event to matching subscribers
func (h *Hub) broadcastChange(schema, table, eventType string, oldRow, newRow map[string]any) {
	event := ChangeEvent{
		Schema:          schema,
		Table:           table,
		CommitTimestamp: time.Now().UTC().Format(time.RFC3339),
		EventType:       eventType,
		New:             newRow,
		Old:             oldRow,
		Errors:          []string{},
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.channels {
		for _, sub := range ch.getSubscribers() {
			matchingIDs := h.getMatchingSubscriptionIDs(sub.pgChanges, event)
			if len(matchingIDs) > 0 {
				msg := NewPostgresChangeMessage(ch.topic, sub.joinRef, matchingIDs, event)
				sub.conn.Send(msg)
			}
		}
	}
}

// getMatchingSubscriptionIDs returns IDs of subscriptions matching the event
func (h *Hub) getMatchingSubscriptionIDs(subs []PostgresChangeSub, event ChangeEvent) []int {
	var ids []int
	for _, sub := range subs {
		if h.matchesSubscription(sub, event) {
			ids = append(ids, sub.ID)
		}
	}
	return ids
}

// matchesSubscription checks if an event matches a subscription
func (h *Hub) matchesSubscription(sub PostgresChangeSub, event ChangeEvent) bool {
	// Check event type
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
	// Check filter (if any)
	if sub.Filter != "" {
		return matchesFilter(sub.Filter, event.New, event.Old)
	}
	return true
}
```

**Step 4: Run all tests**

Run: `go test ./internal/realtime/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/realtime.go internal/realtime/handler.go internal/realtime/hub.go
git commit -m "feat(realtime): add Service and WebSocket handler"
```

---

### Task 1.8: Create filter evaluation

**Files:**
- Create: `internal/realtime/filter.go`
- Create: `internal/realtime/filter_test.go`

**Step 1: Write the test file**

```go
// internal/realtime/filter_test.go
package realtime

import (
	"testing"
)

func TestMatchesFilterEq(t *testing.T) {
	row := map[string]any{"status": "active", "count": 5}

	if !matchesFilter("status=eq.active", row, nil) {
		t.Error("should match status=eq.active")
	}
	if matchesFilter("status=eq.inactive", row, nil) {
		t.Error("should not match status=eq.inactive")
	}
}

func TestMatchesFilterNeq(t *testing.T) {
	row := map[string]any{"status": "active"}

	if !matchesFilter("status=neq.inactive", row, nil) {
		t.Error("should match status=neq.inactive")
	}
	if matchesFilter("status=neq.active", row, nil) {
		t.Error("should not match status=neq.active")
	}
}

func TestMatchesFilterNumeric(t *testing.T) {
	row := map[string]any{"count": float64(5)}

	if !matchesFilter("count=gt.3", row, nil) {
		t.Error("should match count=gt.3")
	}
	if !matchesFilter("count=gte.5", row, nil) {
		t.Error("should match count=gte.5")
	}
	if !matchesFilter("count=lt.10", row, nil) {
		t.Error("should match count=lt.10")
	}
	if !matchesFilter("count=lte.5", row, nil) {
		t.Error("should match count=lte.5")
	}
}

func TestMatchesFilterIn(t *testing.T) {
	row := map[string]any{"status": "active"}

	if !matchesFilter("status=in.(active,pending)", row, nil) {
		t.Error("should match status in (active,pending)")
	}
	if matchesFilter("status=in.(inactive,deleted)", row, nil) {
		t.Error("should not match status in (inactive,deleted)")
	}
}

func TestMatchesFilterInvalidFormat(t *testing.T) {
	row := map[string]any{"status": "active"}

	// Invalid filter formats should not match
	if matchesFilter("invalid", row, nil) {
		t.Error("invalid filter should not match")
	}
	if matchesFilter("status=invalid.value", row, nil) {
		t.Error("invalid operator should not match")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/realtime/... -v -run TestMatchesFilter`
Expected: FAIL (matchesFilter not exported or not complete)

**Step 3: Write the implementation**

```go
// internal/realtime/filter.go
package realtime

import (
	"fmt"
	"strconv"
	"strings"
)

// matchesFilter evaluates a PostgREST-style filter against row data
// filter format: "column=operator.value" (e.g., "user_id=eq.123")
func matchesFilter(filter string, newRow, oldRow map[string]any) bool {
	// Parse filter: column=operator.value
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return false
	}

	column := parts[0]
	opValue := parts[1]

	// Parse operator.value
	dotIdx := strings.Index(opValue, ".")
	if dotIdx == -1 {
		return false
	}

	operator := opValue[:dotIdx]
	value := opValue[dotIdx+1:]

	// Get row value (prefer new, fall back to old for DELETE)
	row := newRow
	if row == nil {
		row = oldRow
	}
	if row == nil {
		return false
	}

	rowValue, exists := row[column]
	if !exists {
		return false
	}

	return evaluateOperator(operator, rowValue, value)
}

// evaluateOperator evaluates a single operator comparison
func evaluateOperator(operator string, rowValue any, filterValue string) bool {
	switch operator {
	case "eq":
		return compareEqual(rowValue, filterValue)
	case "neq":
		return !compareEqual(rowValue, filterValue)
	case "gt":
		return compareNumeric(rowValue, filterValue) > 0
	case "gte":
		return compareNumeric(rowValue, filterValue) >= 0
	case "lt":
		return compareNumeric(rowValue, filterValue) < 0
	case "lte":
		return compareNumeric(rowValue, filterValue) <= 0
	case "in":
		return compareIn(rowValue, filterValue)
	default:
		return false
	}
}

// compareEqual checks if row value equals filter value
func compareEqual(rowValue any, filterValue string) bool {
	switch v := rowValue.(type) {
	case string:
		return v == filterValue
	case float64:
		fv, err := strconv.ParseFloat(filterValue, 64)
		if err != nil {
			return false
		}
		return v == fv
	case int64:
		iv, err := strconv.ParseInt(filterValue, 10, 64)
		if err != nil {
			return false
		}
		return v == iv
	case int:
		iv, err := strconv.Atoi(filterValue)
		if err != nil {
			return false
		}
		return v == iv
	case bool:
		return fmt.Sprintf("%v", v) == filterValue
	case nil:
		return filterValue == "null"
	default:
		return fmt.Sprintf("%v", v) == filterValue
	}
}

// compareNumeric compares row value to filter value numerically
// Returns: -1 if row < filter, 0 if equal, 1 if row > filter
func compareNumeric(rowValue any, filterValue string) int {
	var rowNum float64

	switch v := rowValue.(type) {
	case float64:
		rowNum = v
	case int64:
		rowNum = float64(v)
	case int:
		rowNum = float64(v)
	case string:
		var err error
		rowNum, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0 // Can't compare, treat as equal
		}
	default:
		return 0
	}

	filterNum, err := strconv.ParseFloat(filterValue, 64)
	if err != nil {
		return 0
	}

	if rowNum < filterNum {
		return -1
	} else if rowNum > filterNum {
		return 1
	}
	return 0
}

// compareIn checks if row value is in the filter value list
// filterValue format: "(val1,val2,val3)"
func compareIn(rowValue any, filterValue string) bool {
	// Remove parentheses
	filterValue = strings.TrimPrefix(filterValue, "(")
	filterValue = strings.TrimSuffix(filterValue, ")")

	values := strings.Split(filterValue, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if compareEqual(rowValue, v) {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/realtime/... -v -run TestMatchesFilter`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/realtime/filter.go internal/realtime/filter_test.go
git commit -m "feat(realtime): add filter evaluation for postgres_changes"
```

---

### Task 1.9: Integrate realtime with server

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Add realtime fields to Server struct**

In `internal/server/server.go`, add to the Server struct:

```go
// Realtime fields
realtimeService *realtime.Service
```

**Step 2: Add import**

```go
import (
	// ... existing imports
	"github.com/markb/sblite/internal/realtime"
)
```

**Step 3: Add EnableRealtime method**

```go
// EnableRealtime enables realtime WebSocket support
func (s *Server) EnableRealtime() {
	if s.realtimeService != nil {
		return
	}

	// Get API keys from dashboard store
	var anonKey, serviceKey string
	if s.dashboardStore != nil {
		anonKey, _ = s.dashboardStore.Get("anon_key")
		serviceKey, _ = s.dashboardStore.Get("service_role_key")
	}

	cfg := realtime.Config{
		JWTSecret:  s.jwtSecret,
		AnonKey:    anonKey,
		ServiceKey: serviceKey,
	}

	s.realtimeService = realtime.NewService(s.db.DB, s.rlsService, cfg)

	// Register WebSocket route
	s.router.Get("/realtime/v1/websocket", s.realtimeService.HandleWebSocket)

	log.Info("realtime enabled")
}

// RealtimeService returns the realtime service (may be nil)
func (s *Server) RealtimeService() *realtime.Service {
	return s.realtimeService
}
```

**Step 4: Run tests**

Run: `go test ./internal/server/... -v`
Expected: PASS (or existing tests still pass)

**Step 5: Commit**

```bash
git add internal/server/server.go
git commit -m "feat(server): add realtime service integration"
```

---

### Task 1.10: Add --realtime CLI flag

**Files:**
- Modify: `cmd/serve.go`

**Step 1: Add realtime flag**

In `cmd/serve.go`, add the flag:

```go
var realtimeEnabled bool

func init() {
	// ... existing flags
	serveCmd.Flags().BoolVar(&realtimeEnabled, "realtime", false, "Enable realtime WebSocket support")
}
```

**Step 2: Enable realtime in run function**

In the serve command's run function, after server creation:

```go
if realtimeEnabled {
	srv.EnableRealtime()
}
```

**Step 3: Test the flag**

Run: `go build -o sblite . && ./sblite serve --help`
Expected: Shows `--realtime` flag in help output

**Step 4: Commit**

```bash
git add cmd/serve.go
git commit -m "feat(cli): add --realtime flag to serve command"
```

---

## Phase 2: REST Handler Integration

### Task 2.1: Add NotifyChange calls to REST handlers

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/server/server.go`

**Step 1: Add RealtimeNotifier interface to rest package**

Create interface in `internal/rest/handler.go`:

```go
// RealtimeNotifier is called when data changes
type RealtimeNotifier interface {
	NotifyChange(schema, table, eventType string, oldRow, newRow map[string]any)
}
```

**Step 2: Add notifier field to Handler**

```go
type Handler struct {
	// ... existing fields
	notifier RealtimeNotifier
}

// SetRealtimeNotifier sets the notifier for change events
func (h *Handler) SetRealtimeNotifier(n RealtimeNotifier) {
	h.notifier = n
}
```

**Step 3: Add notification to HandleInsert**

After successful insert in `HandleInsert`:

```go
// Notify realtime subscribers
if h.notifier != nil && returnRepresentation && len(results) > 0 {
	for _, row := range results {
		h.notifier.NotifyChange("public", table, "INSERT", nil, row)
	}
}
```

**Step 4: Add notification to HandleUpdate**

In `HandleUpdate`, capture old rows before update, then notify after:

```go
// Before update, capture old rows for realtime
var oldRows []map[string]any
if h.notifier != nil && returnRepresentation {
	oldRows = h.selectMatchingWithRLS(q)
}

// ... existing update code ...

// Notify realtime subscribers
if h.notifier != nil && returnRepresentation {
	for i, newRow := range results {
		var oldRow map[string]any
		if i < len(oldRows) {
			oldRow = oldRows[i]
		}
		h.notifier.NotifyChange("public", q.Table, "UPDATE", oldRow, newRow)
	}
}
```

**Step 5: Add notification to HandleDelete**

In `HandleDelete`, after capturing deleted rows:

```go
// Notify realtime subscribers
if h.notifier != nil {
	for _, row := range deletedRows {
		h.notifier.NotifyChange("public", q.Table, "DELETE", row, nil)
	}
}
```

**Step 6: Wire up in server.go**

In `EnableRealtime()`:

```go
// Set notifier on REST handler
s.restHandler.SetRealtimeNotifier(s.realtimeService)
```

**Step 7: Run tests**

Run: `go test ./internal/rest/... -v`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/rest/handler.go internal/server/server.go
git commit -m "feat(rest): emit realtime notifications on data changes"
```

---

## Phase 3: Dashboard Integration

### Task 3.1: Add realtime stats endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add realtime service field**

```go
type Handler struct {
	// ... existing fields
	realtimeService interface {
		Stats() interface{}
	}
}

// SetRealtimeService sets the realtime service for stats
func (h *Handler) SetRealtimeService(svc interface{ Stats() interface{} }) {
	h.realtimeService = svc
}
```

**Step 2: Add stats handler**

```go
// handleRealtimeStats returns realtime connection statistics
func (h *Handler) handleRealtimeStats(w http.ResponseWriter, r *http.Request) {
	if h.realtimeService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "realtime not enabled",
		})
		return
	}

	stats := h.realtimeService.Stats()
	json.NewEncoder(w).Encode(stats)
}
```

**Step 3: Register route**

In `RegisterRoutes()`:

```go
r.Get("/api/realtime/stats", h.requireAuth(h.handleRealtimeStats))
```

**Step 4: Wire up in server.go**

In `EnableRealtime()`:

```go
// Set realtime service on dashboard handler
s.dashboardHandler.SetRealtimeService(s.realtimeService)
```

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/server/server.go
git commit -m "feat(dashboard): add realtime stats API endpoint"
```

---

## Phase 4: E2E Tests

### Task 4.1: Create E2E test infrastructure

**Files:**
- Create: `e2e/tests/realtime/helpers.ts`

**Step 1: Write WebSocket test helpers**

```typescript
// e2e/tests/realtime/helpers.ts
import WebSocket from 'ws'

const WS_URL = process.env.SBLITE_WS_URL || 'ws://localhost:8080/realtime/v1/websocket'
const ANON_KEY = process.env.SBLITE_ANON_KEY || 'your-anon-key'

export interface Message {
  event: string
  topic: string
  payload: Record<string, any>
  ref: string
  join_ref?: string
}

export class TestClient {
  private ws: WebSocket
  private messageQueue: Message[] = []
  private messageResolvers: Array<(msg: Message) => void> = []
  private refCounter = 0

  constructor() {
    this.ws = new WebSocket(`${WS_URL}?apikey=${ANON_KEY}`)
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws.on('open', () => resolve())
      this.ws.on('error', reject)
      this.ws.on('message', (data: Buffer) => {
        const msg = JSON.parse(data.toString()) as Message
        if (this.messageResolvers.length > 0) {
          const resolver = this.messageResolvers.shift()!
          resolver(msg)
        } else {
          this.messageQueue.push(msg)
        }
      })
    })
  }

  async close(): Promise<void> {
    this.ws.close()
  }

  send(msg: Omit<Message, 'ref'>): string {
    const ref = String(++this.refCounter)
    this.ws.send(JSON.stringify({ ...msg, ref }))
    return ref
  }

  async receive(timeout = 5000): Promise<Message> {
    if (this.messageQueue.length > 0) {
      return this.messageQueue.shift()!
    }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error('Timeout waiting for message'))
      }, timeout)
      this.messageResolvers.push((msg) => {
        clearTimeout(timer)
        resolve(msg)
      })
    })
  }

  async join(topic: string, config: Record<string, any> = {}): Promise<Message> {
    const ref = this.send({
      event: 'phx_join',
      topic,
      payload: { config },
      join_ref: String(this.refCounter),
    })
    return this.receive()
  }

  async leave(topic: string): Promise<Message> {
    this.send({
      event: 'phx_leave',
      topic,
      payload: {},
    })
    return this.receive()
  }

  async heartbeat(): Promise<Message> {
    this.send({
      event: 'heartbeat',
      topic: 'phoenix',
      payload: {},
    })
    return this.receive()
  }
}
```

**Step 2: Commit**

```bash
git add e2e/tests/realtime/helpers.ts
git commit -m "test(e2e): add realtime WebSocket test helpers"
```

---

### Task 4.2: Write connection tests

**Files:**
- Create: `e2e/tests/realtime/connection.test.ts`

**Step 1: Write connection tests**

```typescript
// e2e/tests/realtime/connection.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient } from './helpers'

describe('Realtime Connection', () => {
  let client: TestClient

  beforeEach(async () => {
    client = new TestClient()
    await client.connect()
  })

  afterEach(async () => {
    await client.close()
  })

  it('connects with valid API key', async () => {
    // Connection established in beforeEach
    const reply = await client.heartbeat()
    expect(reply.event).toBe('phx_reply')
    expect(reply.payload.status).toBe('ok')
  })

  it('responds to heartbeat', async () => {
    const reply = await client.heartbeat()
    expect(reply.event).toBe('phx_reply')
    expect(reply.topic).toBe('phoenix')
    expect(reply.payload.status).toBe('ok')
  })

  it('joins a channel', async () => {
    const reply = await client.join('realtime:test-channel')
    expect(reply.event).toBe('phx_reply')
    expect(reply.payload.status).toBe('ok')
  })

  it('leaves a channel', async () => {
    await client.join('realtime:test-channel')
    const reply = await client.leave('realtime:test-channel')
    expect(reply.event).toBe('phx_reply')
    expect(reply.payload.status).toBe('ok')
  })
})
```

**Step 2: Commit**

```bash
git add e2e/tests/realtime/connection.test.ts
git commit -m "test(e2e): add realtime connection tests"
```

---

### Task 4.3: Write broadcast tests

**Files:**
- Create: `e2e/tests/realtime/broadcast.test.ts`

**Step 1: Write broadcast tests**

```typescript
// e2e/tests/realtime/broadcast.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient } from './helpers'

describe('Realtime Broadcast', () => {
  let client1: TestClient
  let client2: TestClient

  beforeEach(async () => {
    client1 = new TestClient()
    client2 = new TestClient()
    await Promise.all([client1.connect(), client2.connect()])
  })

  afterEach(async () => {
    await Promise.all([client1.close(), client2.close()])
  })

  it('broadcasts message to other subscribers', async () => {
    await client1.join('realtime:broadcast-test', { broadcast: { self: false } })
    await client2.join('realtime:broadcast-test', { broadcast: { self: false } })

    // Client 1 sends broadcast
    client1.send({
      event: 'broadcast',
      topic: 'realtime:broadcast-test',
      payload: {
        type: 'broadcast',
        event: 'test-event',
        payload: { message: 'hello' },
      },
    })

    // Client 2 should receive it
    const msg = await client2.receive()
    expect(msg.event).toBe('broadcast')
    expect(msg.payload.event).toBe('test-event')
    expect(msg.payload.payload.message).toBe('hello')
  })

  it('does not receive own broadcast when self=false', async () => {
    await client1.join('realtime:broadcast-test', { broadcast: { self: false } })

    client1.send({
      event: 'broadcast',
      topic: 'realtime:broadcast-test',
      payload: {
        type: 'broadcast',
        event: 'test-event',
        payload: { message: 'hello' },
      },
    })

    // Should timeout - no message expected
    await expect(client1.receive(500)).rejects.toThrow('Timeout')
  })

  it('receives own broadcast when self=true', async () => {
    await client1.join('realtime:broadcast-test', { broadcast: { self: true } })

    client1.send({
      event: 'broadcast',
      topic: 'realtime:broadcast-test',
      payload: {
        type: 'broadcast',
        event: 'test-event',
        payload: { message: 'hello' },
      },
    })

    const msg = await client1.receive()
    expect(msg.event).toBe('broadcast')
    expect(msg.payload.payload.message).toBe('hello')
  })

  it('sends ack when ack=true', async () => {
    await client1.join('realtime:broadcast-test', { broadcast: { ack: true, self: false } })

    client1.send({
      event: 'broadcast',
      topic: 'realtime:broadcast-test',
      payload: {
        type: 'broadcast',
        event: 'test-event',
        payload: { message: 'hello' },
      },
    })

    const reply = await client1.receive()
    expect(reply.event).toBe('phx_reply')
    expect(reply.payload.status).toBe('ok')
  })
})
```

**Step 2: Commit**

```bash
git add e2e/tests/realtime/broadcast.test.ts
git commit -m "test(e2e): add realtime broadcast tests"
```

---

### Task 4.4: Write postgres_changes tests

**Files:**
- Create: `e2e/tests/realtime/postgres-changes.test.ts`

**Step 1: Write postgres_changes tests**

```typescript
// e2e/tests/realtime/postgres-changes.test.ts
import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient } from '@supabase/supabase-js'
import { TestClient } from './helpers'

const SUPABASE_URL = process.env.SBLITE_URL || 'http://localhost:8080'
const ANON_KEY = process.env.SBLITE_ANON_KEY || 'your-anon-key'

describe('Realtime Postgres Changes', () => {
  let wsClient: TestClient
  let supabase: ReturnType<typeof createClient>

  beforeAll(async () => {
    supabase = createClient(SUPABASE_URL, ANON_KEY)

    // Create test table
    await supabase.from('realtime_test').select().limit(1).catch(() => {
      // Table might not exist, that's ok
    })
  })

  beforeEach(async () => {
    wsClient = new TestClient()
    await wsClient.connect()
  })

  afterEach(async () => {
    await wsClient.close()
  })

  it('receives INSERT events', async () => {
    await wsClient.join('realtime:db-changes', {
      postgres_changes: [
        { event: 'INSERT', schema: 'public', table: 'realtime_test' }
      ]
    })

    // Skip system message
    await wsClient.receive()

    // Insert via REST API
    await supabase.from('realtime_test').insert({ name: 'test-insert' })

    // Should receive change event
    const msg = await wsClient.receive()
    expect(msg.event).toBe('postgres_changes')
    expect(msg.payload.data.eventType).toBe('INSERT')
    expect(msg.payload.data.new.name).toBe('test-insert')
  })

  it('receives UPDATE events with old and new', async () => {
    // Insert a row first
    const { data: inserted } = await supabase
      .from('realtime_test')
      .insert({ name: 'to-update' })
      .select()
      .single()

    await wsClient.join('realtime:db-changes', {
      postgres_changes: [
        { event: 'UPDATE', schema: 'public', table: 'realtime_test' }
      ]
    })
    await wsClient.receive() // system message

    // Update the row
    await supabase
      .from('realtime_test')
      .update({ name: 'updated' })
      .eq('id', inserted.id)

    const msg = await wsClient.receive()
    expect(msg.event).toBe('postgres_changes')
    expect(msg.payload.data.eventType).toBe('UPDATE')
    expect(msg.payload.data.old.name).toBe('to-update')
    expect(msg.payload.data.new.name).toBe('updated')
  })

  it('receives DELETE events', async () => {
    // Insert a row first
    const { data: inserted } = await supabase
      .from('realtime_test')
      .insert({ name: 'to-delete' })
      .select()
      .single()

    await wsClient.join('realtime:db-changes', {
      postgres_changes: [
        { event: 'DELETE', schema: 'public', table: 'realtime_test' }
      ]
    })
    await wsClient.receive() // system message

    // Delete the row
    await supabase.from('realtime_test').delete().eq('id', inserted.id)

    const msg = await wsClient.receive()
    expect(msg.event).toBe('postgres_changes')
    expect(msg.payload.data.eventType).toBe('DELETE')
    expect(msg.payload.data.old.name).toBe('to-delete')
  })

  it('filters by column value', async () => {
    await wsClient.join('realtime:db-changes', {
      postgres_changes: [
        { event: '*', schema: 'public', table: 'realtime_test', filter: 'name=eq.filtered' }
      ]
    })
    await wsClient.receive() // system message

    // Insert non-matching row
    await supabase.from('realtime_test').insert({ name: 'not-filtered' })

    // Insert matching row
    await supabase.from('realtime_test').insert({ name: 'filtered' })

    // Should only receive the filtered one
    const msg = await wsClient.receive()
    expect(msg.payload.data.new.name).toBe('filtered')
  })
})
```

**Step 2: Commit**

```bash
git add e2e/tests/realtime/postgres-changes.test.ts
git commit -m "test(e2e): add realtime postgres_changes tests"
```

---

### Task 4.5: Write presence tests

**Files:**
- Create: `e2e/tests/realtime/presence.test.ts`

**Step 1: Write presence tests**

```typescript
// e2e/tests/realtime/presence.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient } from './helpers'

describe('Realtime Presence', () => {
  let client1: TestClient
  let client2: TestClient

  beforeEach(async () => {
    client1 = new TestClient()
    client2 = new TestClient()
    await Promise.all([client1.connect(), client2.connect()])
  })

  afterEach(async () => {
    await Promise.all([client1.close(), client2.close()])
  })

  it('receives presence_state on join', async () => {
    await client1.join('realtime:presence-test', {
      presence: { key: 'user-1' }
    })

    // Track presence
    client1.send({
      event: 'presence',
      topic: 'realtime:presence-test',
      payload: {
        type: 'presence',
        event: 'track',
        payload: { status: 'online' }
      }
    })

    // Wait for diff
    await client1.receive()

    // Client 2 joins and should receive presence state
    const joinReply = await client2.join('realtime:presence-test', {
      presence: { key: 'user-2' }
    })

    const stateMsg = await client2.receive()
    expect(stateMsg.event).toBe('presence_state')
    expect(stateMsg.payload['user-1']).toBeDefined()
    expect(stateMsg.payload['user-1'][0].status).toBe('online')
  })

  it('broadcasts presence_diff on track', async () => {
    await client1.join('realtime:presence-test', { presence: { key: 'user-1' } })
    await client2.join('realtime:presence-test', { presence: { key: 'user-2' } })

    // Skip presence_state for client2
    await client2.receive()

    // Client 1 tracks presence
    client1.send({
      event: 'presence',
      topic: 'realtime:presence-test',
      payload: {
        type: 'presence',
        event: 'track',
        payload: { status: 'away' }
      }
    })

    // Client 2 should receive presence_diff
    const diffMsg = await client2.receive()
    expect(diffMsg.event).toBe('presence_diff')
    expect(diffMsg.payload.joins['user-1']).toBeDefined()
    expect(diffMsg.payload.joins['user-1'][0].status).toBe('away')
  })

  it('broadcasts presence_diff on leave', async () => {
    await client1.join('realtime:presence-test', { presence: { key: 'user-1' } })

    // Track presence
    client1.send({
      event: 'presence',
      topic: 'realtime:presence-test',
      payload: {
        type: 'presence',
        event: 'track',
        payload: { status: 'online' }
      }
    })
    await client1.receive() // diff

    await client2.join('realtime:presence-test', { presence: { key: 'user-2' } })
    await client2.receive() // presence_state

    // Client 1 leaves
    await client1.leave('realtime:presence-test')

    // Client 2 should receive presence_diff with leave
    const diffMsg = await client2.receive()
    expect(diffMsg.event).toBe('presence_diff')
    expect(diffMsg.payload.leaves['user-1']).toBeDefined()
  })
})
```

**Step 2: Commit**

```bash
git add e2e/tests/realtime/presence.test.ts
git commit -m "test(e2e): add realtime presence tests"
```

---

## Phase 5: Documentation

### Task 5.1: Create realtime documentation

**Files:**
- Create: `docs/realtime.md`

**Step 1: Write documentation**

```markdown
# sblite Realtime

sblite Realtime provides Supabase-compatible WebSocket functionality for real-time features in your applications.

## Features

- **Broadcast** - Send low-latency messages between clients
- **Presence** - Track and synchronize user online state
- **Postgres Changes** - Receive database change notifications

## Quick Start

### Enable Realtime

Start sblite with the `--realtime` flag:

```bash
./sblite serve --realtime
```

### Connect with supabase-js

```typescript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient('http://localhost:8080', 'your-anon-key')

// Subscribe to database changes
const channel = supabase
  .channel('db-changes')
  .on('postgres_changes',
    { event: '*', schema: 'public', table: 'messages' },
    (payload) => console.log('Change:', payload)
  )
  .subscribe()
```

## Broadcast

Send ephemeral messages between clients:

```typescript
const channel = supabase.channel('room:123', {
  config: {
    broadcast: { ack: true, self: false }
  }
})

// Listen for messages
channel.on('broadcast', { event: 'cursor' }, (payload) => {
  console.log('Cursor:', payload.x, payload.y)
})

// Subscribe and send
channel.subscribe((status) => {
  if (status === 'SUBSCRIBED') {
    channel.send({
      type: 'broadcast',
      event: 'cursor',
      payload: { x: 100, y: 200 }
    })
  }
})
```

### Broadcast Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ack` | boolean | false | Wait for server acknowledgment |
| `self` | boolean | false | Receive your own broadcasts |

## Presence

Track online users and their state:

```typescript
const channel = supabase.channel('room:123', {
  config: {
    presence: { key: 'user-123' }
  }
})

// Listen for presence changes
channel.on('presence', { event: 'sync' }, () => {
  const state = channel.presenceState()
  console.log('Online:', Object.keys(state))
})

channel.on('presence', { event: 'join' }, ({ key, newPresences }) => {
  console.log('Joined:', key)
})

channel.on('presence', { event: 'leave' }, ({ key, leftPresences }) => {
  console.log('Left:', key)
})

// Subscribe and track
channel.subscribe(async (status) => {
  if (status === 'SUBSCRIBED') {
    await channel.track({
      user_id: 'user-123',
      status: 'online'
    })
  }
})
```

## Postgres Changes

Listen to database changes:

```typescript
const channel = supabase
  .channel('table-changes')
  .on('postgres_changes',
    { event: 'INSERT', schema: 'public', table: 'messages' },
    (payload) => console.log('New message:', payload.new)
  )
  .on('postgres_changes',
    { event: 'UPDATE', schema: 'public', table: 'messages' },
    (payload) => console.log('Updated:', payload.old, '->', payload.new)
  )
  .on('postgres_changes',
    { event: 'DELETE', schema: 'public', table: 'messages' },
    (payload) => console.log('Deleted:', payload.old)
  )
  .subscribe()
```

### Filtering Changes

Filter by column values:

```typescript
channel.on('postgres_changes',
  {
    event: '*',
    schema: 'public',
    table: 'messages',
    filter: 'user_id=eq.123'  // Only changes where user_id = 123
  },
  (payload) => console.log(payload)
)
```

Supported filter operators: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`

### Event Types

| Event | Description |
|-------|-------------|
| `INSERT` | New row inserted |
| `UPDATE` | Existing row updated |
| `DELETE` | Row deleted |
| `*` | All events |

## Dashboard Monitoring

View realtime statistics in the dashboard at `/_/`:

- Active connections count
- Active channels count
- Per-channel subscriber counts

API endpoint: `GET /_/api/realtime/stats`

## WebSocket Protocol

sblite implements Phoenix Protocol v1.0.0 for compatibility with supabase-js.

Connection URL: `ws://localhost:8080/realtime/v1/websocket?apikey=<API_KEY>`

## Limitations

- Single-node only (no distributed presence sync)
- Change detection via REST handlers only (direct SQL changes not detected)
- No message persistence or replay

## Migration to Supabase

Code using sblite Realtime works with Supabase Realtime without changes. When migrating:

1. Update the Supabase URL and API key
2. Enable Realtime for your tables in Supabase dashboard
3. Your existing channel subscriptions will work automatically
```

**Step 2: Commit**

```bash
git add docs/realtime.md
git commit -m "docs: add realtime documentation"
```

---

### Task 5.2: Update README.md

**Files:**
- Modify: `README.md`

**Step 1: Add realtime to features list and documentation links**

Add to features section:
```markdown
- **Realtime** - WebSocket subscriptions for Broadcast, Presence, and Postgres Changes
```

Add to documentation section:
```markdown
- [Realtime](docs/realtime.md) - WebSocket real-time features
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add realtime to README"
```

---

### Task 5.3: Update SBLITE-TODO.md

**Files:**
- Modify: `docs/plans/SBLITE-TODO.md`

**Step 1: Mark realtime as complete**

Change the Realtime section status to ✅ COMPLETE and update the priority table.

**Step 2: Commit**

```bash
git add docs/plans/SBLITE-TODO.md
git commit -m "docs: mark realtime as complete in TODO"
```

---

## Final Verification

### Task 6.1: Run all tests

**Step 1: Run Go tests**

```bash
go test ./... -v
```
Expected: All tests pass

**Step 2: Run E2E tests**

```bash
cd e2e && npm test -- --grep realtime
```
Expected: All realtime tests pass

**Step 3: Manual verification**

1. Start server: `./sblite serve --realtime`
2. Open dashboard and verify Realtime section shows stats
3. Test with supabase-js client in browser

---

## Summary

This plan implements sblite Realtime in 5 phases with ~25 tasks:

1. **Core Infrastructure** (Tasks 1.1-1.10): Protocol, Hub, Channel, Conn, Presence, Filter, Service
2. **REST Integration** (Task 2.1): NotifyChange calls in handlers
3. **Dashboard** (Task 3.1): Stats endpoint
4. **E2E Tests** (Tasks 4.1-4.5): Connection, Broadcast, Postgres Changes, Presence tests
5. **Documentation** (Tasks 5.1-5.3): docs/realtime.md, README, TODO updates

Total: ~45 commits following TDD approach.
