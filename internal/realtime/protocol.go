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
// Format matches Supabase's Realtime protocol specification
func NewPostgresChangeMessage(topic, joinRef string, ids []int, event ChangeEvent) *Message {
	return &Message{
		Event:   EventPostgres,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: map[string]any{
			"ids": ids,
			"data": map[string]any{
				"schema":           event.Schema,
				"table":            event.Table,
				"commit_timestamp": event.CommitTimestamp,
				"type":             event.EventType,
				"columns":          []map[string]any{}, // Empty columns array
				"record":           event.New,
				"old_record":       event.Old,
				"errors":           nil,
			},
		},
	}
}

// NewPresenceStateMessage creates a presence_state message
func NewPresenceStateMessage(topic, joinRef string, state map[string][]map[string]any) *Message {
	// Convert to generic map for JSON serialization
	payload := make(map[string]any)
	for k, v := range state {
		payload[k] = v
	}
	return &Message{
		Event:   EventPresenceState,
		Topic:   topic,
		JoinRef: joinRef,
		Payload: payload,
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

// Encode serializes a message to JSON bytes in Phoenix array format
// [join_ref, ref, topic, event, payload]
func (m *Message) Encode() ([]byte, error) {
	arr := []any{m.JoinRef, m.Ref, m.Topic, m.Event, m.Payload}
	return json.Marshal(arr)
}

// DecodeMessage parses JSON bytes into a Message
// Supports:
// - Phoenix array format [join_ref, ref, topic, event, payload]
// - Object format {event, topic, payload, ref, join_ref}
// - Phoenix v2 binary format (push/broadcast)
func DecodeMessage(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	// Check first byte to detect format
	firstByte := data[0]

	// Phoenix v2 binary format: first byte is message kind
	// 0 = push, 1 = reply, 2 = broadcast, 3 = push (alternate?)
	// If first byte is small and message doesn't start with '[' or '{', try binary
	if firstByte <= 10 && firstByte != '[' && firstByte != '{' {
		return decodeBinaryMessage(data)
	}

	// Skip whitespace for JSON detection
	for i := 0; i < len(data); i++ {
		if data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r' {
			continue
		}
		if data[i] == '[' {
			// Array format: [join_ref, ref, topic, event, payload]
			return decodeArrayMessage(data)
		}
		break
	}

	// Object format
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("invalid message format: %w", err)
	}
	return &msg, nil
}

// decodeBinaryMessage parses Phoenix v2 binary format
// Format: [kind:1][join_ref_size:1][ref_size:1][topic_size:1][event_size:1][...variable...][topic][event][payload]
// The variable portion contains join_ref and ref as length-prefixed strings
func decodeBinaryMessage(data []byte) (*Message, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("binary message too short: need at least 5 bytes for header")
	}

	// Parse header - sizes tell us the LENGTH of topic and event strings
	// kind := data[0]  // 0=push, 1=reply, 2=broadcast
	topicSize := int(data[3])
	eventSize := int(data[4])

	// Find JSON payload start by searching for '{'
	jsonStart := -1
	for i := 5; i < len(data); i++ {
		if data[i] == '{' {
			jsonStart = i
			break
		}
	}

	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON payload found in binary message")
	}

	// Work backwards from JSON start to extract event and topic
	// Event is the last eventSize bytes before JSON
	// Topic is the topicSize bytes before that
	eventStart := jsonStart - eventSize
	topicStart := eventStart - topicSize

	if topicStart < 5 {
		return nil, fmt.Errorf("invalid binary message: topic would start before header")
	}

	topic := string(data[topicStart : topicStart+topicSize])
	event := string(data[eventStart : eventStart+eventSize])

	// Extract join_ref and ref from the space between header and topic
	// This area contains length-prefixed strings
	middleSection := data[5:topicStart]
	joinRef, ref := parseRefsFromMiddle(middleSection)

	msg := &Message{
		JoinRef: joinRef,
		Ref:     ref,
		Topic:   topic,
		Event:   event,
	}

	// Parse JSON payload
	if err := json.Unmarshal(data[jsonStart:], &msg.Payload); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}

	// For broadcast events, wrap the payload with event info
	// This matches what handleBroadcast expects
	if msg.Event != EventJoin && msg.Event != EventLeave && msg.Event != EventHeartbeat &&
		msg.Event != EventAccessToken && msg.Event != EventPresence {
		// This is a user-defined broadcast event (like "message")
		originalPayload := msg.Payload
		msg.Payload = map[string]any{
			"event":   msg.Event,
			"payload": originalPayload,
			"type":    "broadcast",
		}
		msg.Event = EventBroadcast
	}

	return msg, nil
}

// parseRefsFromMiddle extracts join_ref and ref from the middle section
// The format appears to be length-prefixed strings
func parseRefsFromMiddle(data []byte) (joinRef, ref string) {
	if len(data) == 0 {
		return "", ""
	}

	pos := 0

	// First length-prefixed string is join_ref
	if pos < len(data) {
		length := int(data[pos])
		pos++
		if pos+length <= len(data) {
			joinRef = string(data[pos : pos+length])
			pos += length
		}
	}

	// Second length-prefixed string is ref
	if pos < len(data) {
		length := int(data[pos])
		pos++
		if pos+length <= len(data) {
			ref = string(data[pos : pos+length])
		}
	}

	return joinRef, ref
}

// decodeArrayMessage parses Phoenix array format [join_ref, ref, topic, event, payload]
func decodeArrayMessage(data []byte) (*Message, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, fmt.Errorf("invalid array message: %w", err)
	}

	if len(arr) < 5 {
		return nil, fmt.Errorf("array message must have 5 elements, got %d", len(arr))
	}

	msg := &Message{}

	// Parse join_ref (index 0)
	var joinRef string
	if err := json.Unmarshal(arr[0], &joinRef); err != nil {
		// Try as null
		var nullVal any
		json.Unmarshal(arr[0], &nullVal)
		if nullVal != nil {
			return nil, fmt.Errorf("invalid join_ref: %w", err)
		}
	}
	msg.JoinRef = joinRef

	// Parse ref (index 1)
	var ref string
	if err := json.Unmarshal(arr[1], &ref); err != nil {
		var nullVal any
		json.Unmarshal(arr[1], &nullVal)
		if nullVal != nil {
			return nil, fmt.Errorf("invalid ref: %w", err)
		}
	}
	msg.Ref = ref

	// Parse topic (index 2)
	if err := json.Unmarshal(arr[2], &msg.Topic); err != nil {
		return nil, fmt.Errorf("invalid topic: %w", err)
	}

	// Parse event (index 3)
	if err := json.Unmarshal(arr[3], &msg.Event); err != nil {
		return nil, fmt.Errorf("invalid event: %w", err)
	}

	// Parse payload (index 4)
	if err := json.Unmarshal(arr[4], &msg.Payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	return msg, nil
}
