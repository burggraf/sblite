package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	require.NoError(t, err)
	err = database.RunMigrations()
	require.NoError(t, err)
	return database, func() { database.Close() }
}

func TestStoreAndRetrieveFlowState(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database.DB)

	state := &FlowState{
		ID:           "test-state-123",
		Provider:     "google",
		CodeVerifier: "test-verifier-abc",
		RedirectTo:   "https://example.com/callback",
	}

	err := store.Save(state)
	require.NoError(t, err)

	retrieved, err := store.Get("test-state-123")
	require.NoError(t, err)
	assert.Equal(t, state.Provider, retrieved.Provider)
	assert.Equal(t, state.CodeVerifier, retrieved.CodeVerifier)
	assert.Equal(t, state.RedirectTo, retrieved.RedirectTo)
}

func TestFlowStateExpiry(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database.DB)

	// Create expired state by manipulating time
	state := &FlowState{
		ID:           "expired-state",
		Provider:     "google",
		CodeVerifier: "verifier",
		RedirectTo:   "https://example.com",
	}
	err := store.Save(state)
	require.NoError(t, err)

	// Manually expire it
	_, err = database.Exec("UPDATE auth_flow_state SET expires_at = datetime('now', '-1 minute') WHERE id = ?", state.ID)
	require.NoError(t, err)

	// Should not be retrievable
	_, err = store.Get("expired-state")
	assert.Error(t, err)
}

func TestDeleteFlowState(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database.DB)

	state := &FlowState{
		ID:           "to-delete",
		Provider:     "github",
		CodeVerifier: "verifier",
	}
	err := store.Save(state)
	require.NoError(t, err)

	err = store.Delete("to-delete")
	require.NoError(t, err)

	_, err = store.Get("to-delete")
	assert.Error(t, err)
}

func TestCleanupExpiredStates(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database.DB)

	// Create two states
	state1 := &FlowState{
		ID:           "state-1",
		Provider:     "google",
		CodeVerifier: "verifier-1",
	}
	state2 := &FlowState{
		ID:           "state-2",
		Provider:     "github",
		CodeVerifier: "verifier-2",
	}
	err := store.Save(state1)
	require.NoError(t, err)
	err = store.Save(state2)
	require.NoError(t, err)

	// Expire state1
	_, err = database.Exec("UPDATE auth_flow_state SET expires_at = datetime('now', '-1 minute') WHERE id = ?", state1.ID)
	require.NoError(t, err)

	// Cleanup expired states
	err = store.CleanupExpired()
	require.NoError(t, err)

	// state1 should be deleted
	_, err = store.Get("state-1")
	assert.Error(t, err)

	// state2 should still exist
	retrieved, err := store.Get("state-2")
	require.NoError(t, err)
	assert.Equal(t, "github", retrieved.Provider)
}

func TestGetNonExistentState(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database.DB)

	_, err := store.Get("non-existent")
	assert.ErrorIs(t, err, ErrStateNotFound)
}
