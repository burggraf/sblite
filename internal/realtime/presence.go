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
