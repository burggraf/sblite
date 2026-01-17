// internal/migration/runner_test.go
package migration

import (
	"os"
	"path/filepath"
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

func TestRunnerApply(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := NewRunner(database.DB)

	m := Migration{
		Version: "20260117120000",
		Name:    "create_posts",
		SQL: `
			CREATE TABLE posts (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL
			);
			INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
			VALUES ('posts', 'id', 'uuid', 0, 1), ('posts', 'title', 'text', 0, 0);
		`,
	}

	err := runner.Apply(m)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Verify table was created
	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&tableName)
	if err != nil {
		t.Errorf("posts table not created: %v", err)
	}

	// Verify migration was recorded
	applied, err := runner.GetApplied()
	if err != nil {
		t.Fatalf("GetApplied() error: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied migration, got %d", len(applied))
	}
	if applied[0].Version != "20260117120000" {
		t.Errorf("expected version 20260117120000, got %s", applied[0].Version)
	}
}

func TestRunnerApply_Rollback(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := NewRunner(database.DB)

	m := Migration{
		Version: "20260117120000",
		Name:    "bad_migration",
		SQL: `
			CREATE TABLE good_table (id TEXT);
			INSERT INTO nonexistent_table (id) VALUES ('fail');
		`,
	}

	err := runner.Apply(m)
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}

	// Verify good_table was NOT created (rolled back)
	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='good_table'").Scan(&tableName)
	if err == nil {
		t.Error("good_table should not exist after rollback")
	}

	// Verify migration was NOT recorded
	applied, _ := runner.GetApplied()
	if len(applied) != 0 {
		t.Errorf("expected 0 applied migrations after rollback, got %d", len(applied))
	}
}

func TestRunnerReadFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create test migration files
	files := map[string]string{
		"20260117100000_create_posts.sql": "CREATE TABLE posts (id TEXT);",
		"20260117110000_create_users.sql": "CREATE TABLE users (id TEXT);",
		"not_a_migration.txt":             "should be ignored",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	migrations, err := ReadFromDir(dir)
	if err != nil {
		t.Fatalf("ReadFromDir() error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	// Verify order (by version)
	if migrations[0].Version != "20260117100000" {
		t.Errorf("expected first version 20260117100000, got %s", migrations[0].Version)
	}
	if migrations[0].Name != "create_posts" {
		t.Errorf("expected first name create_posts, got %s", migrations[0].Name)
	}
	if migrations[0].SQL != "CREATE TABLE posts (id TEXT);" {
		t.Errorf("unexpected SQL content: %s", migrations[0].SQL)
	}

	if migrations[1].Version != "20260117110000" {
		t.Errorf("expected second version 20260117110000, got %s", migrations[1].Version)
	}
}

func TestRunnerReadFromDir_Empty(t *testing.T) {
	dir := t.TempDir()

	migrations, err := ReadFromDir(dir)
	if err != nil {
		t.Fatalf("ReadFromDir() error: %v", err)
	}

	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestRunnerReadFromDir_NotExists(t *testing.T) {
	migrations, err := ReadFromDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("ReadFromDir() should not error for nonexistent dir: %v", err)
	}

	if len(migrations) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestRunnerGetPending(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	dir := t.TempDir()

	// Create migration files
	files := map[string]string{
		"20260117100000_create_posts.sql": "CREATE TABLE posts (id TEXT);",
		"20260117110000_create_users.sql": "CREATE TABLE users (id TEXT);",
		"20260117120000_add_email.sql":    "ALTER TABLE users ADD email TEXT;",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	// Mark first migration as applied
	_, err := database.Exec(`
		INSERT INTO _schema_migrations (version, name)
		VALUES ('20260117100000', 'create_posts')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	runner := NewRunner(database.DB)
	pending, err := runner.GetPending(dir)
	if err != nil {
		t.Fatalf("GetPending() error: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending migrations, got %d", len(pending))
	}

	if pending[0].Version != "20260117110000" {
		t.Errorf("expected first pending version 20260117110000, got %s", pending[0].Version)
	}
	if pending[1].Version != "20260117120000" {
		t.Errorf("expected second pending version 20260117120000, got %s", pending[1].Version)
	}
}
