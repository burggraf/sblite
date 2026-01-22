// internal/realtime/conn.go
package realtime

import (
	"fmt"
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
	id        string
	ws        *websocket.Conn
	hub       *Hub
	mu        sync.Mutex
	channels  map[string]*ChannelSub // topic -> subscription
	claims    jwt.MapClaims          // parsed from access_token
	send      chan []byte            // outbound message queue
	done      chan struct{}          // closed when connection ends
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
		if c.hub != nil {
			c.hub.unregisterConn(c)
		}
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
			n := len(data)
			if n > 100 {
				n = 100
			}
			log.Debug("realtime: invalid message", "conn_id", c.id, "error", err.Error(), "raw_hex", fmt.Sprintf("%x", data[:n]), "raw_str", string(data), "len", len(data))
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
	log.Debug("realtime: handleMessage", "conn_id", c.id, "event", msg.Event, "topic", msg.Topic)

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
		log.Debug("realtime: unknown event", "conn_id", c.id, "event", msg.Event, "topic", msg.Topic)
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
	log.Debug("realtime: handleBroadcast", "conn_id", c.id, "topic", msg.Topic, "event", msg.Event, "payload", msg.Payload)

	c.mu.Lock()
	sub, ok := c.channels[msg.Topic]
	c.mu.Unlock()

	if !ok {
		log.Debug("realtime: handleBroadcast - not subscribed to channel", "topic", msg.Topic)
		return
	}

	ch := c.hub.getChannel(msg.Topic)
	if ch == nil {
		log.Debug("realtime: handleBroadcast - channel not found", "topic", msg.Topic)
		return
	}

	// Extract broadcast payload
	payload, _ := msg.Payload["payload"].(map[string]any)
	event, _ := msg.Payload["event"].(string)

	log.Debug("realtime: handleBroadcast - broadcasting", "event", event, "numSubscribers", len(ch.getSubscribers()))

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

	// Check for event type in both "event" and "type" fields
	eventType, _ := msg.Payload["event"].(string)
	if eventType == "" {
		eventType, _ = msg.Payload["type"].(string)
	}
	payload, _ := msg.Payload["payload"].(map[string]any)

	switch eventType {
	case "track":
		meta := presence.Track(sub.presenceConfig.Key, c.id, payload)
		joins := map[string][]map[string]any{sub.presenceConfig.Key: {meta}}
		diff := NewPresenceDiffMessage(msg.Topic, sub.joinRef, joins, map[string][]map[string]any{})
		c.broadcastToChannel(ch, diff, "")
	case "untrack":
		leaves := presence.Untrack(sub.presenceConfig.Key, c.id)
		if len(leaves) > 0 {
			diff := NewPresenceDiffMessage(msg.Topic, sub.joinRef,
				map[string][]map[string]any{},
				map[string][]map[string]any{sub.presenceConfig.Key: leaves})
			c.broadcastToChannel(ch, diff, "")
		}
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
