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

func TestHubRegisterConn(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	conn := &Conn{id: "conn-1"}
	hub.registerConn(conn)

	stats := hub.Stats()
	if stats.Connections != 1 {
		t.Errorf("expected 1 connection, got %d", stats.Connections)
	}

	// Register another connection
	conn2 := &Conn{id: "conn-2"}
	hub.registerConn(conn2)

	stats = hub.Stats()
	if stats.Connections != 2 {
		t.Errorf("expected 2 connections, got %d", stats.Connections)
	}
}

func TestHubUnregisterConn(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	conn := &Conn{id: "conn-1"}
	hub.registerConn(conn)

	// Add connection to a channel
	ch := hub.getOrCreateChannel("realtime:test", false)
	ch.subscribers[conn.id] = &ChannelSub{conn: conn, joinRef: "1"}

	stats := hub.Stats()
	if stats.Connections != 1 {
		t.Errorf("expected 1 connection, got %d", stats.Connections)
	}
	if stats.Channels != 1 {
		t.Errorf("expected 1 channel, got %d", stats.Channels)
	}

	// Unregister connection - should remove from hub and channel
	hub.unregisterConn(conn)

	stats = hub.Stats()
	if stats.Connections != 0 {
		t.Errorf("expected 0 connections after unregister, got %d", stats.Connections)
	}
	// Empty channel should be cleaned up
	if stats.Channels != 0 {
		t.Errorf("expected 0 channels after unregister (empty channel cleanup), got %d", stats.Channels)
	}
}

func TestHubGetChannel(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	// Get non-existent channel
	ch := hub.getChannel("realtime:nonexistent")
	if ch != nil {
		t.Error("expected nil for non-existent channel")
	}

	// Create channel
	created := hub.getOrCreateChannel("realtime:test", false)

	// Get existing channel
	ch = hub.getChannel("realtime:test")
	if ch != created {
		t.Error("expected to get the same channel instance")
	}
}

func TestHubRemoveChannelIfEmpty(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	// Create a channel with no subscribers
	hub.getOrCreateChannel("realtime:empty", false)

	stats := hub.Stats()
	if stats.Channels != 1 {
		t.Errorf("expected 1 channel, got %d", stats.Channels)
	}

	// Remove empty channel
	hub.removeChannelIfEmpty("realtime:empty")

	stats = hub.Stats()
	if stats.Channels != 0 {
		t.Errorf("expected 0 channels after removal, got %d", stats.Channels)
	}
}

func TestHubRemoveChannelIfEmptyWithSubscribers(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	// Create a channel with subscribers
	ch := hub.getOrCreateChannel("realtime:active", false)
	conn := &Conn{id: "conn-1"}
	ch.subscribers[conn.id] = &ChannelSub{conn: conn, joinRef: "1"}

	// Try to remove non-empty channel
	hub.removeChannelIfEmpty("realtime:active")

	// Channel should still exist
	stats := hub.Stats()
	if stats.Channels != 1 {
		t.Errorf("expected 1 channel (non-empty should not be removed), got %d", stats.Channels)
	}
}

func TestHubStatsChannelDetails(t *testing.T) {
	hub := NewHub(nil, nil, "test-secret")

	// Create channels
	ch1 := hub.getOrCreateChannel("realtime:ch1", false)
	_ = hub.getOrCreateChannel("realtime:ch2", true) // ch2 has no subscribers

	// Add subscribers to ch1
	conn := &Conn{id: "conn-1"}
	ch1.subscribers[conn.id] = &ChannelSub{conn: conn, joinRef: "1"}

	stats := hub.Stats()
	if len(stats.ChannelDetails) != 2 {
		t.Fatalf("expected 2 channel details, got %d", len(stats.ChannelDetails))
	}

	// Find ch1 stats
	var ch1Stats *ChannelStats
	for i, cs := range stats.ChannelDetails {
		if cs.Topic == "realtime:ch1" {
			ch1Stats = &stats.ChannelDetails[i]
			break
		}
	}

	if ch1Stats == nil {
		t.Fatal("expected to find ch1 in channel details")
	}
	if ch1Stats.Subscribers != 1 {
		t.Errorf("expected 1 subscriber for ch1, got %d", ch1Stats.Subscribers)
	}

	// Verify ch2 has no subscribers
	var ch2Stats *ChannelStats
	for i, cs := range stats.ChannelDetails {
		if cs.Topic == "realtime:ch2" {
			ch2Stats = &stats.ChannelDetails[i]
			break
		}
	}

	if ch2Stats == nil {
		t.Fatal("expected to find ch2 in channel details")
	}
	if ch2Stats.Subscribers != 0 {
		t.Errorf("expected 0 subscribers for ch2, got %d", ch2Stats.Subscribers)
	}
}

func TestBuildAuthContext(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]any
		expectUID string
		expectRole string
		expectBypass bool
	}{
		{
			name:   "nil claims",
			claims: nil,
			expectUID: "",
			expectRole: "",
			expectBypass: false,
		},
		{
			name: "authenticated user",
			claims: map[string]any{
				"sub":   "user-123",
				"email": "test@example.com",
				"role":  "authenticated",
			},
			expectUID: "user-123",
			expectRole: "authenticated",
			expectBypass: false,
		},
		{
			name: "service_role bypasses RLS",
			claims: map[string]any{
				"sub":  "service",
				"role": "service_role",
			},
			expectUID: "service",
			expectRole: "service_role",
			expectBypass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildAuthContext(tt.claims)
			if ctx.UserID != tt.expectUID {
				t.Errorf("UserID: expected %q, got %q", tt.expectUID, ctx.UserID)
			}
			if ctx.Role != tt.expectRole {
				t.Errorf("Role: expected %q, got %q", tt.expectRole, ctx.Role)
			}
			if ctx.BypassRLS != tt.expectBypass {
				t.Errorf("BypassRLS: expected %v, got %v", tt.expectBypass, ctx.BypassRLS)
			}
		})
	}
}

func TestCheckRLSAccessNoRLSService(t *testing.T) {
	// Hub without RLS service should allow all
	hub := NewHub(nil, nil, "test-secret")

	allowed := hub.checkRLSAccess("test_table", map[string]any{"id": 1}, nil)
	if !allowed {
		t.Error("expected access to be allowed when no RLS service")
	}
}
