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
