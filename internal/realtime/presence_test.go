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

func TestPresenceUntrackConn(t *testing.T) {
	ps := NewPresenceState()

	// Single user tracks with multiple keys
	ps.Track("user-1", "conn-1", map[string]any{"status": "online"})
	ps.Track("room-1", "conn-1", map[string]any{"role": "admin"})

	// Different connection for same user
	ps.Track("user-1", "conn-2", map[string]any{"status": "away"})

	// Untrack all presences for conn-1
	leaves := ps.UntrackConn("conn-1")

	// Should have leaves from both keys
	if len(leaves) != 2 {
		t.Errorf("expected leaves from 2 keys, got %d", len(leaves))
	}

	state := ps.GetState()
	// user-1 should still have conn-2 presence
	if len(state["user-1"]) != 1 {
		t.Errorf("expected 1 presence for user-1 after untrack conn-1, got %d", len(state["user-1"]))
	}
	// room-1 should have no presence
	if len(state["room-1"]) != 0 {
		t.Errorf("expected 0 presences for room-1 after untrack, got %d", len(state["room-1"]))
	}
}

func TestPresenceUpdateExisting(t *testing.T) {
	ps := NewPresenceState()

	// Track initial presence
	meta1 := ps.Track("user-1", "conn-1", map[string]any{"status": "online"})
	ref1 := meta1["phx_ref"]

	// Update presence for same key/conn
	meta2 := ps.Track("user-1", "conn-1", map[string]any{"status": "away"})
	ref2 := meta2["phx_ref"]

	// Should have new phx_ref
	if ref1 == ref2 {
		t.Error("phx_ref should change on update")
	}

	// Should only have 1 presence entry
	state := ps.GetState()
	if len(state["user-1"]) != 1 {
		t.Errorf("expected 1 presence after update, got %d", len(state["user-1"]))
	}

	// Status should be updated
	if state["user-1"][0]["status"] != "away" {
		t.Errorf("expected status 'away', got %v", state["user-1"][0]["status"])
	}
}

func TestPresenceGetStateSnapshot(t *testing.T) {
	ps := NewPresenceState()

	ps.Track("user-1", "conn-1", map[string]any{"status": "online"})
	ps.Track("user-2", "conn-2", map[string]any{"status": "away"})

	state := ps.GetState()

	// Verify snapshot contains both users
	if len(state) != 2 {
		t.Errorf("expected 2 keys in state, got %d", len(state))
	}

	// Modify snapshot shouldn't affect original
	delete(state, "user-1")

	state2 := ps.GetState()
	if len(state2) != 2 {
		t.Errorf("expected 2 keys in state after modification, got %d (snapshot should not affect original)", len(state2))
	}
}

func TestPresencePhxRefIsUnique(t *testing.T) {
	ps := NewPresenceState()

	meta1 := ps.Track("user-1", "conn-1", map[string]any{"status": "online"})
	meta2 := ps.Track("user-2", "conn-2", map[string]any{"status": "online"})

	if meta1["phx_ref"] == meta2["phx_ref"] {
		t.Error("phx_ref should be unique across different presences")
	}
}
