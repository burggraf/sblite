# Phase 1: Migration System Foundation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a Supabase CLI-compatible migration system that tracks schema changes in both a database table and filesystem.

**Architecture:** Migrations are SQL files stored in `./migrations/` with timestamp prefixes. The `_schema_migrations` table tracks which migrations have been applied. All commands mirror Supabase CLI conventions.

**Tech Stack:** Go, SQLite, Cobra CLI

---

## Task 1: Add _schema_migrations Table

**Files:**
- Modify: `internal/db/migrations.go`
- Test: `internal/db/migrations_test.go`

**Step 1: Write the failing test**

Add to `internal/db/migrations_test.go`:

```go
func TestSchemaMigrationsTableCreated(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify _schema_migrations table exists
	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='_schema_migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("_schema_migrations table not found: %v", err)
	}

	// Verify table has correct columns
	rows, err := database.Query("PRAGMA table_info(_schema_migrations)")
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		columns[name] = true
	}

	expected := []string{"version", "name", "applied_at"}
	for _, col := range expected {
		if !columns[col] {
			t.Errorf("expected column %s not found", col)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/db/... -run TestSchemaMigrationsTableCreated -v
```

Expected: FAIL with "_schema_migrations table not found"

**Step 3: Write minimal implementation**

Add to `internal/db/migrations.go` after the `columnsSchema` const:

```go
const schemaMigrationsSchema = `
CREATE TABLE IF NOT EXISTS _schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`
```

Then add to `RunMigrations()` function, after the `columnsSchema` execution:

```go
_, err = db.Exec(schemaMigrationsSchema)
if err != nil {
    return fmt.Errorf("failed to run schema migrations table creation: %w", err)
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/db/... -run TestSchemaMigrationsTableCreated -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/db/migrations.go internal/db/migrations_test.go
git commit -m "feat(db): add _schema_migrations table for tracking applied migrations"
```

---

## Task 2: Create Migration Package - Types

**Files:**
- Create: `internal/migration/migration.go`
- Test: `internal/migration/migration_test.go`

**Step 1: Write the failing test**

Create `internal/migration/migration_test.go`:

```go
// internal/migration/migration_test.go
package migration

import (
	"testing"
	"time"
)

func TestMigrationVersion(t *testing.T) {
	// Test that GenerateVersion produces expected format
	version := GenerateVersion()

	// Should be 14 digits: YYYYMMDDHHmmss
	if len(version) != 14 {
		t.Errorf("expected version length 14, got %d: %s", len(version), version)
	}

	// Should be parseable as a timestamp
	_, err := time.Parse("20060102150405", version)
	if err != nil {
		t.Errorf("version not parseable as timestamp: %v", err)
	}
}

func TestMigrationFilename(t *testing.T) {
	m := Migration{
		Version: "20260117143022",
		Name:    "create_posts",
	}

	expected := "20260117143022_create_posts.sql"
	if m.Filename() != expected {
		t.Errorf("expected %s, got %s", expected, m.Filename())
	}
}

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		filename string
		version  string
		name     string
		valid    bool
	}{
		{"20260117143022_create_posts.sql", "20260117143022", "create_posts", true},
		{"20260117143022_add_user_id_to_posts.sql", "20260117143022", "add_user_id_to_posts", true},
		{"invalid.sql", "", "", false},
		{"20260117143022.sql", "", "", false},
		{"not_a_migration.txt", "", "", false},
	}

	for _, tt := range tests {
		m, err := ParseFilename(tt.filename)
		if tt.valid {
			if err != nil {
				t.Errorf("ParseFilename(%q) unexpected error: %v", tt.filename, err)
				continue
			}
			if m.Version != tt.version {
				t.Errorf("ParseFilename(%q) version = %q, want %q", tt.filename, m.Version, tt.version)
			}
			if m.Name != tt.name {
				t.Errorf("ParseFilename(%q) name = %q, want %q", tt.filename, m.Name, tt.name)
			}
		} else {
			if err == nil {
				t.Errorf("ParseFilename(%q) expected error, got nil", tt.filename)
			}
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -v
```

Expected: FAIL with "package not found" or "undefined: GenerateVersion"

**Step 3: Write minimal implementation**

Create `internal/migration/migration.go`:

```go
// internal/migration/migration.go
package migration

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Migration represents a single database migration
type Migration struct {
	Version   string    // Timestamp version (YYYYMMDDHHmmss)
	Name      string    // Human-readable name
	SQL       string    // SQL statements to execute
	AppliedAt time.Time // When migration was applied (zero if pending)
}

// GenerateVersion creates a new migration version based on current UTC time
func GenerateVersion() string {
	return time.Now().UTC().Format("20060102150405")
}

// Filename returns the migration filename
func (m Migration) Filename() string {
	return fmt.Sprintf("%s_%s.sql", m.Version, m.Name)
}

// filenameRegex matches migration filenames: YYYYMMDDHHmmss_name.sql
var filenameRegex = regexp.MustCompile(`^(\d{14})_(.+)\.sql$`)

// ParseFilename parses a migration filename into a Migration struct
func ParseFilename(filename string) (Migration, error) {
	matches := filenameRegex.FindStringSubmatch(filename)
	if matches == nil {
		return Migration{}, fmt.Errorf("invalid migration filename: %s", filename)
	}

	return Migration{
		Version: matches[1],
		Name:    strings.TrimSuffix(matches[2], ".sql"),
	}, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/migration/migration.go internal/migration/migration_test.go
git commit -m "feat(migration): add core migration types and filename parsing"
```

---

## Task 3: Create Migration Runner - Applied Migrations Query

**Files:**
- Create: `internal/migration/runner.go`
- Test: `internal/migration/runner_test.go`

**Step 1: Write the failing test**

Create `internal/migration/runner_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunner -v
```

Expected: FAIL with "undefined: NewRunner"

**Step 3: Write minimal implementation**

Create `internal/migration/runner.go`:

```go
// internal/migration/runner.go
package migration

import (
	"database/sql"
	"time"
)

// Runner handles migration execution against a database
type Runner struct {
	db *sql.DB
}

// NewRunner creates a new migration runner
func NewRunner(db *sql.DB) *Runner {
	return &Runner{db: db}
}

// GetApplied returns all applied migrations, ordered by version ascending
func (r *Runner) GetApplied() ([]Migration, error) {
	rows, err := r.db.Query(`
		SELECT version, name, applied_at
		FROM _schema_migrations
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var m Migration
		var appliedAt string
		if err := rows.Scan(&m.Version, &m.Name, &appliedAt); err != nil {
			return nil, err
		}
		m.AppliedAt, _ = time.Parse("2006-01-02 15:04:05", appliedAt)
		migrations = append(migrations, m)
	}

	return migrations, rows.Err()
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunner -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/migration/runner.go internal/migration/runner_test.go
git commit -m "feat(migration): add Runner with GetApplied() query"
```

---

## Task 4: Migration Runner - Apply Single Migration

**Files:**
- Modify: `internal/migration/runner.go`
- Modify: `internal/migration/runner_test.go`

**Step 1: Write the failing test**

Add to `internal/migration/runner_test.go`:

```go
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
			CREATE TABLE bad_table (id INVALID_TYPE);
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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerApply -v
```

Expected: FAIL with "undefined: runner.Apply"

**Step 3: Write minimal implementation**

Add to `internal/migration/runner.go`:

```go
import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Apply executes a migration within a transaction
func (r *Runner) Apply(m Migration) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the migration SQL
	// Split by semicolons and execute each statement
	statements := splitStatements(m.SQL)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.Version, err)
		}
	}

	// Record the migration
	_, err = tx.Exec(`
		INSERT INTO _schema_migrations (version, name)
		VALUES (?, ?)
	`, m.Version, m.Name)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// splitStatements splits SQL by semicolons, respecting quotes
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for _, ch := range sql {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar {
				inString = false
			}
		} else if ch == '\'' || ch == '"' {
			current.WriteRune(ch)
			inString = true
			stringChar = ch
		} else if ch == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}

	// Don't forget the last statement if no trailing semicolon
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerApply -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/migration/runner.go internal/migration/runner_test.go
git commit -m "feat(migration): add Apply() with transaction and rollback support"
```

---

## Task 5: Migration Runner - Read Migrations from Filesystem

**Files:**
- Modify: `internal/migration/runner.go`
- Modify: `internal/migration/runner_test.go`

**Step 1: Write the failing test**

Add to `internal/migration/runner_test.go`:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/markb/sblite/internal/db"
)

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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerReadFromDir -v
```

Expected: FAIL with "undefined: ReadFromDir"

**Step 3: Write minimal implementation**

Add to `internal/migration/runner.go`:

```go
import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReadFromDir reads all migration files from a directory
func ReadFromDir(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		m, err := ParseFilename(entry.Name())
		if err != nil {
			// Skip files that don't match migration pattern
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}
		m.SQL = string(content)

		migrations = append(migrations, m)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerReadFromDir -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/migration/runner.go internal/migration/runner_test.go
git commit -m "feat(migration): add ReadFromDir() to load migrations from filesystem"
```

---

## Task 6: Migration Runner - Get Pending Migrations

**Files:**
- Modify: `internal/migration/runner.go`
- Modify: `internal/migration/runner_test.go`

**Step 1: Write the failing test**

Add to `internal/migration/runner_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerGetPending -v
```

Expected: FAIL with "undefined: runner.GetPending"

**Step 3: Write minimal implementation**

Add to `internal/migration/runner.go`:

```go
// GetPending returns migrations that exist in the directory but haven't been applied
func (r *Runner) GetPending(dir string) ([]Migration, error) {
	all, err := ReadFromDir(dir)
	if err != nil {
		return nil, err
	}

	applied, err := r.GetApplied()
	if err != nil {
		return nil, err
	}

	// Build set of applied versions
	appliedSet := make(map[string]bool)
	for _, m := range applied {
		appliedSet[m.Version] = true
	}

	// Filter to pending only
	var pending []Migration
	for _, m := range all {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}

	return pending, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./internal/migration/... -run TestRunnerGetPending -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add internal/migration/runner.go internal/migration/runner_test.go
git commit -m "feat(migration): add GetPending() to find unapplied migrations"
```

---

## Task 7: CLI Command - migration new

**Files:**
- Create: `cmd/migration.go`

**Step 1: Create the command file**

Create `cmd/migration.go`:

```go
// cmd/migration.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/markb/sblite/internal/migration"
	"github.com/spf13/cobra"
)

var migrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "Manage database migrations",
	Long:  `Commands for creating and listing database migrations.`,
}

var migrationNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new migration file",
	Long: `Create a new migration file with a timestamp prefix.

The name should be a short description using snake_case.

Examples:
  sblite migration new create_posts
  sblite migration new add_user_id_to_posts`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Validate name (alphanumeric and underscores only)
		if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(name) {
			return fmt.Errorf("migration name must be lowercase alphanumeric with underscores, starting with a letter")
		}

		// Create migrations directory if needed
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}

		// Generate migration
		m := migration.Migration{
			Version: migration.GenerateVersion(),
			Name:    name,
		}

		// Create file with template
		filename := filepath.Join(migrationsDir, m.Filename())
		content := fmt.Sprintf(`-- Migration: %s
-- Created: %s

-- Write your SQL here

`, m.Name, m.Version)

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create migration file: %w", err)
		}

		fmt.Printf("Created migration: %s\n", filename)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrationCmd)
	migrationCmd.AddCommand(migrationNewCmd)

	migrationNewCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
}
```

**Step 2: Build and test manually**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go build -o sblite .
./sblite migration new create_posts
ls -la migrations/
cat migrations/*_create_posts.sql
```

Expected: File created with correct format

**Step 3: Clean up test file**

```bash
rm -rf migrations/
```

**Step 4: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add cmd/migration.go
git commit -m "feat(cli): add 'sblite migration new' command"
```

---

## Task 8: CLI Command - migration list

**Files:**
- Modify: `cmd/migration.go`

**Step 1: Add the list subcommand**

Add to `cmd/migration.go`:

```go
var migrationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all migrations and their status",
	Long: `Show all migrations from the migrations directory and whether they've been applied.

Examples:
  sblite migration list
  sblite migration list --db data.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s (run 'sblite init' first)", dbPath)
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		runner := migration.NewRunner(database.DB)

		// Get all migrations from filesystem
		all, err := migration.ReadFromDir(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to read migrations: %w", err)
		}

		// Get applied migrations
		applied, err := runner.GetApplied()
		if err != nil {
			return fmt.Errorf("failed to get applied migrations: %w", err)
		}

		// Build applied set
		appliedMap := make(map[string]migration.Migration)
		for _, m := range applied {
			appliedMap[m.Version] = m
		}

		if len(all) == 0 {
			fmt.Println("No migrations found in", migrationsDir)
			return nil
		}

		// Print table
		fmt.Printf("%-16s %-30s %s\n", "VERSION", "NAME", "STATUS")
		fmt.Println(strings.Repeat("-", 60))

		pendingCount := 0
		for _, m := range all {
			status := "pending"
			if a, ok := appliedMap[m.Version]; ok {
				status = fmt.Sprintf("applied %s", a.AppliedAt.Format("2006-01-02 15:04"))
			} else {
				pendingCount++
			}
			fmt.Printf("%-16s %-30s %s\n", m.Version, m.Name, status)
		}

		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%d applied, %d pending\n", len(applied), pendingCount)

		return nil
	},
}
```

Also add to the `init()` function:

```go
migrationCmd.AddCommand(migrationListCmd)

migrationListCmd.Flags().String("db", "data.db", "Database path")
migrationListCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
```

And add the import:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/migration"
	"github.com/spf13/cobra"
)
```

**Step 2: Build and test manually**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go build -o sblite .
./sblite init --db test.db
mkdir -p migrations
echo "CREATE TABLE posts (id TEXT);" > migrations/20260117100000_create_posts.sql
echo "CREATE TABLE users (id TEXT);" > migrations/20260117110000_create_users.sql
./sblite migration list --db test.db
```

Expected: Shows both migrations as "pending"

**Step 3: Clean up test files**

```bash
rm -f test.db test.db-wal test.db-shm
rm -rf migrations/
```

**Step 4: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add cmd/migration.go
git commit -m "feat(cli): add 'sblite migration list' command"
```

---

## Task 9: CLI Command - db push

**Files:**
- Create: `cmd/db.go`

**Step 1: Create the db push command**

Create `cmd/db.go`:

```go
// cmd/db.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/migration"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Commands for managing the database schema via migrations.`,
}

var dbPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Apply pending migrations",
	Long: `Apply all pending migrations to the database.

Migrations are applied in order by their timestamp version.
Each migration runs in a transaction - if it fails, the migration
is rolled back and no further migrations are applied.

Examples:
  sblite db push
  sblite db push --db data.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s (run 'sblite init' first)", dbPath)
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		runner := migration.NewRunner(database.DB)

		// Get pending migrations
		pending, err := runner.GetPending(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to get pending migrations: %w", err)
		}

		if len(pending) == 0 {
			fmt.Println("No pending migrations")
			return nil
		}

		fmt.Printf("Applying %d migration(s)...\n\n", len(pending))

		for _, m := range pending {
			fmt.Printf("  Applying %s_%s... ", m.Version, m.Name)
			if err := runner.Apply(m); err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("migration failed: %w", err)
			}
			fmt.Println("done")
		}

		fmt.Printf("\nApplied %d migration(s)\n", len(pending))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbPushCmd)

	dbPushCmd.Flags().String("db", "data.db", "Database path")
	dbPushCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
}
```

**Step 2: Build and test manually**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go build -o sblite .

# Initialize database
./sblite init --db test.db

# Create test migrations
mkdir -p migrations
cat > migrations/20260117100000_create_posts.sql << 'EOF'
CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL
);

INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
VALUES ('posts', 'id', 'uuid', 0, 1), ('posts', 'title', 'text', 0, 0);
EOF

cat > migrations/20260117110000_create_comments.sql << 'EOF'
CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL,
    body TEXT
);

INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
VALUES ('comments', 'id', 'uuid', 0, 1), ('comments', 'post_id', 'uuid', 0, 0), ('comments', 'body', 'text', 1, 0);
EOF

# Push migrations
./sblite db push --db test.db

# Verify
./sblite migration list --db test.db
sqlite3 test.db ".tables"
```

Expected: Both migrations applied successfully, tables created

**Step 3: Clean up test files**

```bash
rm -f test.db test.db-wal test.db-shm
rm -rf migrations/
```

**Step 4: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add cmd/db.go
git commit -m "feat(cli): add 'sblite db push' command to apply pending migrations"
```

---

## Task 10: Run All Tests and Verify

**Step 1: Run the full test suite**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go test ./...
```

Expected: All tests pass (except the 3 pre-existing failures in rest package)

**Step 2: Manual integration test**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
go build -o sblite .

# Full workflow test
rm -f integration.db integration.db-*
rm -rf migrations/

./sblite init --db integration.db
./sblite migration new create_posts
./sblite migration new add_comments

# Edit the migrations to add actual SQL
cat > migrations/*_create_posts.sql << 'EOF'
-- Migration: create_posts

CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
VALUES
    ('posts', 'id', 'uuid', 0, 1),
    ('posts', 'title', 'text', 0, 0),
    ('posts', 'created_at', 'timestamptz', 1, 0);
EOF

cat > migrations/*_add_comments.sql << 'EOF'
-- Migration: add_comments

CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    post_id TEXT REFERENCES posts(id),
    body TEXT NOT NULL
);

INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary)
VALUES
    ('comments', 'id', 'uuid', 0, 1),
    ('comments', 'post_id', 'uuid', 1, 0),
    ('comments', 'body', 'text', 0, 0);
EOF

./sblite migration list --db integration.db
./sblite db push --db integration.db
./sblite migration list --db integration.db

# Verify tables exist
sqlite3 integration.db ".tables"
sqlite3 integration.db "SELECT * FROM _schema_migrations"
```

**Step 3: Clean up**

```bash
rm -f integration.db integration.db-*
rm -rf migrations/
```

**Step 4: Final commit with all tests passing**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard
git add -A
git status
# If there are any uncommitted changes, commit them
```

---

## Summary

Phase 1 complete. The migration system now supports:

- **`_schema_migrations` table** - Tracks applied migrations in the database
- **`internal/migration` package** - Core types and logic for migrations
- **`sblite migration new <name>`** - Creates timestamped migration files
- **`sblite migration list`** - Shows applied/pending migration status
- **`sblite db push`** - Applies pending migrations with transaction safety

Next phase: Dashboard shell with setup password authentication.
