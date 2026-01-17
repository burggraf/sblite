// Package dashboard provides the web dashboard for sblite administration.
package dashboard

import (
	"database/sql"
)

// Store handles dashboard key-value storage in _dashboard table.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get retrieves a value by key. Returns empty string if not found.
func (s *Store) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM _dashboard WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// Set stores a value by key (upsert).
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO _dashboard (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, key, value)
	return err
}

// HasPassword returns true if a password hash is stored.
func (s *Store) HasPassword() bool {
	hash, err := s.Get("password_hash")
	return err == nil && hash != ""
}
