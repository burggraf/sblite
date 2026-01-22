// internal/realtime/hub.go
package realtime

import (
	"database/sql"
	"sync"

	"github.com/markb/sblite/internal/rls"
)

// PresenceState tracks presence for a channel (stub for now)
type PresenceState struct {
	// Will be fully implemented in Task 1.6
	mu    sync.RWMutex
	state map[string][]map[string]any // key -> list of presence metas
}

// NewPresenceState creates a new PresenceState
func NewPresenceState() *PresenceState {
	return &PresenceState{
		state: make(map[string][]map[string]any),
	}
}

// GetState returns the current presence state
func (p *PresenceState) GetState() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]any)
	for k, v := range p.state {
		result[k] = v
	}
	return result
}

// Track adds a presence for a key
func (p *PresenceState) Track(key, connID string, meta map[string]any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	presenceMeta := make(map[string]any)
	for k, v := range meta {
		presenceMeta[k] = v
	}
	presenceMeta["phx_ref"] = connID

	p.state[key] = append(p.state[key], presenceMeta)
	return presenceMeta
}

// Untrack removes a presence for a key and connID
func (p *PresenceState) Untrack(key, connID string) []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	metas, ok := p.state[key]
	if !ok {
		return nil
	}

	var removed []map[string]any
	var remaining []map[string]any
	for _, m := range metas {
		if ref, _ := m["phx_ref"].(string); ref == connID {
			removed = append(removed, m)
		} else {
			remaining = append(remaining, m)
		}
	}

	if len(remaining) == 0 {
		delete(p.state, key)
	} else {
		p.state[key] = remaining
	}

	return removed
}

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
