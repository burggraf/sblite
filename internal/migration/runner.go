// internal/migration/runner.go
package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/markb/sblite/internal/pgtranslate"
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

		// Translate PostgreSQL syntax to SQLite
		// This allows migrations written in PostgreSQL syntax to work with SQLite
		translated := pgtranslate.TranslateToSQLite(stmt)

		if _, err := tx.Exec(translated); err != nil {
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
