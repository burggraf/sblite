package dashboard

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

func TestStoreSetAndGet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Set a value
	err := store.Set("test_key", "test_value")
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// Get the value
	value, err := store.Get("test_key")
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if value != "test_value" {
		t.Errorf("expected 'test_value', got '%s'", value)
	}
}

func TestStoreGetMissing(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Get non-existent key
	value, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "" {
		t.Errorf("expected empty string for missing key, got '%s'", value)
	}
}

func TestStoreHasPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Initially no password
	if store.HasPassword() {
		t.Error("expected no password initially")
	}

	// Set password hash
	err := store.Set("password_hash", "$2a$10$somehash")
	if err != nil {
		t.Fatalf("failed to set password_hash: %v", err)
	}

	// Now should have password
	if !store.HasPassword() {
		t.Error("expected password to exist")
	}
}
