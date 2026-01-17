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

func TestEmailTablesMigration(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Test auth_emails table exists
	_, err = database.Exec(`INSERT INTO auth_emails (id, to_email, from_email, subject, email_type, created_at)
		VALUES ('test-id', 'to@test.com', 'from@test.com', 'Test', 'confirmation', datetime('now'))`)
	if err != nil {
		t.Fatalf("auth_emails table should exist: %v", err)
	}

	// Test auth_email_templates table exists
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM auth_email_templates").Scan(&count)
	if err != nil {
		t.Fatalf("auth_email_templates table should exist: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 default templates, got %d", count)
	}

	// Test auth_verification_tokens table exists
	_, err = database.Exec(`INSERT INTO auth_verification_tokens (id, user_id, type, email, expires_at, created_at)
		VALUES ('token-id', 'user-id', 'confirmation', 'test@test.com', datetime('now', '+1 day'), datetime('now'))`)
	if err != nil {
		t.Fatalf("auth_verification_tokens table should exist: %v", err)
	}
}

func TestColumnsTableCreated(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Check _columns table exists
	var name string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='_columns'").Scan(&name)
	if err != nil {
		t.Fatalf("_columns table not found: %v", err)
	}

	// Check required columns exist
	rows, err := database.Query("PRAGMA table_info(_columns)")
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		columns[name] = true
	}

	required := []string{"table_name", "column_name", "pg_type", "is_nullable", "default_value", "is_primary"}
	for _, col := range required {
		if !columns[col] {
			t.Errorf("missing required column: %s", col)
		}
	}
}
