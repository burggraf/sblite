package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
)

func setupTestHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	sch := schema.New(database.DB)
	handler := NewHandler(database, sch)
	return handler, database
}

func TestCreateTable(t *testing.T) {
	handler, database := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	// Create a table with various column types
	body := `{
		"name": "users",
		"columns": [
			{"name": "id", "type": "uuid", "nullable": false, "default": "gen_random_uuid()", "primary": true},
			{"name": "email", "type": "text", "nullable": false},
			{"name": "age", "type": "integer", "nullable": true},
			{"name": "active", "type": "boolean", "nullable": false, "default": "true"},
			{"name": "created_at", "type": "timestamptz", "nullable": false, "default": "now()"}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the response contains table info
	var response TableInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Name != "users" {
		t.Errorf("expected table name 'users', got %q", response.Name)
	}

	if len(response.Columns) != 5 {
		t.Errorf("expected 5 columns, got %d", len(response.Columns))
	}

	// Verify the SQLite table actually exists
	var tableName string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&tableName)
	if err != nil {
		t.Fatalf("table 'users' was not created: %v", err)
	}

	// Verify metadata was registered
	sch := schema.New(database.DB)
	columns, err := sch.GetColumns("users")
	if err != nil {
		t.Fatalf("failed to get columns: %v", err)
	}

	if len(columns) != 5 {
		t.Errorf("expected 5 columns in metadata, got %d", len(columns))
	}

	// Check specific column metadata
	idCol, ok := columns["id"]
	if !ok {
		t.Fatal("expected 'id' column in metadata")
	}
	if idCol.PgType != "uuid" {
		t.Errorf("expected id type 'uuid', got %q", idCol.PgType)
	}
	if !idCol.IsPrimary {
		t.Error("expected id to be primary key")
	}
}

func TestCreateTable_InvalidType(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	// Try to create a table with an invalid type
	body := `{
		"name": "invalid_table",
		"columns": [
			{"name": "id", "type": "varchar", "nullable": false}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response contains error message
	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error == "" {
		t.Error("expected error message in response")
	}
}

func TestCreateTable_EmptyName(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	body := `{
		"name": "",
		"columns": [
			{"name": "id", "type": "uuid", "nullable": false}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateTable_NoColumns(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	body := `{
		"name": "empty_table",
		"columns": []
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTables(t *testing.T) {
	handler, database := setupTestHandler(t)

	// Create a table using the handler
	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)
	r.Get("/admin/v1/tables", handler.ListTables)

	// First create a table
	body := `{
		"name": "products",
		"columns": [
			{"name": "id", "type": "uuid", "nullable": false, "primary": true},
			{"name": "name", "type": "text", "nullable": false}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create table: %d: %s", w.Code, w.Body.String())
	}

	// Now list tables
	req = httptest.NewRequest("GET", "/admin/v1/tables", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []TableInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 1 {
		t.Errorf("expected 1 table, got %d", len(response))
	}

	if len(response) > 0 && response[0].Name != "products" {
		t.Errorf("expected table name 'products', got %q", response[0].Name)
	}

	// Verify columns are included
	if len(response) > 0 && len(response[0].Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(response[0].Columns))
	}

	_ = database // silence unused warning
}

func TestGetTable(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)
	r.Get("/admin/v1/tables/{name}", handler.GetTable)

	// First create a table
	body := `{
		"name": "orders",
		"columns": [
			{"name": "id", "type": "uuid", "nullable": false, "primary": true},
			{"name": "total", "type": "numeric", "nullable": false},
			{"name": "data", "type": "jsonb", "nullable": true}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create table: %d: %s", w.Code, w.Body.String())
	}

	// Now get the table
	req = httptest.NewRequest("GET", "/admin/v1/tables/orders", nil)
	w = httptest.NewRecorder()

	// Need to set up chi context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "orders")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response TableInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Name != "orders" {
		t.Errorf("expected table name 'orders', got %q", response.Name)
	}

	if len(response.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(response.Columns))
	}
}

func TestGetTable_NotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Get("/admin/v1/tables/{name}", handler.GetTable)

	req := httptest.NewRequest("GET", "/admin/v1/tables/nonexistent", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteTable(t *testing.T) {
	handler, database := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)
	r.Delete("/admin/v1/tables/{name}", handler.DeleteTable)

	// First create a table
	body := `{
		"name": "temp_table",
		"columns": [
			{"name": "id", "type": "uuid", "nullable": false, "primary": true}
		]
	}`
	req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create table: %d: %s", w.Code, w.Body.String())
	}

	// Verify table exists
	var tableName string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='temp_table'`).Scan(&tableName)
	if err != nil {
		t.Fatalf("table 'temp_table' was not created: %v", err)
	}

	// Now delete the table
	req = httptest.NewRequest("DELETE", "/admin/v1/tables/temp_table", nil)
	w = httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "temp_table")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify table no longer exists
	err = database.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='temp_table'`).Scan(&tableName)
	if err == nil {
		t.Error("table 'temp_table' should have been deleted")
	}

	// Verify metadata was deleted
	sch := schema.New(database.DB)
	columns, _ := sch.GetColumns("temp_table")
	if len(columns) != 0 {
		t.Errorf("expected 0 columns in metadata after delete, got %d", len(columns))
	}
}

func TestPgTypeToSQLite(t *testing.T) {
	tests := []struct {
		pgType     string
		sqliteType string
	}{
		{"uuid", "TEXT"},
		{"text", "TEXT"},
		{"numeric", "TEXT"},
		{"timestamptz", "TEXT"},
		{"jsonb", "TEXT"},
		{"integer", "INTEGER"},
		{"boolean", "INTEGER"},
		{"bytea", "BLOB"},
	}

	for _, tt := range tests {
		t.Run(tt.pgType, func(t *testing.T) {
			result := pgTypeToSQLite(tt.pgType)
			if result != tt.sqliteType {
				t.Errorf("pgTypeToSQLite(%q) = %q, want %q", tt.pgType, result, tt.sqliteType)
			}
		})
	}
}

func TestMapDefaultValue(t *testing.T) {
	tests := []struct {
		defaultVal string
		pgType     string
		expected   string
	}{
		{"gen_random_uuid()", "uuid", "(lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))"},
		{"now()", "timestamptz", "(datetime('now'))"},
		{"true", "boolean", "1"},
		{"false", "boolean", "0"},
		{"'default text'", "text", "'default text'"},
		{"123", "integer", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.defaultVal+"_"+tt.pgType, func(t *testing.T) {
			result := mapDefaultValue(tt.defaultVal, tt.pgType)
			if result != tt.expected {
				t.Errorf("mapDefaultValue(%q, %q) = %q, want %q", tt.defaultVal, tt.pgType, result, tt.expected)
			}
		})
	}
}
