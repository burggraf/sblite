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
		{"now()", "timestamptz", "(strftime('%Y-%m-%d %H:%M:%f+00', 'now'))"},
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

func TestCreateTable_ReservedTableNames(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	tests := []struct {
		name        string
		tableName   string
		wantMessage string
	}{
		{
			name:        "auth_users is reserved",
			tableName:   "auth_users",
			wantMessage: "Table name 'auth_users' is reserved",
		},
		{
			name:        "auth_sessions is reserved",
			tableName:   "auth_sessions",
			wantMessage: "Table name 'auth_sessions' is reserved",
		},
		{
			name:        "auth_refresh_tokens is reserved",
			tableName:   "auth_refresh_tokens",
			wantMessage: "Table name 'auth_refresh_tokens' is reserved",
		},
		{
			name:        "_columns is reserved",
			tableName:   "_columns",
			wantMessage: "Table name '_columns' is reserved",
		},
		{
			name:        "_rls_policies is reserved",
			tableName:   "_rls_policies",
			wantMessage: "Table name '_rls_policies' is reserved",
		},
		{
			name:        "auth_ prefix is reserved",
			tableName:   "auth_custom",
			wantMessage: "Table names starting with 'auth_' are reserved",
		},
		{
			name:        "_ prefix is reserved",
			tableName:   "_custom_table",
			wantMessage: "Table names starting with '_' are reserved",
		},
		{
			name:        "sqlite_ prefix is reserved",
			tableName:   "sqlite_master_copy",
			wantMessage: "Table names starting with 'sqlite_' are reserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{
				"name": "` + tt.tableName + `",
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

			var response ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if response.Message != tt.wantMessage {
				t.Errorf("expected message %q, got %q", tt.wantMessage, response.Message)
			}
		})
	}
}

func TestCreateTable_InvalidColumnNames(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	tests := []struct {
		name           string
		columnName     string
		wantContains   string
	}{
		{
			name:         "column name starting with digit",
			columnName:   "1invalid",
			wantContains: "must start with a letter or underscore",
		},
		{
			name:         "column name with special characters",
			columnName:   "col-name",
			wantContains: "must start with a letter or underscore",
		},
		{
			name:         "column name with space",
			columnName:   "col name",
			wantContains: "must start with a letter or underscore",
		},
		{
			name:         "rowid is reserved",
			columnName:   "rowid",
			wantContains: "'rowid' is reserved by SQLite",
		},
		{
			name:         "ROWID is reserved (case insensitive)",
			columnName:   "ROWID",
			wantContains: "'rowid' is reserved by SQLite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{
				"name": "test_table",
				"columns": [
					{"name": "` + tt.columnName + `", "type": "text", "nullable": false}
				]
			}`
			req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
			}

			var response ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if !bytes.Contains([]byte(response.Message), []byte(tt.wantContains)) {
				t.Errorf("expected message to contain %q, got %q", tt.wantContains, response.Message)
			}
		})
	}
}

func TestCreateTable_ValidColumnNames(t *testing.T) {
	handler, _ := setupTestHandler(t)

	r := chi.NewRouter()
	r.Post("/admin/v1/tables", handler.CreateTable)

	validNames := []string{
		"id",
		"user_id",
		"_private",
		"Column1",
		"camelCase",
		"UPPERCASE",
		"a1b2c3",
	}

	for _, colName := range validNames {
		t.Run(colName, func(t *testing.T) {
			// Use a unique table name for each test
			tableName := "test_" + colName
			body := `{
				"name": "` + tableName + `",
				"columns": [
					{"name": "` + colName + `", "type": "text", "nullable": false}
				]
			}`
			req := httptest.NewRequest("POST", "/admin/v1/tables", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status 201 for column name %q, got %d: %s", colName, w.Code, w.Body.String())
			}
		})
	}
}

func TestIsReservedTableName(t *testing.T) {
	tests := []struct {
		name       string
		tableName  string
		isReserved bool
	}{
		{"auth_users", "auth_users", true},
		{"auth_sessions", "auth_sessions", true},
		{"auth_refresh_tokens", "auth_refresh_tokens", true},
		{"_columns", "_columns", true},
		{"_rls_policies", "_rls_policies", true},
		{"auth_custom", "auth_custom", true},
		{"_custom", "_custom", true},
		{"sqlite_sequence", "sqlite_sequence", true},
		{"regular_table", "regular_table", false},
		{"users", "users", false},
		{"my_auth_table", "my_auth_table", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reserved, _ := isReservedTableName(tt.tableName)
			if reserved != tt.isReserved {
				t.Errorf("isReservedTableName(%q) = %v, want %v", tt.tableName, reserved, tt.isReserved)
			}
		})
	}
}

func TestValidateColumnName(t *testing.T) {
	tests := []struct {
		name     string
		colName  string
		isValid  bool
	}{
		{"valid lowercase", "name", true},
		{"valid with underscore", "user_id", true},
		{"valid starting with underscore", "_id", true},
		{"valid uppercase", "NAME", true},
		{"valid mixed case", "userName", true},
		{"valid with numbers", "col1", true},
		{"empty name", "", false},
		{"starts with digit", "1col", false},
		{"contains hyphen", "col-name", false},
		{"contains space", "col name", false},
		{"contains dot", "col.name", false},
		{"rowid lowercase", "rowid", false},
		{"rowid uppercase", "ROWID", false},
		{"rowid mixed case", "RowId", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := validateColumnName(tt.colName)
			if valid != tt.isValid {
				t.Errorf("validateColumnName(%q) = %v, want %v", tt.colName, valid, tt.isValid)
			}
		})
	}
}
