// internal/db/migrations_test.go
package db

import (
	"testing"
)

func TestRunMigrations(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify auth_users table exists
	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='auth_users'").Scan(&tableName)
	if err != nil {
		t.Fatalf("auth_users table not found: %v", err)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	// Run twice - should not error
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
}
