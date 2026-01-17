// internal/migration/runner.go
package migration

import (
	"database/sql"
	"fmt"
	"strings"
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
