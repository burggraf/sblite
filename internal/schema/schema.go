// Package schema provides metadata management for table columns in sblite.
// It operates on the _columns table to track column types, nullability,
// defaults, and primary key status for PostgreSQL type validation.
package schema

import (
	"database/sql"
	"fmt"

	"github.com/markb/sblite/internal/types"
)

// Column represents metadata for a single column in a table.
type Column struct {
	TableName    string // Name of the table
	ColumnName   string // Name of the column
	PgType       string // PostgreSQL type (uuid, text, integer, etc.)
	IsNullable   bool   // Whether the column allows NULL values
	DefaultValue string // Default value expression (if any)
	IsPrimary    bool   // Whether this column is a primary key
}

// Schema provides operations on the _columns metadata table.
type Schema struct {
	db *sql.DB
}

// New creates a new Schema instance with the given database connection.
func New(db *sql.DB) *Schema {
	return &Schema{db: db}
}

// Execer is an interface that both *sql.DB and *sql.Tx implement.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// RegisterColumn inserts or updates a column definition in the _columns table.
// Returns an error if the PgType is not a valid supported type.
func (s *Schema) RegisterColumn(col Column) error {
	return s.registerColumnWithExecer(s.db, col)
}

// RegisterColumnTx inserts or updates a column definition within an existing transaction.
// Returns an error if the PgType is not a valid supported type.
func (s *Schema) RegisterColumnTx(tx *sql.Tx, col Column) error {
	return s.registerColumnWithExecer(tx, col)
}

// registerColumnWithExecer is the internal implementation that works with any Execer.
func (s *Schema) registerColumnWithExecer(exec Execer, col Column) error {
	// Validate the PostgreSQL type first
	if !types.IsValidType(col.PgType) {
		return fmt.Errorf("invalid PostgreSQL type: %s", col.PgType)
	}

	query := `
		INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (table_name, column_name)
		DO UPDATE SET
			pg_type = excluded.pg_type,
			is_nullable = excluded.is_nullable,
			default_value = excluded.default_value,
			is_primary = excluded.is_primary
	`

	_, err := exec.Exec(query,
		col.TableName,
		col.ColumnName,
		col.PgType,
		boolToInt(col.IsNullable),
		nullString(col.DefaultValue),
		boolToInt(col.IsPrimary),
	)
	if err != nil {
		return fmt.Errorf("failed to register column %s.%s: %w", col.TableName, col.ColumnName, err)
	}

	return nil
}

// GetColumns retrieves all column metadata for a given table.
// Returns an empty map if the table doesn't exist in _columns.
func (s *Schema) GetColumns(tableName string) (map[string]Column, error) {
	query := `
		SELECT table_name, column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns
		WHERE table_name = ?
	`

	rows, err := s.db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]Column)
	for rows.Next() {
		var col Column
		var isNullable, isPrimary int
		var defaultValue sql.NullString

		err := rows.Scan(
			&col.TableName,
			&col.ColumnName,
			&col.PgType,
			&isNullable,
			&defaultValue,
			&isPrimary,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}

		col.IsNullable = isNullable != 0
		col.IsPrimary = isPrimary != 0
		if defaultValue.Valid {
			col.DefaultValue = defaultValue.String
		}

		columns[col.ColumnName] = col
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating column rows: %w", err)
	}

	return columns, nil
}

// DeleteTableColumns removes all column metadata for a given table.
func (s *Schema) DeleteTableColumns(tableName string) error {
	query := `DELETE FROM _columns WHERE table_name = ?`
	_, err := s.db.Exec(query, tableName)
	if err != nil {
		return fmt.Errorf("failed to delete columns for table %s: %w", tableName, err)
	}
	return nil
}

// DeleteColumn removes a single column's metadata from the _columns table.
func (s *Schema) DeleteColumn(tableName, columnName string) error {
	query := `DELETE FROM _columns WHERE table_name = ? AND column_name = ?`
	_, err := s.db.Exec(query, tableName, columnName)
	if err != nil {
		return fmt.Errorf("failed to delete column %s.%s: %w", tableName, columnName, err)
	}
	return nil
}

// ListTables returns a list of all distinct table names that have columns registered.
func (s *Schema) ListTables() ([]string, error) {
	query := `SELECT DISTINCT table_name FROM _columns ORDER BY table_name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}

	return tables, nil
}

// boolToInt converts a boolean to an integer (0 or 1) for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullString converts an empty string to NULL for SQLite storage,
// or returns the string value if non-empty.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
