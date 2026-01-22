// internal/realtime/hub.go
package realtime

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/log"
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

	// Remove from all channels and handle presence leave
	for topic, ch := range h.channels {
		ch.mu.Lock()
		sub, ok := ch.subscribers[conn.id]
		if ok {
			// Handle presence leave if this subscriber had presence
			if presence := ch.presence; presence != nil && sub.presenceConfig.Key != "" {
				leaves := presence.UntrackConn(conn.id)
				if len(leaves) > 0 {
					// Broadcast presence_diff to remaining subscribers
					for key, metas := range leaves {
						diff := NewPresenceDiffMessage(topic, sub.joinRef,
							map[string][]map[string]any{},
							map[string][]map[string]any{key: metas})
						// Send to all remaining subscribers
						for _, otherSub := range ch.subscribers {
							if otherSub.conn.id != conn.id {
								otherSub.conn.Send(diff)
							}
						}
					}
				}
			}

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

	log.Debug("realtime: broadcastChange", "schema", schema, "table", table, "eventType", eventType)

	h.mu.RLock()
	defer h.mu.RUnlock()

	log.Debug("realtime: checking channels", "count", len(h.channels))

	for _, ch := range h.channels {
		subs := ch.getSubscribers()
		for _, sub := range subs {
			log.Debug("realtime: checking subscriber", "topic", ch.topic, "pgChanges", len(sub.pgChanges))
			matchingIDs := h.getMatchingSubscriptionIDs(sub.pgChanges, event)
			if len(matchingIDs) > 0 {
				// Check RLS access for this subscriber
				// For INSERT/UPDATE, check against the new row
				// For DELETE, check against the old row
				rowToCheck := newRow
				if eventType == "DELETE" {
					rowToCheck = oldRow
				}

				// Only send if subscriber passes RLS check
				if h.checkRLSAccess(table, rowToCheck, sub.conn.claims) {
					msg := NewPostgresChangeMessage(ch.topic, sub.joinRef, matchingIDs, event)
					sub.conn.Send(msg)
					log.Debug("realtime: sent postgres_changes", "topic", ch.topic)
				} else {
					log.Debug("realtime: RLS check failed", "topic", ch.topic)
				}
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

// checkRLSAccess checks if a subscriber can access a row based on RLS policies
func (h *Hub) checkRLSAccess(tableName string, row map[string]any, claims jwt.MapClaims) bool {
	if h.rlsService == nil {
		return true // No RLS service, allow all
	}

	// Check if RLS is enabled for this table
	enabled, err := h.rlsService.IsRLSEnabled(tableName)
	if err != nil {
		log.Error("realtime: failed to check RLS status", "table", tableName, "error", err.Error())
		return false // Fail closed
	}
	if !enabled {
		return true // RLS not enabled, allow all
	}

	// Get SELECT policies for the table
	policies, err := h.rlsService.GetPoliciesForTable(tableName)
	if err != nil {
		log.Error("realtime: failed to get RLS policies", "table", tableName, "error", err.Error())
		return false // Fail closed
	}

	if len(policies) == 0 {
		return true // No policies, allow all
	}

	// Build auth context from JWT claims
	ctx := buildAuthContext(claims)
	if ctx.BypassRLS {
		return true // service_role bypasses RLS
	}

	// Collect SELECT policy conditions
	var conditions []string
	for _, p := range policies {
		if p.Command == "SELECT" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := rls.SubstituteAuthFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return true // No SELECT policies
	}

	// Build a test query using the row data
	return h.evaluateRLSCondition(strings.Join(conditions, " AND "), row)
}

// buildAuthContext creates an RLS AuthContext from JWT claims
func buildAuthContext(claims jwt.MapClaims) *rls.AuthContext {
	ctx := &rls.AuthContext{
		Claims: make(map[string]any),
	}

	if claims == nil {
		return ctx
	}

	// Extract standard claims
	if sub, ok := claims["sub"].(string); ok {
		ctx.UserID = sub
	}
	if email, ok := claims["email"].(string); ok {
		ctx.Email = email
	}
	if role, ok := claims["role"].(string); ok {
		ctx.Role = role
		// service_role bypasses RLS
		if role == "service_role" {
			ctx.BypassRLS = true
		}
	}

	// Copy all claims for auth.jwt()->>'key' access
	for k, v := range claims {
		ctx.Claims[k] = v
	}

	return ctx
}

// evaluateRLSCondition evaluates an RLS condition against a row
func (h *Hub) evaluateRLSCondition(condition string, row map[string]any) bool {
	if h.db == nil || len(row) == 0 {
		return false
	}

	// Build column list and values for the test query
	var columns []string
	var placeholders []string
	var values []any

	for col, val := range row {
		columns = append(columns, col)
		placeholders = append(placeholders, "? AS "+col)
		values = append(values, val)
	}

	// Build a query that tests if the row satisfies the condition
	// SELECT 1 FROM (SELECT val1 AS col1, val2 AS col2, ...) WHERE condition
	query := fmt.Sprintf(
		"SELECT 1 FROM (SELECT %s) WHERE %s",
		strings.Join(placeholders, ", "),
		condition,
	)

	// Execute the query
	rows, err := h.db.Query(query, values...)
	if err != nil {
		log.Debug("realtime: RLS condition evaluation failed", "error", err.Error(), "condition", condition)
		return false // Fail closed on query errors
	}
	defer rows.Close()

	// If we get any rows, the condition is satisfied
	return rows.Next()
}

