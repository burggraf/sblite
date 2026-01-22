// Package realtime implements Supabase Realtime protocol support.
// It provides Phoenix Protocol v1.0.0 compatible WebSocket messaging
// for broadcast, presence, and postgres_changes features.
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
	Broadcast       BroadcastConfig     `json:"broadcast"`
	Presence        PresenceConfig      `json:"presence"`
	PostgresChanges []PostgresChangeSub `json:"postgres_changes"`
	Private         bool                `json:"private"`
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
func NewPresenceStateMessage(topic, joinRef string, state map[string]any) *Message {
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
