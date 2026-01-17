// Package migration provides database migration tracking and execution for sblite.
// It manages versioned SQL migration files and tracks which migrations have been
// applied to the database via the _schema_migrations table.
package migration

import (
	"fmt"
	"regexp"
	"time"
)

// Migration represents a single database migration.
type Migration struct {
	Version   string    // Timestamp version (YYYYMMDDHHmmss)
	Name      string    // Human-readable name
	SQL       string    // SQL statements to execute
	AppliedAt time.Time // When migration was applied (zero if pending)
}

// GenerateVersion creates a new migration version based on current UTC time.
func GenerateVersion() string {
	return time.Now().UTC().Format("20060102150405")
}

// Filename returns the migration filename in the format: version_name.sql
func (m Migration) Filename() string {
	return fmt.Sprintf("%s_%s.sql", m.Version, m.Name)
}

// filenameRegex matches migration filenames: YYYYMMDDHHmmss_name.sql
var filenameRegex = regexp.MustCompile(`^(\d{14})_(.+)\.sql$`)

// ParseFilename parses a migration filename into a Migration struct.
// Returns an error if the filename doesn't match the expected format.
func ParseFilename(filename string) (Migration, error) {
	matches := filenameRegex.FindStringSubmatch(filename)
	if matches == nil {
		return Migration{}, fmt.Errorf("invalid migration filename: %s", filename)
	}

	return Migration{
		Version: matches[1],
		Name:    matches[2],
	}, nil
}
