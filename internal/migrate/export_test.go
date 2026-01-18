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

	sch := schema.New(database.DB)
	return database, sch
}

func TestExportDDL(t *testing.T) {
	_, sch := setupTestDB(t)

	// Register metadata for a table
	cols := []schema.Column{
		{TableName: "users", ColumnName: "id", PgType: "uuid", IsNullable: false, IsPrimary: true},
		{TableName: "users", ColumnName: "email", PgType: "text", IsNullable: false},
		{TableName: "users", ColumnName: "name", PgType: "text", IsNullable: true},
		{TableName: "users", ColumnName: "age", PgType: "integer", IsNullable: true},
	}

	for _, col := range cols {
		if err := sch.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	// Create exporter and export DDL
	exporter := New(sch)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Verify output contains correct Postgres types
	if !strings.Contains(ddl, "CREATE TABLE users") {
		t.Error("expected DDL to contain 'CREATE TABLE users'")
	}
	if !strings.Contains(ddl, "id UUID") {
		t.Error("expected DDL to contain 'id UUID'")
	}
	if !strings.Contains(ddl, "email TEXT NOT NULL") {
		t.Error("expected DDL to contain 'email TEXT NOT NULL'")
	}
	if !strings.Contains(ddl, "name TEXT") {
		t.Error("expected DDL to contain 'name TEXT'")
	}
	if !strings.Contains(ddl, "age INTEGER") {
		t.Error("expected DDL to contain 'age INTEGER'")
	}
	if !strings.Contains(ddl, "PRIMARY KEY") {
		t.Error("expected DDL to contain 'PRIMARY KEY'")
	}
}

func TestExportDDL_WithDefaults(t *testing.T) {
	_, sch := setupTestDB(t)

	// Register columns with various default values
	cols := []schema.Column{
		{TableName: "items", ColumnName: "id", PgType: "uuid", IsNullable: false, DefaultValue: "gen_uuid()", IsPrimary: true},
		{TableName: "items", ColumnName: "created_at", PgType: "timestamptz", IsNullable: false, DefaultValue: "now()"},
		{TableName: "items", ColumnName: "active", PgType: "boolean", IsNullable: false, DefaultValue: "true"},
		{TableName: "items", ColumnName: "deleted", PgType: "boolean", IsNullable: false, DefaultValue: "false"},
		{TableName: "items", ColumnName: "status", PgType: "text", IsNullable: false, DefaultValue: "pending"},
		{TableName: "items", ColumnName: "count", PgType: "integer", IsNullable: false, DefaultValue: "0"},
	}

	for _, col := range cols {
		if err := sch.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	exporter := New(sch)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Test default value mapping
	// gen_uuid() should become gen_random_uuid()
	if !strings.Contains(ddl, "gen_random_uuid()") {
		t.Error("expected 'gen_uuid()' to be mapped to 'gen_random_uuid()'")
	}
	// now() stays as now()
	if !strings.Contains(ddl, "DEFAULT now()") {
		t.Error("expected 'now()' to remain as 'now()'")
	}
	// true/false stay as true/false
	if !strings.Contains(ddl, "DEFAULT true") {
		t.Error("expected 'true' to remain as 'true'")
	}
	if !strings.Contains(ddl, "DEFAULT false") {
		t.Error("expected 'false' to remain as 'false'")
	}
	// String literals should be quoted
	if !strings.Contains(ddl, "DEFAULT 'pending'") {
		t.Error("expected 'pending' to become quoted 'pending'")
	}
	// Numeric literals stay as-is
	if !strings.Contains(ddl, "DEFAULT 0") {
		t.Error("expected '0' to remain as '0'")
	}
}

func TestExportDDL_MultipleTables(t *testing.T) {
	_, sch := setupTestDB(t)

	// Register columns for multiple tables
	cols := []schema.Column{
		{TableName: "users", ColumnName: "id", PgType: "uuid", IsNullable: false, IsPrimary: true},
		{TableName: "users", ColumnName: "email", PgType: "text", IsNullable: false},
		{TableName: "posts", ColumnName: "id", PgType: "uuid", IsNullable: false, IsPrimary: true},
		{TableName: "posts", ColumnName: "title", PgType: "text", IsNullable: false},
		{TableName: "posts", ColumnName: "user_id", PgType: "uuid", IsNullable: false},
	}

	for _, col := range cols {
		if err := sch.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	exporter := New(sch)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Should contain both tables
	if !strings.Contains(ddl, "CREATE TABLE users") {
		t.Error("expected DDL to contain 'CREATE TABLE users'")
	}
	if !strings.Contains(ddl, "CREATE TABLE posts") {
		t.Error("expected DDL to contain 'CREATE TABLE posts'")
	}
}

func TestExportDDL_EmptySchema(t *testing.T) {
	_, sch := setupTestDB(t)

	// Don't register any columns
	exporter := New(sch)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Should have header but no CREATE TABLE statements
	if !strings.Contains(ddl, "sblite") {
		t.Error("expected DDL to contain header comment")
	}
	if strings.Contains(ddl, "CREATE TABLE") {
		t.Error("expected no CREATE TABLE statements for empty schema")
	}
}

func TestExportDDL_AllTypes(t *testing.T) {
	_, sch := setupTestDB(t)

	// Register columns with all supported types
	cols := []schema.Column{
		{TableName: "test_types", ColumnName: "col_uuid", PgType: "uuid", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_text", PgType: "text", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_integer", PgType: "integer", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_numeric", PgType: "numeric", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_boolean", PgType: "boolean", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_timestamptz", PgType: "timestamptz", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_jsonb", PgType: "jsonb", IsNullable: true},
		{TableName: "test_types", ColumnName: "col_bytea", PgType: "bytea", IsNullable: true},
	}

	for _, col := range cols {
		if err := sch.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	exporter := New(sch)
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Verify all types are present in uppercase (PostgreSQL convention)
	expectedTypes := []string{"UUID", "TEXT", "INTEGER", "NUMERIC", "BOOLEAN", "TIMESTAMPTZ", "JSONB", "BYTEA"}
	for _, typ := range expectedTypes {
		if !strings.Contains(ddl, typ) {
			t.Errorf("expected DDL to contain type '%s'", typ)
		}
	}
}

func TestMapDefaultToPostgres(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gen_uuid()", "gen_random_uuid()"},
		{"now()", "now()"},
		{"true", "true"},
		{"false", "false"},
		{"123", "123"},
		{"0", "0"},
		{"-42", "-42"},
		{"3.14", "3.14"},
		{"pending", "'pending'"},
		{"hello world", "'hello world'"},
		{"", ""},
	}

	for _, tt := range tests {
		result := mapDefaultToPostgres(tt.input)
		if result != tt.expected {
			t.Errorf("mapDefaultToPostgres(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExportDDL_WithFTSIndex(t *testing.T) {
	database, sch := setupTestDB(t)

	// Create test table
	_, err := database.DB.Exec(`CREATE TABLE articles (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		body TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Register columns
	cols := []schema.Column{
		{TableName: "articles", ColumnName: "id", PgType: "integer", IsNullable: false, IsPrimary: true},
		{TableName: "articles", ColumnName: "title", PgType: "text", IsNullable: false},
		{TableName: "articles", ColumnName: "body", PgType: "text", IsNullable: false},
	}
	for _, col := range cols {
		if err := sch.RegisterColumn(col); err != nil {
			t.Fatalf("RegisterColumn failed: %v", err)
		}
	}

	// Create FTS exporter with FTS support
	exporter := NewWithFTS(sch, database.DB)

	// Create FTS index using the fts package
	ftsManager := exporter.fts
	err = ftsManager.CreateIndex("articles", "search", []string{"title", "body"}, "porter")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Export DDL
	ddl, err := exporter.ExportDDL()
	if err != nil {
		t.Fatalf("ExportDDL failed: %v", err)
	}

	// Verify FTS index is included
	if !strings.Contains(ddl, "Full-Text Search Indexes") {
		t.Error("expected DDL to contain FTS section header")
	}
	if !strings.Contains(ddl, "articles_search_fts_idx") {
		t.Error("expected DDL to contain FTS index name")
	}
	if !strings.Contains(ddl, "USING GIN") {
		t.Error("expected DDL to contain GIN index type")
	}
	if !strings.Contains(ddl, "to_tsvector('english'") {
		t.Error("expected DDL to contain to_tsvector with 'english' config for porter tokenizer")
	}
	if !strings.Contains(ddl, "coalesce(title, '')") {
		t.Error("expected DDL to contain coalesce for title column")
	}
	if !strings.Contains(ddl, "coalesce(body, '')") {
		t.Error("expected DDL to contain coalesce for body column")
	}
}

func TestMapTokenizerToTSConfig(t *testing.T) {
	tests := []struct {
		tokenizer string
		expected  string
	}{
		{"porter", "english"},
		{"unicode61", "simple"},
		{"ascii", "simple"},
		{"trigram", "simple"},
		{"unknown", "simple"},
		{"", "simple"},
	}

	for _, tt := range tests {
		result := mapTokenizerToTSConfig(tt.tokenizer)
		if result != tt.expected {
			t.Errorf("mapTokenizerToTSConfig(%q) = %q, want %q", tt.tokenizer, result, tt.expected)
		}
	}
}
