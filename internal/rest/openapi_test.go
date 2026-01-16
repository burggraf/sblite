// internal/rest/openapi_test.go
package rest

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupOpenAPITestDB creates an in-memory SQLite database for OpenAPI tests.
func setupOpenAPITestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	return db
}

func TestGenerateOpenAPISpec_EmptyDatabase(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.OpenAPI != "3.0.0" {
		t.Errorf("expected OpenAPI 3.0.0, got %s", spec.OpenAPI)
	}

	if spec.Info.Title != "sblite REST API" {
		t.Errorf("expected title 'sblite REST API', got %s", spec.Info.Title)
	}

	if spec.Info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", spec.Info.Version)
	}

	if len(spec.Paths) != 0 {
		t.Errorf("expected 0 paths for empty database, got %d", len(spec.Paths))
	}

	if len(spec.Components.Schemas) != 0 {
		t.Errorf("expected 0 schemas for empty database, got %d", len(spec.Components.Schemas))
	}
}

func TestGenerateOpenAPISpec_WithTables(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	// Create test tables
	_, err := db.Exec(`
		CREATE TABLE todos (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			completed BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create todos table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT,
			age INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check paths
	if len(spec.Paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(spec.Paths))
	}

	if _, ok := spec.Paths["/rest/v1/todos"]; !ok {
		t.Error("expected /rest/v1/todos path")
	}

	if _, ok := spec.Paths["/rest/v1/users"]; !ok {
		t.Error("expected /rest/v1/users path")
	}

	// Check schemas
	if len(spec.Components.Schemas) != 2 {
		t.Errorf("expected 2 schemas, got %d", len(spec.Components.Schemas))
	}

	if _, ok := spec.Components.Schemas["todos"]; !ok {
		t.Error("expected todos schema")
	}

	if _, ok := spec.Components.Schemas["users"]; !ok {
		t.Error("expected users schema")
	}

	// Check todos schema properties
	todosSchema := spec.Components.Schemas["todos"]
	if todosSchema.Type != "object" {
		t.Errorf("expected todos schema type 'object', got %s", todosSchema.Type)
	}

	if _, ok := todosSchema.Properties["id"]; !ok {
		t.Error("expected id property in todos schema")
	}

	if _, ok := todosSchema.Properties["title"]; !ok {
		t.Error("expected title property in todos schema")
	}

	// Check title is required (NOT NULL without default)
	foundTitle := false
	for _, req := range todosSchema.Required {
		if req == "title" {
			foundTitle = true
			break
		}
	}
	if !foundTitle {
		t.Error("expected 'title' to be in required fields")
	}
}

func TestGenerateOpenAPISpec_ExcludesInternalTables(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	// Create internal tables (should be excluded)
	_, err := db.Exec(`CREATE TABLE auth_users (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create auth_users table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE _migrations (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create _migrations table: %v", err)
	}

	// Create a user table (should be included)
	_, err = db.Exec(`CREATE TABLE posts (id INTEGER PRIMARY KEY, content TEXT)`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have posts
	if len(spec.Paths) != 1 {
		t.Errorf("expected 1 path (posts only), got %d", len(spec.Paths))
	}

	if _, ok := spec.Paths["/rest/v1/posts"]; !ok {
		t.Error("expected /rest/v1/posts path")
	}

	if _, ok := spec.Paths["/rest/v1/auth_users"]; ok {
		t.Error("auth_users should be excluded")
	}

	if _, ok := spec.Paths["/rest/v1/_migrations"]; ok {
		t.Error("_migrations should be excluded")
	}
}

func TestGenerateOpenAPISpec_PathOperations(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path, ok := spec.Paths["/rest/v1/items"]
	if !ok {
		t.Fatal("expected /rest/v1/items path")
	}

	// Check all CRUD operations exist
	if path.Get == nil {
		t.Error("expected GET operation")
	}
	if path.Post == nil {
		t.Error("expected POST operation")
	}
	if path.Patch == nil {
		t.Error("expected PATCH operation")
	}
	if path.Delete == nil {
		t.Error("expected DELETE operation")
	}

	// Check GET operation details
	if path.Get.Summary == "" {
		t.Error("expected GET summary")
	}
	if len(path.Get.Responses) == 0 {
		t.Error("expected GET responses")
	}
	if _, ok := path.Get.Responses["200"]; !ok {
		t.Error("expected 200 response")
	}

	// Check POST operation has request body
	if path.Post.RequestBody == nil {
		t.Error("expected POST request body")
	}
}

func TestSqliteTypeToSchema(t *testing.T) {
	tests := []struct {
		sqliteType     string
		expectedType   string
		expectedFormat string
	}{
		{"INTEGER", "integer", ""},
		{"INT", "integer", ""},
		{"BIGINT", "integer", ""},
		{"REAL", "number", "double"},
		{"FLOAT", "number", "double"},
		{"DOUBLE", "number", "double"},
		{"TEXT", "string", ""},
		{"VARCHAR(255)", "string", ""},
		{"CHAR(10)", "string", ""},
		{"BLOB", "string", "binary"},
		{"BOOLEAN", "boolean", ""},
		{"BOOL", "boolean", ""},
		{"DATE", "string", "date"},
		{"TIME", "string", "time"},
		{"TIMESTAMP", "string", "date-time"},
		{"DATETIME", "string", "date-time"},
		{"UUID", "string", "uuid"},
		{"JSON", "object", ""},
		{"UNKNOWN", "string", ""}, // Default to string
	}

	for _, tt := range tests {
		t.Run(tt.sqliteType, func(t *testing.T) {
			schema := sqliteTypeToSchema(tt.sqliteType)
			if schema.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, schema.Type)
			}
			if schema.Format != tt.expectedFormat {
				t.Errorf("expected format %s, got %s", tt.expectedFormat, schema.Format)
			}
		})
	}
}

func TestGenerateOpenAPISpec_SecuritySchemes(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check security schemes
	if len(spec.Components.SecuritySchemes) != 2 {
		t.Errorf("expected 2 security schemes, got %d", len(spec.Components.SecuritySchemes))
	}

	bearer, ok := spec.Components.SecuritySchemes["bearerAuth"]
	if !ok {
		t.Fatal("expected bearerAuth security scheme")
	}
	if bearer.Type != "http" {
		t.Errorf("expected type http, got %s", bearer.Type)
	}
	if bearer.Scheme != "bearer" {
		t.Errorf("expected scheme bearer, got %s", bearer.Scheme)
	}
	if bearer.BearerFormat != "JWT" {
		t.Errorf("expected bearerFormat JWT, got %s", bearer.BearerFormat)
	}

	apiKey, ok := spec.Components.SecuritySchemes["apiKey"]
	if !ok {
		t.Fatal("expected apiKey security scheme")
	}
	if apiKey.Type != "apiKey" {
		t.Errorf("expected type apiKey, got %s", apiKey.Type)
	}
	if apiKey.Name != "apikey" {
		t.Errorf("expected name apikey, got %s", apiKey.Name)
	}
	if apiKey.In != "header" {
		t.Errorf("expected in header, got %s", apiKey.In)
	}
}

func TestGenerateOpenAPISpec_NullableColumns(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE products (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			price REAL NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	spec, err := GenerateOpenAPISpec(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema := spec.Components.Schemas["products"]

	// name should NOT be nullable (NOT NULL)
	if schema.Properties["name"].Nullable {
		t.Error("name should not be nullable")
	}

	// description should be nullable (no NOT NULL)
	if !schema.Properties["description"].Nullable {
		t.Error("description should be nullable")
	}

	// price should NOT be nullable (NOT NULL)
	if schema.Properties["price"].Nullable {
		t.Error("price should not be nullable")
	}

	// Check required fields
	requiredMap := make(map[string]bool)
	for _, r := range schema.Required {
		requiredMap[r] = true
	}

	if !requiredMap["name"] {
		t.Error("name should be in required")
	}
	if !requiredMap["price"] {
		t.Error("price should be in required")
	}
	if requiredMap["description"] {
		t.Error("description should not be in required")
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "Users"},
		{"todos", "Todos"},
		{"", ""},
		{"U", "U"},
		{"user_posts", "User_posts"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalizeFirst(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetUserTables(t *testing.T) {
	db := setupOpenAPITestDB(t)
	defer db.Close()

	// Create various tables
	tables := []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY)",
		"CREATE TABLE posts (id INTEGER PRIMARY KEY)",
		"CREATE TABLE auth_tokens (id INTEGER PRIMARY KEY)",
		"CREATE TABLE _internal (id INTEGER PRIMARY KEY)",
	}

	for _, sql := range tables {
		if _, err := db.Exec(sql); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	userTables, err := getUserTables(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(userTables) != 2 {
		t.Errorf("expected 2 user tables, got %d: %v", len(userTables), userTables)
	}

	// Tables should be sorted alphabetically
	if userTables[0] != "posts" {
		t.Errorf("expected first table to be 'posts', got %s", userTables[0])
	}
	if userTables[1] != "users" {
		t.Errorf("expected second table to be 'users', got %s", userTables[1])
	}
}
