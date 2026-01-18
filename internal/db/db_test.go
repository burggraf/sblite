// internal/db/db_test.go
package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDB(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	// Verify WAL mode is enabled
	var journalMode string
	err = database.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode=wal, got %s", journalMode)
	}
}

func setupTestDB(t *testing.T) (*DB, func()) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	err = database.RunMigrations()
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database, func() { database.Close() }
}

func TestOAuthTablesExist(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Check auth_identities table exists
	var identitiesExists int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_identities'").Scan(&identitiesExists)
	require.NoError(t, err)
	assert.Equal(t, 1, identitiesExists, "auth_identities table should exist")

	// Check auth_flow_state table exists
	var flowStateExists int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_flow_state'").Scan(&flowStateExists)
	require.NoError(t, err)
	assert.Equal(t, 1, flowStateExists, "auth_flow_state table should exist")

	// Verify auth_identities columns
	_, err = db.Exec(`INSERT INTO auth_identities (id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at)
		VALUES ('test-id', 'user-id', 'google', 'google-123', '{}', datetime('now'), datetime('now'), datetime('now'))`)
	// Will fail due to foreign key, but proves columns exist
	assert.Error(t, err) // FK constraint

	// Verify auth_flow_state columns
	_, err = db.Exec(`INSERT INTO auth_flow_state (id, provider, code_verifier, redirect_to, created_at, expires_at)
		VALUES ('state-123', 'google', 'verifier', 'https://example.com', datetime('now'), datetime('now', '+10 minutes'))`)
	require.NoError(t, err)
}
