// internal/rpc/store.go
package rpc

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store provides CRUD operations for function definitions.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create stores a new function definition.
func (s *Store) Create(def *FunctionDef) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO _rpc_functions (id, name, language, return_type, returns_set, volatility, security, source_pg, source_sqlite, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, def.ID, def.Name, def.Language, def.ReturnType, boolToInt(def.ReturnsSet), def.Volatility, def.Security, def.SourcePG, def.SourceSQLite, now, now)
	if err != nil {
		return fmt.Errorf("insert function: %w", err)
	}

	for _, arg := range def.Args {
		argID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO _rpc_function_args (id, function_id, name, type, position, default_value)
			VALUES (?, ?, ?, ?, ?, ?)
		`, argID, def.ID, arg.Name, arg.Type, arg.Position, arg.DefaultValue)
		if err != nil {
			return fmt.Errorf("insert arg %s: %w", arg.Name, err)
		}
	}

	return tx.Commit()
}

// CreateOrReplace creates or updates a function definition.
func (s *Store) CreateOrReplace(def *FunctionDef) error {
	existing, err := s.Get(def.Name)
	if err == nil {
		// Delete existing first
		if err := s.Delete(existing.Name); err != nil {
			return fmt.Errorf("delete existing: %w", err)
		}
	}
	return s.Create(def)
}

// Get retrieves a function by name.
func (s *Store) Get(name string) (*FunctionDef, error) {
	var def FunctionDef
	var returnsSet int
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT id, name, language, return_type, returns_set, volatility, security, source_pg, source_sqlite, created_at, updated_at
		FROM _rpc_functions WHERE name = ?
	`, name).Scan(&def.ID, &def.Name, &def.Language, &def.ReturnType, &returnsSet, &def.Volatility, &def.Security, &def.SourcePG, &def.SourceSQLite, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("function %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query function: %w", err)
	}
	def.ReturnsSet = returnsSet == 1

	// Parse timestamps
	def.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	def.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Load arguments
	rows, err := s.db.Query(`
		SELECT id, function_id, name, type, position, default_value
		FROM _rpc_function_args WHERE function_id = ? ORDER BY position
	`, def.ID)
	if err != nil {
		return nil, fmt.Errorf("query args: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var arg FunctionArg
		if err := rows.Scan(&arg.ID, &arg.FunctionID, &arg.Name, &arg.Type, &arg.Position, &arg.DefaultValue); err != nil {
			return nil, fmt.Errorf("scan arg: %w", err)
		}
		def.Args = append(def.Args, arg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate args: %w", err)
	}

	return &def, nil
}

// List returns all function definitions.
func (s *Store) List() ([]*FunctionDef, error) {
	rows, err := s.db.Query(`SELECT name FROM _rpc_functions ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query functions: %w", err)
	}

	// Collect all names first, then close rows before calling Get
	// to avoid holding the connection during nested queries
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan name: %w", err)
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate functions: %w", err)
	}

	var funcs []*FunctionDef
	for _, name := range names {
		def, err := s.Get(name)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, def)
	}
	return funcs, nil
}

// Delete removes a function by name.
func (s *Store) Delete(name string) error {
	result, err := s.db.Exec(`DELETE FROM _rpc_functions WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete function: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("function %q not found", name)
	}
	return nil
}

// Exists checks if a function exists.
func (s *Store) Exists(name string) bool {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM _rpc_functions WHERE name = ?`, name).Scan(&count)
	return count > 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
