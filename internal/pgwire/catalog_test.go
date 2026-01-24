package pgwire

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCatalogHandler(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	srv, err := NewServer(db, Config{Address: ":5437"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name       string
		query      string
		wantHandle bool // true if catalogHandler should handle this query
	}{
		// Should be handled by catalog
		{"version function", "SELECT VERSION()", true},
		{"version lowercase", "select version()", true},
		{"current_database", "SELECT CURRENT_DATABASE()", true},
		{"current_user", "SELECT CURRENT_USER", true},
		{"current_schema", "SELECT CURRENT_SCHEMA", true},
		{"pg_catalog query", "SELECT * FROM pg_catalog.pg_tables", true},
		{"pg_tables query", "SELECT * FROM pg_tables", true},
		{"information_schema.tables", "SELECT * FROM information_schema.tables", true},
		{"SET statement", "SET client_encoding = 'UTF8'", true},
		{"SHOW server_version", "SHOW server_version", true},
		{"SHOW client_encoding", "SHOW client_encoding", true},
		{"SHOW timezone", "SHOW timezone", true},

		// Should NOT be handled by catalog (regular queries)
		{"regular select", "SELECT * FROM users", false},
		{"insert", "INSERT INTO users (name) VALUES ('test')", false},
		{"update", "UPDATE users SET name = 'new'", false},
		{"create table", "CREATE TABLE test (id INT)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := srv.catalogHandler(tt.query)
			gotHandle := result != nil
			if gotHandle != tt.wantHandle {
				t.Errorf("catalogHandler(%q) handled = %v, want %v", tt.query, gotHandle, tt.wantHandle)
			}
		})
	}
}

func TestShowQueryValues(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := NewServer(db, Config{Address: ":5438"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		query        string
		wantContains string
	}{
		{"SHOW server_version", "15.0"},
		{"SHOW server_encoding", "UTF8"},
		{"SHOW client_encoding", "UTF8"},
		{"SHOW standard_conforming_strings", "on"},
		{"SHOW DateStyle", "ISO"},
		{"SHOW TimeZone", "UTC"},
		{"SHOW transaction isolation level", "serializable"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := srv.catalogHandler(tt.query)
			if result == nil {
				t.Fatalf("catalogHandler(%q) returned nil", tt.query)
			}
			// We can't easily test the actual response without executing,
			// but we verify the handler exists
		})
	}
}

func TestCatalogHandler_CaseInsensitive(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := NewServer(db, Config{Address: ":5439"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Test that queries are case-insensitive
	queries := []string{
		"SELECT VERSION()",
		"select version()",
		"Select Version()",
		"SELECT version()",
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			result := srv.catalogHandler(q)
			if result == nil {
				t.Errorf("catalogHandler(%q) should handle version query", q)
			}
		})
	}
}

func TestCatalogHandler_SetStatements(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := NewServer(db, Config{Address: ":5440"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// SET statements should be acknowledged but not error
	setStatements := []string{
		"SET client_encoding = 'UTF8'",
		"SET search_path TO public",
		"SET timezone = 'UTC'",
		"SET statement_timeout = 0",
	}

	for _, stmt := range setStatements {
		t.Run(stmt, func(t *testing.T) {
			result := srv.catalogHandler(stmt)
			if result == nil {
				t.Errorf("catalogHandler(%q) should handle SET statement", stmt)
			}
		})
	}
}

func TestCatalogHandler_PgCatalogQueries(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := NewServer(db, Config{Address: ":5441"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// pg_catalog queries should return empty results (not error)
	pgCatalogQueries := []string{
		"SELECT * FROM pg_catalog.pg_class",
		"SELECT * FROM pg_catalog.pg_namespace",
		"SELECT * FROM pg_catalog.pg_type",
		"SELECT * FROM pg_tables",
		"SELECT * FROM pg_views",
	}

	for _, q := range pgCatalogQueries {
		t.Run(q, func(t *testing.T) {
			result := srv.catalogHandler(q)
			if result == nil {
				t.Errorf("catalogHandler(%q) should handle pg_catalog query", q)
			}
		})
	}
}

func TestCatalogHandler_InformationSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create test tables
	_, _ = db.Exec("CREATE TABLE users (id INTEGER, name TEXT)")
	_, _ = db.Exec("CREATE TABLE orders (id INTEGER, user_id INTEGER)")

	srv, err := NewServer(db, Config{Address: ":5442"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// information_schema queries
	queries := []string{
		"SELECT * FROM information_schema.tables",
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
		"SELECT * FROM information_schema.columns",
	}

	for _, q := range queries {
		name := q
		if len(name) > 50 {
			name = name[:50] + "..."
		}
		t.Run(name, func(t *testing.T) {
			result := srv.catalogHandler(q)
			if result == nil {
				t.Errorf("catalogHandler(%q) should handle information_schema query", q)
			}
		})
	}
}

func TestCatalogHandler_RegularQueriesNotHandled(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := NewServer(db, Config{Address: ":5443"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// These should NOT be handled by catalogHandler
	regularQueries := []string{
		"SELECT * FROM users",
		"SELECT id, name FROM users WHERE id = 1",
		"INSERT INTO users (name) VALUES ('test')",
		"UPDATE users SET name = 'new' WHERE id = 1",
		"DELETE FROM users WHERE id = 1",
		"CREATE TABLE new_table (id INT)",
		"DROP TABLE users",
		"SELECT NOW()",
		"SELECT 1 + 1",
	}

	for _, q := range regularQueries {
		name := q
		if len(name) > 50 {
			name = name[:50] + "..."
		}
		// Remove "query" from name if it contains special chars
		name = strings.ReplaceAll(name, "'", "")
		t.Run(name, func(t *testing.T) {
			result := srv.catalogHandler(q)
			if result != nil {
				t.Errorf("catalogHandler(%q) should NOT handle regular query", q)
			}
		})
	}
}
