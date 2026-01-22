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
