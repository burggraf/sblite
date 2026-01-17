# Type System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable sblite apps to track PostgreSQL types for SQLite-stored data and migrate cleanly to Supabase.

**Architecture:** A `_columns` metadata table stores intended Postgres types. A `types` package validates data on writes. An admin API allows structured table creation. A CLI command exports PostgreSQL DDL.

**Tech Stack:** Go 1.25, Chi router, modernc.org/sqlite, google/uuid

---

## Task 1: Add `_columns` Metadata Table

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/migrations_test.go`

**Step 1: Write the failing test**

Add to `internal/db/migrations_test.go`:

```go
func TestColumnsTableCreated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Check _columns table exists
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='_columns'").Scan(&name)
	if err != nil {
		t.Fatalf("_columns table not found: %v", err)
	}

	// Check required columns exist
	rows, err := db.Query("PRAGMA table_info(_columns)")
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		columns[name] = true
	}

	required := []string{"table_name", "column_name", "pg_type", "is_nullable", "default_value", "is_primary"}
	for _, col := range required {
		if !columns[col] {
			t.Errorf("missing required column: %s", col)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/db -run TestColumnsTableCreated -v`
Expected: FAIL with "_columns table not found"

**Step 3: Add the schema**

In `internal/db/migrations.go`, add after `emailSchema`:

```go
const columnsSchema = `
CREATE TABLE IF NOT EXISTS _columns (
    table_name    TEXT NOT NULL,
    column_name   TEXT NOT NULL,
    pg_type       TEXT NOT NULL CHECK (pg_type IN (
                    'uuid', 'text', 'integer', 'numeric',
                    'boolean', 'timestamptz', 'jsonb', 'bytea'
                  )),
    is_nullable   INTEGER DEFAULT 1,
    default_value TEXT,
    is_primary    INTEGER DEFAULT 0,
    created_at    TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (table_name, column_name)
);

CREATE INDEX IF NOT EXISTS idx_columns_table ON _columns(table_name);
`
```

In `RunMigrations()`, add before the final return:

```go
	_, err = db.Exec(columnsSchema)
	if err != nil {
		return fmt.Errorf("failed to run columns schema migration: %w", err)
	}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/db -run TestColumnsTableCreated -v`
Expected: PASS

**Step 5: Run all db tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/db -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/db/migrations.go internal/db/migrations_test.go
git commit -m "feat(db): add _columns metadata table for type tracking"
```

---

## Task 2: Create Type Validators

**Files:**
- Create: `internal/types/types.go`
- Create: `internal/types/validate.go`
- Create: `internal/types/validate_test.go`

**Step 1: Create the types package with type definitions**

Create `internal/types/types.go`:

```go
// Package types provides PostgreSQL type definitions and validation for sblite.
package types

// PgType represents a supported PostgreSQL type.
type PgType string

const (
	TypeUUID        PgType = "uuid"
	TypeText        PgType = "text"
	TypeInteger     PgType = "integer"
	TypeNumeric     PgType = "numeric"
	TypeBoolean     PgType = "boolean"
	TypeTimestamptz PgType = "timestamptz"
	TypeJSONB       PgType = "jsonb"
	TypeBytea       PgType = "bytea"
)

// ValidTypes is the set of all supported types.
var ValidTypes = map[PgType]bool{
	TypeUUID:        true,
	TypeText:        true,
	TypeInteger:     true,
	TypeNumeric:     true,
	TypeBoolean:     true,
	TypeTimestamptz: true,
	TypeJSONB:       true,
	TypeBytea:       true,
}

// IsValidType checks if a type string is a supported type.
func IsValidType(t string) bool {
	return ValidTypes[PgType(t)]
}
```

**Step 2: Write failing tests for validators**

Create `internal/types/validate_test.go`:

```go
package types

import "testing"

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid uuid uppercase", "550E8400-E29B-41D4-A716-446655440000", false},
		{"invalid format", "not-a-uuid", true},
		{"too short", "550e8400-e29b-41d4", true},
		{"nil", nil, false}, // null is valid
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeUUID, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(uuid, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid int", 42, false},
		{"valid int64", int64(42), false},
		{"valid float64 whole", float64(42), false},
		{"negative", -100, false},
		{"max int32", 2147483647, false},
		{"min int32", -2147483648, false},
		{"overflow", int64(2147483648), true},
		{"underflow", int64(-2147483649), true},
		{"float with decimal", 42.5, true},
		{"string number", "42", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeInteger, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(integer, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNumeric(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid decimal string", "123.45", false},
		{"valid negative", "-99.00", false},
		{"valid integer string", "1000", false},
		{"valid with leading zero", "0.123", false},
		{"number as float", 123.45, false},
		{"number as int", 100, false},
		{"invalid double decimal", "12.34.56", true},
		{"invalid letters", "abc", true},
		{"invalid mixed", "12abc", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeNumeric, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(numeric, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBoolean(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"int 1", 1, false},
		{"int 0", 0, false},
		{"float 1", float64(1), false},
		{"float 0", float64(0), false},
		{"int 2", 2, true},
		{"string true", "true", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeBoolean, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(boolean, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimestamptz(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"ISO 8601 UTC", "2024-01-15T10:30:00Z", false},
		{"ISO 8601 offset", "2024-01-15T10:30:00+05:00", false},
		{"ISO 8601 no tz", "2024-01-15T10:30:00", false},
		{"date only", "2024-01-15", false},
		{"invalid", "yesterday", true},
		{"invalid format", "01-15-2024", true},
		{"empty", "", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeTimestamptz, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(timestamptz, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSONB(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"object string", `{"key": "value"}`, false},
		{"array string", `[1, 2, 3]`, false},
		{"map", map[string]any{"key": "value"}, false},
		{"slice", []any{1, 2, 3}, false},
		{"invalid json string", `{invalid`, true},
		{"plain string", "hello", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeJSONB, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(jsonb, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBytea(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid base64", "SGVsbG8gV29ybGQ=", false},
		{"valid base64 padded", "SGVsbG8=", false},
		{"byte slice", []byte("hello"), false},
		{"invalid base64", "not@valid!base64", true},
		{"empty string", "", false}, // empty is valid base64
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeBytea, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(bytea, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateText(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"string", "hello", false},
		{"empty", "", false},
		{"unicode", "こんにちは", false},
		{"nil", nil, false},
		{"number", 123, true}, // text requires string type
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(TypeText, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(text, %v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUnknownType(t *testing.T) {
	err := Validate(PgType("unknown"), "value")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/types -v`
Expected: FAIL (package doesn't exist yet or Validate function missing)

**Step 4: Implement validators**

Create `internal/types/validate.go`:

```go
package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"
)

// Validator is a function that validates a value for a specific type.
type Validator func(value any) error

var validators = map[PgType]Validator{
	TypeUUID:        validateUUID,
	TypeText:        validateText,
	TypeInteger:     validateInteger,
	TypeNumeric:     validateNumeric,
	TypeBoolean:     validateBoolean,
	TypeTimestamptz: validateTimestamptz,
	TypeJSONB:       validateJSONB,
	TypeBytea:       validateBytea,
}

// Validate checks if a value is valid for the given PostgreSQL type.
// Returns nil if valid, error describing the problem if invalid.
// Null values (nil) are always valid - use is_nullable for null checks.
func Validate(pgType PgType, value any) error {
	if value == nil {
		return nil
	}

	v, ok := validators[pgType]
	if !ok {
		return fmt.Errorf("unknown type: %s", pgType)
	}

	return v(value)
}

// UUID regex: 8-4-4-4-12 hex digits
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateUUID(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("uuid must be a string, got %T", value)
	}
	if s == "" {
		return fmt.Errorf("uuid cannot be empty")
	}
	if !uuidRegex.MatchString(s) {
		return fmt.Errorf("invalid uuid format: %s", s)
	}
	return nil
}

func validateText(value any) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("text must be a string, got %T", value)
	}
	return nil
}

func validateInteger(value any) error {
	var n int64

	switch v := value.(type) {
	case int:
		n = int64(v)
	case int32:
		n = int64(v)
	case int64:
		n = v
	case float64:
		if v != math.Trunc(v) {
			return fmt.Errorf("integer cannot have decimal places: %v", v)
		}
		n = int64(v)
	default:
		return fmt.Errorf("integer must be a number, got %T", value)
	}

	if n > math.MaxInt32 || n < math.MinInt32 {
		return fmt.Errorf("integer out of range (must fit int32): %d", n)
	}

	return nil
}

// Numeric regex: optional sign, digits, optional decimal with more digits
var numericRegex = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

func validateNumeric(value any) error {
	switch v := value.(type) {
	case string:
		if !numericRegex.MatchString(v) {
			return fmt.Errorf("invalid numeric format: %s", v)
		}
		return nil
	case int, int32, int64, float32, float64:
		// Numbers are valid
		return nil
	default:
		return fmt.Errorf("numeric must be a string or number, got %T", value)
	}
}

func validateBoolean(value any) error {
	switch v := value.(type) {
	case bool:
		return nil
	case int:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean integer must be 0 or 1, got %d", v)
		}
		return nil
	case int64:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean integer must be 0 or 1, got %d", v)
		}
		return nil
	case float64:
		if v != 0 && v != 1 {
			return fmt.Errorf("boolean must be 0 or 1, got %v", v)
		}
		return nil
	default:
		return fmt.Errorf("boolean must be true/false or 0/1, got %T", value)
	}
}

// Supported timestamp formats
var timestampFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func validateTimestamptz(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("timestamptz must be a string, got %T", value)
	}
	if s == "" {
		return fmt.Errorf("timestamptz cannot be empty")
	}

	for _, format := range timestampFormats {
		if _, err := time.Parse(format, s); err == nil {
			return nil
		}
	}

	return fmt.Errorf("invalid timestamp format: %s (expected ISO 8601)", s)
}

func validateJSONB(value any) error {
	switch v := value.(type) {
	case string:
		if !json.Valid([]byte(v)) {
			return fmt.Errorf("invalid JSON: %s", v)
		}
		// Also check it's an object or array, not a primitive
		var js any
		if err := json.Unmarshal([]byte(v), &js); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		switch js.(type) {
		case map[string]any, []any:
			return nil
		default:
			return fmt.Errorf("jsonb must be an object or array, got primitive")
		}
	case map[string]any, []any:
		return nil
	default:
		return fmt.Errorf("jsonb must be a JSON string, object, or array, got %T", value)
	}
}

func validateBytea(value any) error {
	switch v := value.(type) {
	case []byte:
		return nil
	case string:
		if v == "" {
			return nil // empty string is valid
		}
		_, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("invalid base64: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("bytea must be base64 string or []byte, got %T", value)
	}
}
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/types -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/types/
git commit -m "feat(types): add type validators for all supported pg types"
```

---

## Task 3: Create Schema Package

**Files:**
- Create: `internal/schema/schema.go`
- Create: `internal/schema/schema_test.go`

**Step 1: Write failing tests**

Create `internal/schema/schema_test.go`:

```go
package schema

import (
	"database/sql"
	"os"
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "schema_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return database
}

func TestRegisterColumn(t *testing.T) {
	database := setupTestDB(t)
	s := New(database.DB)

	err := s.RegisterColumn("products", "id", "uuid", false, "", true)
	if err != nil {
		t.Fatalf("RegisterColumn failed: %v", err)
	}

	cols, err := s.GetColumns("products")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(cols) != 1 {
		t.Fatalf("expected 1 column, got %d", len(cols))
	}

	col := cols["id"]
	if col.PgType != "uuid" {
		t.Errorf("expected pg_type uuid, got %s", col.PgType)
	}
	if col.IsNullable {
		t.Error("expected is_nullable false")
	}
	if !col.IsPrimary {
		t.Error("expected is_primary true")
	}
}

func TestGetColumns_EmptyTable(t *testing.T) {
	database := setupTestDB(t)
	s := New(database.DB)

	cols, err := s.GetColumns("nonexistent")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(cols) != 0 {
		t.Errorf("expected empty map, got %d columns", len(cols))
	}
}

func TestRegisterColumn_InvalidType(t *testing.T) {
	database := setupTestDB(t)
	s := New(database.DB)

	err := s.RegisterColumn("products", "id", "invalid_type", true, "", false)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestDeleteTableColumns(t *testing.T) {
	database := setupTestDB(t)
	s := New(database.DB)

	_ = s.RegisterColumn("products", "id", "uuid", false, "", true)
	_ = s.RegisterColumn("products", "name", "text", false, "", false)

	err := s.DeleteTableColumns("products")
	if err != nil {
		t.Fatalf("DeleteTableColumns failed: %v", err)
	}

	cols, _ := s.GetColumns("products")
	if len(cols) != 0 {
		t.Errorf("expected 0 columns after delete, got %d", len(cols))
	}
}

func TestListTables(t *testing.T) {
	database := setupTestDB(t)
	s := New(database.DB)

	_ = s.RegisterColumn("products", "id", "uuid", false, "", true)
	_ = s.RegisterColumn("orders", "id", "uuid", false, "", true)

	tables, err := s.ListTables()
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}

	if len(tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(tables))
	}

	found := make(map[string]bool)
	for _, table := range tables {
		found[table] = true
	}

	if !found["products"] || !found["orders"] {
		t.Errorf("expected products and orders, got %v", tables)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/schema -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement schema package**

Create `internal/schema/schema.go`:

```go
// Package schema manages type metadata for user tables.
package schema

import (
	"database/sql"
	"fmt"

	"github.com/markb/sblite/internal/types"
)

// Column represents column metadata from _columns table.
type Column struct {
	TableName    string
	ColumnName   string
	PgType       string
	IsNullable   bool
	DefaultValue string
	IsPrimary    bool
}

// Schema provides operations on table/column metadata.
type Schema struct {
	db *sql.DB
}

// New creates a new Schema instance.
func New(db *sql.DB) *Schema {
	return &Schema{db: db}
}

// RegisterColumn adds or updates a column's type metadata.
func (s *Schema) RegisterColumn(tableName, columnName, pgType string, isNullable bool, defaultValue string, isPrimary bool) error {
	if !types.IsValidType(pgType) {
		return fmt.Errorf("invalid type: %s", pgType)
	}

	_, err := s.db.Exec(`
		INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (table_name, column_name) DO UPDATE SET
			pg_type = excluded.pg_type,
			is_nullable = excluded.is_nullable,
			default_value = excluded.default_value,
			is_primary = excluded.is_primary
	`, tableName, columnName, pgType, boolToInt(isNullable), nullString(defaultValue), boolToInt(isPrimary))

	return err
}

// GetColumns returns all column metadata for a table.
func (s *Schema) GetColumns(tableName string) (map[string]Column, error) {
	rows, err := s.db.Query(`
		SELECT table_name, column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns
		WHERE table_name = ?
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]Column)
	for rows.Next() {
		var col Column
		var isNullable, isPrimary int
		var defaultValue sql.NullString

		err := rows.Scan(&col.TableName, &col.ColumnName, &col.PgType, &isNullable, &defaultValue, &isPrimary)
		if err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}

		col.IsNullable = isNullable == 1
		col.IsPrimary = isPrimary == 1
		col.DefaultValue = defaultValue.String

		columns[col.ColumnName] = col
	}

	return columns, rows.Err()
}

// DeleteTableColumns removes all column metadata for a table.
func (s *Schema) DeleteTableColumns(tableName string) error {
	_, err := s.db.Exec("DELETE FROM _columns WHERE table_name = ?", tableName)
	return err
}

// DeleteColumn removes metadata for a single column.
func (s *Schema) DeleteColumn(tableName, columnName string) error {
	_, err := s.db.Exec("DELETE FROM _columns WHERE table_name = ? AND column_name = ?", tableName, columnName)
	return err
}

// ListTables returns all table names that have column metadata.
func (s *Schema) ListTables() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT table_name FROM _columns ORDER BY table_name")
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/schema -v`
Expected: All tests PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/schema/
git commit -m "feat(schema): add schema metadata management package"
```

---

## Task 4: Create Admin API Handlers

**Files:**
- Create: `internal/admin/handler.go`
- Create: `internal/admin/handler_test.go`

**Step 1: Write failing tests**

Create `internal/admin/handler_test.go`:

```go
package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
)

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "admin_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	s := schema.New(database.DB)
	return New(database, s)
}

func TestCreateTable(t *testing.T) {
	h := setupTestHandler(t)

	body := CreateTableRequest{
		Name: "products",
		Columns: []ColumnDef{
			{Name: "id", Type: "uuid", Primary: true},
			{Name: "name", Type: "text", Nullable: false},
			{Name: "price", Type: "numeric"},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.CreateTable(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify table exists in SQLite
	var tableName string
	err := h.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='products'").Scan(&tableName)
	if err != nil {
		t.Errorf("table not created: %v", err)
	}

	// Verify metadata exists
	cols, _ := h.schema.GetColumns("products")
	if len(cols) != 3 {
		t.Errorf("expected 3 columns in metadata, got %d", len(cols))
	}
}

func TestCreateTable_InvalidType(t *testing.T) {
	h := setupTestHandler(t)

	body := CreateTableRequest{
		Name: "products",
		Columns: []ColumnDef{
			{Name: "id", Type: "invalid_type"},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.CreateTable(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListTables(t *testing.T) {
	h := setupTestHandler(t)

	// Create a table first
	body := CreateTableRequest{
		Name: "products",
		Columns: []ColumnDef{
			{Name: "id", Type: "uuid", Primary: true},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateTable(rr, req)

	// List tables
	req = httptest.NewRequest("GET", "/admin/v1/tables", nil)
	rr = httptest.NewRecorder()
	h.ListTables(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp []TableInfo
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 table, got %d", len(resp))
	}
}

func TestGetTable(t *testing.T) {
	h := setupTestHandler(t)

	// Create a table first
	body := CreateTableRequest{
		Name: "products",
		Columns: []ColumnDef{
			{Name: "id", Type: "uuid", Primary: true},
			{Name: "name", Type: "text"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateTable(rr, req)

	// Get table - need to set chi URL param
	req = httptest.NewRequest("GET", "/admin/v1/tables/products", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "products")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr = httptest.NewRecorder()
	h.GetTable(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp TableInfo
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Name != "products" {
		t.Errorf("expected products, got %s", resp.Name)
	}
	if len(resp.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(resp.Columns))
	}
}

func TestDeleteTable(t *testing.T) {
	h := setupTestHandler(t)

	// Create a table first
	body := CreateTableRequest{
		Name: "products",
		Columns: []ColumnDef{
			{Name: "id", Type: "uuid", Primary: true},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreateTable(rr, req)

	// Delete table
	req = httptest.NewRequest("DELETE", "/admin/v1/tables/products", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "products")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr = httptest.NewRecorder()
	h.DeleteTable(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}

	// Verify table is gone
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='products'").Scan(&count)
	if count != 0 {
		t.Error("table still exists")
	}

	// Verify metadata is gone
	cols, _ := h.schema.GetColumns("products")
	if len(cols) != 0 {
		t.Error("metadata still exists")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/admin -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement admin handler**

Create `internal/admin/handler.go`:

```go
// Package admin provides HTTP handlers for administrative operations.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
	"github.com/markb/sblite/internal/types"
)

// Handler handles admin API requests.
type Handler struct {
	db     *db.DB
	schema *schema.Schema
}

// New creates a new admin handler.
func New(db *db.DB, schema *schema.Schema) *Handler {
	return &Handler{db: db, schema: schema}
}

// ColumnDef defines a column in a table creation request.
type ColumnDef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

// CreateTableRequest is the request body for creating a table.
type CreateTableRequest struct {
	Name    string      `json:"name"`
	Columns []ColumnDef `json:"columns"`
}

// TableInfo represents table information in responses.
type TableInfo struct {
	Name    string      `json:"name"`
	Columns []ColumnDef `json:"columns"`
}

// CreateTable handles POST /admin/v1/tables
func (h *Handler) CreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "table name required", http.StatusBadRequest)
		return
	}

	if len(req.Columns) == 0 {
		http.Error(w, "at least one column required", http.StatusBadRequest)
		return
	}

	// Validate all types first
	for _, col := range req.Columns {
		if !types.IsValidType(col.Type) {
			http.Error(w, fmt.Sprintf("invalid type for column %s: %s", col.Name, col.Type), http.StatusBadRequest)
			return
		}
	}

	// Build CREATE TABLE SQL
	sql, err := h.buildCreateTableSQL(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build SQL: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute in transaction
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to begin transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sql); err != nil {
		http.Error(w, fmt.Sprintf("failed to create table: %v", err), http.StatusInternalServerError)
		return
	}

	// Register column metadata
	for _, col := range req.Columns {
		if err := h.schema.RegisterColumn(req.Name, col.Name, col.Type, col.Nullable, col.Default, col.Primary); err != nil {
			http.Error(w, fmt.Sprintf("failed to register column %s: %v", col.Name, err), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("failed to commit: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(TableInfo{Name: req.Name, Columns: req.Columns})
}

func (h *Handler) buildCreateTableSQL(req CreateTableRequest) (string, error) {
	var cols []string
	var primaryKeys []string

	for _, col := range req.Columns {
		sqlType := pgTypeToSQLite(col.Type)
		colDef := fmt.Sprintf(`"%s" %s`, col.Name, sqlType)

		if !col.Nullable {
			colDef += " NOT NULL"
		}

		if col.Default != "" {
			defaultSQL := mapDefaultValue(col.Default, col.Type)
			colDef += fmt.Sprintf(" DEFAULT %s", defaultSQL)
		}

		if col.Type == "jsonb" {
			colDef += fmt.Sprintf(` CHECK (json_valid("%s"))`, col.Name)
		}

		cols = append(cols, colDef)

		if col.Primary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, col.Name))
		}
	}

	if len(primaryKeys) > 0 {
		cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	return fmt.Sprintf(`CREATE TABLE "%s" (%s)`, req.Name, strings.Join(cols, ", ")), nil
}

func pgTypeToSQLite(pgType string) string {
	switch pgType {
	case "uuid", "text", "numeric", "timestamptz", "jsonb":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "boolean":
		return "INTEGER"
	case "bytea":
		return "BLOB"
	default:
		return "TEXT"
	}
}

func mapDefaultValue(defaultVal, pgType string) string {
	switch defaultVal {
	case "gen_uuid()":
		return "(lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))"
	case "now()":
		return "(datetime('now'))"
	case "true":
		return "1"
	case "false":
		return "0"
	default:
		return fmt.Sprintf("'%s'", defaultVal)
	}
}

// ListTables handles GET /admin/v1/tables
func (h *Handler) ListTables(w http.ResponseWriter, r *http.Request) {
	tables, err := h.schema.ListTables()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list tables: %v", err), http.StatusInternalServerError)
		return
	}

	var result []TableInfo
	for _, tableName := range tables {
		cols, err := h.schema.GetColumns(tableName)
		if err != nil {
			continue
		}

		var colDefs []ColumnDef
		for _, col := range cols {
			colDefs = append(colDefs, ColumnDef{
				Name:     col.ColumnName,
				Type:     col.PgType,
				Nullable: col.IsNullable,
				Default:  col.DefaultValue,
				Primary:  col.IsPrimary,
			})
		}

		result = append(result, TableInfo{Name: tableName, Columns: colDefs})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetTable handles GET /admin/v1/tables/{name}
func (h *Handler) GetTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		http.Error(w, "table name required", http.StatusBadRequest)
		return
	}

	cols, err := h.schema.GetColumns(tableName)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get table: %v", err), http.StatusInternalServerError)
		return
	}

	if len(cols) == 0 {
		http.Error(w, "table not found", http.StatusNotFound)
		return
	}

	var colDefs []ColumnDef
	for _, col := range cols {
		colDefs = append(colDefs, ColumnDef{
			Name:     col.ColumnName,
			Type:     col.PgType,
			Nullable: col.IsNullable,
			Default:  col.DefaultValue,
			Primary:  col.IsPrimary,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TableInfo{Name: tableName, Columns: colDefs})
}

// DeleteTable handles DELETE /admin/v1/tables/{name}
func (h *Handler) DeleteTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		http.Error(w, "table name required", http.StatusBadRequest)
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to begin transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Drop the table
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)); err != nil {
		http.Error(w, fmt.Sprintf("failed to drop table: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete metadata
	if err := h.schema.DeleteTableColumns(tableName); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete metadata: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("failed to commit: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Add missing import to test file**

Add at top of `internal/admin/handler_test.go`:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	// ... rest of imports
)
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/admin -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/admin/
git commit -m "feat(admin): add admin API handlers for table management"
```

---

## Task 5: Wire Admin Routes into Server

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Step 1: Add admin route registration**

In `internal/server/server.go`, add import:

```go
import (
	// existing imports...
	"github.com/markb/sblite/internal/admin"
	"github.com/markb/sblite/internal/schema"
)
```

Add to Server struct:

```go
type Server struct {
	// existing fields...
	adminHandler *admin.Handler
	schema       *schema.Schema
}
```

In `New()` function, after db is set:

```go
	s.schema = schema.New(s.db.DB)
	s.adminHandler = admin.New(s.db, s.schema)
```

In `setupRoutes()`, add admin routes:

```go
	// Admin API routes
	r.Route("/admin/v1", func(r chi.Router) {
		r.Post("/tables", s.adminHandler.CreateTable)
		r.Get("/tables", s.adminHandler.ListTables)
		r.Get("/tables/{name}", s.adminHandler.GetTable)
		r.Delete("/tables/{name}", s.adminHandler.DeleteTable)
	})
```

**Step 2: Run all tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/server/server.go
git commit -m "feat(server): wire admin API routes"
```

---

## Task 6: Add Migration Export Command

**Files:**
- Create: `cmd/migrate.go`
- Create: `internal/migrate/export.go`
- Create: `internal/migrate/export_test.go`

**Step 1: Write failing tests**

Create `internal/migrate/export_test.go`:

```go
package migrate

import (
	"os"
	"strings"
	"testing"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
)

func setupTestDB(t *testing.T) (*db.DB, *schema.Schema) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	s := schema.New(database.DB)
	return database, s
}

func TestExportDDL(t *testing.T) {
	database, s := setupTestDB(t)

	// Create a table
	_, err := database.Exec(`CREATE TABLE "products" ("id" TEXT, "name" TEXT, "price" TEXT, "active" INTEGER)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Register metadata
	s.RegisterColumn("products", "id", "uuid", false, "", true)
	s.RegisterColumn("products", "name", "text", false, "", false)
	s.RegisterColumn("products", "price", "numeric", true, "", false)
	s.RegisterColumn("products", "active", "boolean", true, "true", false)

	exporter := New(database, s)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Check output contains correct types
	if !strings.Contains(ddl, `"id" uuid`) {
		t.Error("expected uuid type for id")
	}
	if !strings.Contains(ddl, `"name" text NOT NULL`) {
		t.Error("expected text NOT NULL for name")
	}
	if !strings.Contains(ddl, `"price" numeric`) {
		t.Error("expected numeric for price")
	}
	if !strings.Contains(ddl, `"active" boolean`) {
		t.Error("expected boolean for active")
	}
	if !strings.Contains(ddl, "PRIMARY KEY") {
		t.Error("expected PRIMARY KEY")
	}
}

func TestExportDDL_WithDefaults(t *testing.T) {
	database, s := setupTestDB(t)

	_, err := database.Exec(`CREATE TABLE "items" ("id" TEXT, "created_at" TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s.RegisterColumn("items", "id", "uuid", false, "gen_uuid()", true)
	s.RegisterColumn("items", "created_at", "timestamptz", false, "now()", false)

	exporter := New(database, s)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	if !strings.Contains(ddl, "DEFAULT gen_random_uuid()") {
		t.Errorf("expected gen_random_uuid() default, got: %s", ddl)
	}
	if !strings.Contains(ddl, "DEFAULT now()") {
		t.Errorf("expected now() default, got: %s", ddl)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/migrate -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement export**

Create `internal/migrate/export.go`:

```go
// Package migrate provides migration export functionality.
package migrate

import (
	"fmt"
	"strings"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
)

// Exporter exports sblite schema to PostgreSQL format.
type Exporter struct {
	db     *db.DB
	schema *schema.Schema
}

// New creates a new Exporter.
func New(db *db.DB, schema *schema.Schema) *Exporter {
	return &Exporter{db: db, schema: schema}
}

// ExportDDL generates PostgreSQL DDL for all user tables.
func (e *Exporter) ExportDDL() (string, error) {
	tables, err := e.schema.ListTables()
	if err != nil {
		return "", fmt.Errorf("list tables: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("-- Generated by sblite migrate export\n\n")

	for _, tableName := range tables {
		ddl, err := e.exportTable(tableName)
		if err != nil {
			return "", fmt.Errorf("export table %s: %w", tableName, err)
		}
		sb.WriteString(ddl)
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}

func (e *Exporter) exportTable(tableName string) (string, error) {
	cols, err := e.schema.GetColumns(tableName)
	if err != nil {
		return "", err
	}

	var colDefs []string
	var primaryKeys []string

	for _, col := range cols {
		colDef := fmt.Sprintf(`    "%s" %s`, col.ColumnName, col.PgType)

		if !col.IsNullable {
			colDef += " NOT NULL"
		}

		if col.DefaultValue != "" {
			pgDefault := mapDefaultToPostgres(col.DefaultValue)
			colDef += fmt.Sprintf(" DEFAULT %s", pgDefault)
		}

		colDefs = append(colDefs, colDef)

		if col.IsPrimary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, col.ColumnName))
		}
	}

	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, fmt.Sprintf("    PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	return fmt.Sprintf(`CREATE TABLE "%s" (
%s
);`, tableName, strings.Join(colDefs, ",\n")), nil
}

func mapDefaultToPostgres(defaultVal string) string {
	switch defaultVal {
	case "gen_uuid()":
		return "gen_random_uuid()"
	case "now()":
		return "now()"
	case "true":
		return "true"
	case "false":
		return "false"
	default:
		return fmt.Sprintf("'%s'", defaultVal)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./internal/migrate -v`
Expected: All tests PASS

**Step 5: Create CLI command**

Create `cmd/migrate.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/migrate"
	"github.com/markb/sblite/internal/schema"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migration commands",
}

var migrateExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export schema as PostgreSQL DDL",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		output, _ := cmd.Flags().GetString("output")

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		s := schema.New(database.DB)
		exporter := migrate.New(database, s)

		ddl, err := exporter.ExportDDL()
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		if output == "" {
			fmt.Print(ddl)
		} else {
			if err := os.WriteFile(output, []byte(ddl), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Printf("Schema exported to %s\n", output)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateExportCmd)

	migrateExportCmd.Flags().String("db", "data.db", "Database path")
	migrateExportCmd.Flags().StringP("output", "o", "", "Output file (stdout if not specified)")
}
```

**Step 6: Run all tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./... -v`
Expected: All tests PASS

**Step 7: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/migrate/ cmd/migrate.go
git commit -m "feat(migrate): add export command for PostgreSQL DDL"
```

---

## Task 7: Integrate Validation into REST Handler

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/rest/handler_test.go`

**Step 1: Add schema dependency to REST handler**

In `internal/rest/handler.go`, add to Handler struct and New function signature to accept `*schema.Schema`.

**Step 2: Add validation before insert/update**

Add validation method:

```go
func (h *Handler) validateRow(table string, data map[string]any) error {
	cols, err := h.schema.GetColumns(table)
	if err != nil {
		return err
	}

	// If no schema defined, skip validation (raw SQL table)
	if len(cols) == 0 {
		return nil
	}

	for colName, value := range data {
		col, ok := cols[colName]
		if !ok {
			continue // Unknown column, let SQLite handle it
		}

		if err := types.Validate(types.PgType(col.PgType), value); err != nil {
			return fmt.Errorf("column %q: %w", colName, err)
		}

		// Check nullable
		if !col.IsNullable && value == nil {
			return fmt.Errorf("column %q cannot be null", colName)
		}
	}

	return nil
}
```

Call `validateRow` at the start of insert/update handlers.

**Step 3: Run all tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/type-system && go test ./... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git add internal/rest/handler.go internal/server/server.go
git commit -m "feat(rest): integrate type validation on insert/update"
```

---

## Task 8: Final Integration Test

**Step 1: Run full test suite**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
go test ./... -v
```

**Step 2: Build and manual test**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
go build -o sblite .
./sblite init --db test.db
./sblite serve --db test.db &

# Create a table via admin API
curl -X POST http://localhost:8080/admin/v1/tables \
  -H "Content-Type: application/json" \
  -d '{"name":"products","columns":[{"name":"id","type":"uuid","primary":true},{"name":"name","type":"text"},{"name":"price","type":"numeric"}]}'

# Export schema
./sblite migrate export --db test.db

# Cleanup
kill %1
rm test.db*
```

**Step 3: Commit any fixes**

If any issues found, fix and commit.

**Step 4: Final commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/type-system
git log --oneline
```

Verify all commits are in place.

---

## Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | _columns table | internal/db/migrations.go |
| 2 | Type validators | internal/types/*.go |
| 3 | Schema package | internal/schema/*.go |
| 4 | Admin handlers | internal/admin/*.go |
| 5 | Route wiring | internal/server/server.go |
| 6 | Export command | internal/migrate/*.go, cmd/migrate.go |
| 7 | REST validation | internal/rest/handler.go |
| 8 | Integration test | Manual verification |

Total: ~8 commits, clean TDD progression.
