package schema

import (
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
	schema := New(database.DB)

	// Register a column
	col := Column{
		TableName:    "users",
		ColumnName:   "email",
		PgType:       "text",
		IsNullable:   false,
		DefaultValue: "",
		IsPrimary:    false,
	}

	err := schema.RegisterColumn(col)
	if err != nil {
		t.Fatalf("RegisterColumn failed: %v", err)
	}

	// Verify it's in GetColumns result
	columns, err := schema.GetColumns("users")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(columns))
	}

	result, ok := columns["email"]
	if !ok {
		t.Fatal("expected to find 'email' column")
	}

	if result.TableName != "users" {
		t.Errorf("expected TableName 'users', got '%s'", result.TableName)
	}
	if result.ColumnName != "email" {
		t.Errorf("expected ColumnName 'email', got '%s'", result.ColumnName)
	}
	if result.PgType != "text" {
		t.Errorf("expected PgType 'text', got '%s'", result.PgType)
	}
	if result.IsNullable != false {
		t.Errorf("expected IsNullable false, got %v", result.IsNullable)
	}
}

func TestRegisterColumn_Update(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Register a column
	col := Column{
		TableName:    "users",
		ColumnName:   "email",
		PgType:       "text",
		IsNullable:   true,
		DefaultValue: "",
		IsPrimary:    false,
	}

	err := schema.RegisterColumn(col)
	if err != nil {
		t.Fatalf("RegisterColumn failed: %v", err)
	}

	// Update the same column
	col.IsNullable = false
	col.DefaultValue = "default@example.com"

	err = schema.RegisterColumn(col)
	if err != nil {
		t.Fatalf("RegisterColumn (update) failed: %v", err)
	}

	// Verify it's updated
	columns, err := schema.GetColumns("users")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	result := columns["email"]
	if result.IsNullable != false {
		t.Errorf("expected IsNullable false after update, got %v", result.IsNullable)
	}
	if result.DefaultValue != "default@example.com" {
		t.Errorf("expected DefaultValue 'default@example.com', got '%s'", result.DefaultValue)
	}
}

func TestGetColumns_EmptyTable(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Get columns for a nonexistent table
	columns, err := schema.GetColumns("nonexistent_table")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 0 {
		t.Errorf("expected empty map for nonexistent table, got %d columns", len(columns))
	}
}

func TestRegisterColumn_InvalidType(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Try to register a column with an invalid pg type
	col := Column{
		TableName:    "users",
		ColumnName:   "bad_column",
		PgType:       "varchar", // not in our supported types
		IsNullable:   true,
		DefaultValue: "",
		IsPrimary:    false,
	}

	err := schema.RegisterColumn(col)
	if err == nil {
		t.Fatal("expected error for invalid pg type, got nil")
	}
}

func TestDeleteTableColumns(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Register multiple columns
	cols := []Column{
		{TableName: "users", ColumnName: "id", PgType: "uuid", IsPrimary: true},
		{TableName: "users", ColumnName: "email", PgType: "text"},
		{TableName: "users", ColumnName: "created_at", PgType: "timestamptz"},
		{TableName: "posts", ColumnName: "id", PgType: "uuid", IsPrimary: true},
	}

	for _, col := range cols {
		if err := schema.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed for %s.%s: %v", col.TableName, col.ColumnName, err)
		}
	}

	// Verify users table has 3 columns
	columns, _ := schema.GetColumns("users")
	if len(columns) != 3 {
		t.Fatalf("expected 3 columns for users, got %d", len(columns))
	}

	// Delete all columns for users table
	err := schema.DeleteTableColumns("users")
	if err != nil {
		t.Fatalf("DeleteTableColumns failed: %v", err)
	}

	// Verify users table has no columns
	columns, _ = schema.GetColumns("users")
	if len(columns) != 0 {
		t.Errorf("expected 0 columns for users after delete, got %d", len(columns))
	}

	// Verify posts table is unaffected
	columns, _ = schema.GetColumns("posts")
	if len(columns) != 1 {
		t.Errorf("expected 1 column for posts, got %d", len(columns))
	}
}

func TestDeleteColumn(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Register multiple columns
	cols := []Column{
		{TableName: "users", ColumnName: "id", PgType: "uuid", IsPrimary: true},
		{TableName: "users", ColumnName: "email", PgType: "text"},
		{TableName: "users", ColumnName: "name", PgType: "text"},
	}

	for _, col := range cols {
		if err := schema.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	// Delete single column
	err := schema.DeleteColumn("users", "email")
	if err != nil {
		t.Fatalf("DeleteColumn failed: %v", err)
	}

	// Verify email column is gone
	columns, _ := schema.GetColumns("users")
	if len(columns) != 2 {
		t.Errorf("expected 2 columns after delete, got %d", len(columns))
	}
	if _, ok := columns["email"]; ok {
		t.Error("email column should have been deleted")
	}
	if _, ok := columns["id"]; !ok {
		t.Error("id column should still exist")
	}
	if _, ok := columns["name"]; !ok {
		t.Error("name column should still exist")
	}
}

func TestListTables(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Register columns for multiple tables
	cols := []Column{
		{TableName: "users", ColumnName: "id", PgType: "uuid", IsPrimary: true},
		{TableName: "users", ColumnName: "email", PgType: "text"},
		{TableName: "posts", ColumnName: "id", PgType: "uuid", IsPrimary: true},
		{TableName: "posts", ColumnName: "title", PgType: "text"},
		{TableName: "comments", ColumnName: "id", PgType: "uuid", IsPrimary: true},
	}

	for _, col := range cols {
		if err := schema.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	// List all tables
	tables, err := schema.ListTables()
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("expected 3 tables, got %d", len(tables))
	}

	// Check that all expected tables are present
	tableSet := make(map[string]bool)
	for _, name := range tables {
		tableSet[name] = true
	}

	for _, expected := range []string{"users", "posts", "comments"} {
		if !tableSet[expected] {
			t.Errorf("expected table '%s' not found", expected)
		}
	}
}

func TestListTables_Empty(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// List tables when there are none
	tables, err := schema.ListTables()
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}

	if len(tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(tables))
	}
}

func TestRegisterColumn_AllTypes(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	// Test all valid pg types
	validTypes := []string{"uuid", "text", "integer", "numeric", "boolean", "timestamptz", "jsonb", "bytea"}

	for i, pgType := range validTypes {
		col := Column{
			TableName:  "test_table",
			ColumnName: "col_" + pgType,
			PgType:     pgType,
			IsNullable: true,
		}
		err := schema.RegisterColumn(col)
		if err != nil {
			t.Errorf("RegisterColumn failed for type '%s': %v", pgType, err)
		}

		// Verify count
		columns, _ := schema.GetColumns("test_table")
		if len(columns) != i+1 {
			t.Errorf("expected %d columns after registering '%s', got %d", i+1, pgType, len(columns))
		}
	}
}

func TestColumn_PrimaryAndDefault(t *testing.T) {
	database := setupTestDB(t)
	schema := New(database.DB)

	col := Column{
		TableName:    "users",
		ColumnName:   "id",
		PgType:       "uuid",
		IsNullable:   false,
		DefaultValue: "gen_random_uuid()",
		IsPrimary:    true,
	}

	err := schema.RegisterColumn(col)
	if err != nil {
		t.Fatalf("RegisterColumn failed: %v", err)
	}

	columns, _ := schema.GetColumns("users")
	result := columns["id"]

	if !result.IsPrimary {
		t.Error("expected IsPrimary to be true")
	}
	if result.DefaultValue != "gen_random_uuid()" {
		t.Errorf("expected DefaultValue 'gen_random_uuid()', got '%s'", result.DefaultValue)
	}
}
