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
