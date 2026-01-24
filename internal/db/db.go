// internal/db/db.go
package db

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"

	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

func init() {
	// Register the REGEXP function for SQLite
	// This enables PostgreSQL regex operators (~, ~*, !~, !~*) to work after translation
	if err := sqlite.RegisterScalarFunction("regexp", 2, regexpFunc); err != nil {
		// Ignore error if already registered (e.g., during tests)
		_ = err
	}
}

// regexpFunc implements the REGEXP function for SQLite.
// Returns 1 if pattern matches text, 0 otherwise.
// Signature: REGEXP(pattern, text) -> returns 1 if pattern matches text
func regexpFunc(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if args[0] == nil || args[1] == nil {
		return nil, nil // NULL
	}

	var pattern, text string

	switch p := args[0].(type) {
	case string:
		pattern = p
	case []byte:
		pattern = string(p)
	default:
		return int64(0), nil
	}

	switch t := args[1].(type) {
	case string:
		text = t
	case []byte:
		text = string(t)
	default:
		return int64(0), nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return int64(0), nil // Invalid pattern returns false
	}

	if re.MatchString(text) {
		return int64(1), nil
	}
	return int64(0), nil
}

type DB struct {
	*sql.DB
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &DB{db}, nil
}
