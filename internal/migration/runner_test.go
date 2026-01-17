// internal/migration/runner_test.go
package migration

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
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

func TestRunnerGetAppliedMigrations_Empty(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := NewRunner(database.DB)
	applied, err := runner.GetApplied()
	if err != nil {
		t.Fatalf("GetApplied() error: %v", err)
	}

	if len(applied) != 0 {
		t.Errorf("expected 0 applied migrations, got %d", len(applied))
	}
}

func TestRunnerGetAppliedMigrations_WithData(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Insert test migration records
	_, err := database.Exec(`
		INSERT INTO _schema_migrations (version, name, applied_at)
		VALUES ('20260117100000', 'create_posts', datetime('now')),
		       ('20260117110000', 'add_user_id', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	runner := NewRunner(database.DB)
	applied, err := runner.GetApplied()
	if err != nil {
		t.Fatalf("GetApplied() error: %v", err)
	}

	if len(applied) != 2 {
		t.Errorf("expected 2 applied migrations, got %d", len(applied))
	}

	// Verify order (oldest first)
	if applied[0].Version != "20260117100000" {
		t.Errorf("expected first version 20260117100000, got %s", applied[0].Version)
	}
	if applied[1].Version != "20260117110000" {
		t.Errorf("expected second version 20260117110000, got %s", applied[1].Version)
	}
}
