// internal/rest/relations_test.go
package rest

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with foreign keys enabled.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Enable foreign keys (required for PRAGMA foreign_key_list to work)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	return db
}

func TestNewRelationshipCache(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewRelationshipCache(db)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.db != db {
		t.Error("expected cache to store db reference")
	}
	if cache.cache == nil {
		t.Error("expected cache map to be initialized")
	}
}

func TestGetRelationships_ManyToOne(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create parent table
	_, err := db.Exec(`CREATE TABLE countries (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	// Create child table with FK to countries
	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// cities should have a many-to-one relationship to countries
	rels, err := cache.GetRelationships("cities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rels) == 0 {
		t.Fatal("expected at least one relationship for cities")
	}

	// Find the many-to-one relationship
	var found bool
	for _, rel := range rels {
		if rel.Type == "many-to-one" && rel.ForeignTable == "countries" {
			found = true
			if rel.Name != "countries" {
				t.Errorf("expected Name 'countries', got %q", rel.Name)
			}
			if rel.LocalColumn != "country_id" {
				t.Errorf("expected LocalColumn 'country_id', got %q", rel.LocalColumn)
			}
			if rel.ForeignColumn != "id" {
				t.Errorf("expected ForeignColumn 'id', got %q", rel.ForeignColumn)
			}
		}
	}

	if !found {
		t.Error("expected to find many-to-one relationship from cities to countries")
	}
}

func TestGetRelationships_OneToMany(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create parent table
	_, err := db.Exec(`CREATE TABLE countries (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	// Create child table with FK to countries
	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// countries should have a one-to-many relationship to cities
	rels, err := cache.GetRelationships("countries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rels) == 0 {
		t.Fatal("expected at least one relationship for countries")
	}

	// Find the one-to-many relationship
	var found bool
	for _, rel := range rels {
		if rel.Type == "one-to-many" && rel.ForeignTable == "cities" {
			found = true
			if rel.Name != "cities" {
				t.Errorf("expected Name 'cities', got %q", rel.Name)
			}
			if rel.LocalColumn != "id" {
				t.Errorf("expected LocalColumn 'id', got %q", rel.LocalColumn)
			}
			if rel.ForeignColumn != "country_id" {
				t.Errorf("expected ForeignColumn 'country_id', got %q", rel.ForeignColumn)
			}
		}
	}

	if !found {
		t.Error("expected to find one-to-many relationship from countries to cities")
	}
}

func TestGetRelationships_MultipleFKs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create tables
	_, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create categories table: %v", err)
	}

	// Table with multiple FKs
	_, err = db.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		author_id INTEGER REFERENCES users(id),
		category_id INTEGER REFERENCES categories(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create posts table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// posts should have two many-to-one relationships
	rels, err := cache.GetRelationships("posts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manyToOneCount := 0
	hasUsersRel := false
	hasCategoriesRel := false

	for _, rel := range rels {
		if rel.Type == "many-to-one" {
			manyToOneCount++
			if rel.ForeignTable == "users" {
				hasUsersRel = true
				if rel.LocalColumn != "author_id" {
					t.Errorf("expected LocalColumn 'author_id' for users rel, got %q", rel.LocalColumn)
				}
			}
			if rel.ForeignTable == "categories" {
				hasCategoriesRel = true
				if rel.LocalColumn != "category_id" {
					t.Errorf("expected LocalColumn 'category_id' for categories rel, got %q", rel.LocalColumn)
				}
			}
		}
	}

	if manyToOneCount != 2 {
		t.Errorf("expected 2 many-to-one relationships, got %d", manyToOneCount)
	}
	if !hasUsersRel {
		t.Error("expected to find relationship to users")
	}
	if !hasCategoriesRel {
		t.Error("expected to find relationship to categories")
	}
}

func TestGetRelationships_Caching(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// First call should populate cache
	rels1, err := cache.GetRelationships("cities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should use cache
	rels2, err := cache.GetRelationships("cities")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	// Results should be the same
	if len(rels1) != len(rels2) {
		t.Errorf("cached results differ: got %d vs %d", len(rels1), len(rels2))
	}
}

func TestGetRelationships_NoForeignKeys(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE standalone (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	cache := NewRelationshipCache(db)

	rels, err := cache.GetRelationships("standalone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rels) != 0 {
		t.Errorf("expected no relationships, got %d", len(rels))
	}
}

func TestInvalidateCache(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// Populate cache
	_, err = cache.GetRelationships("cities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache is populated
	cache.mu.RLock()
	_, cached := cache.cache["cities"]
	cache.mu.RUnlock()
	if !cached {
		t.Error("expected cities to be cached")
	}

	// Invalidate specific table
	cache.InvalidateCache("cities")

	cache.mu.RLock()
	_, cached = cache.cache["cities"]
	cache.mu.RUnlock()
	if cached {
		t.Error("expected cities cache to be invalidated")
	}
}

func TestInvalidateCache_All(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// Populate cache for both tables
	_, _ = cache.GetRelationships("cities")
	_, _ = cache.GetRelationships("countries")

	// Invalidate all
	cache.InvalidateCache("")

	cache.mu.RLock()
	cacheSize := len(cache.cache)
	cache.mu.RUnlock()

	if cacheSize != 0 {
		t.Errorf("expected empty cache, got %d entries", cacheSize)
	}
}

func TestFindRelationship(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// Find existing relationship
	rel, err := cache.FindRelationship("cities", "countries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected to find relationship")
	}
	if rel.Type != "many-to-one" {
		t.Errorf("expected many-to-one, got %q", rel.Type)
	}

	// Find non-existent relationship
	rel, err = cache.FindRelationship("cities", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Error("expected nil for non-existent relationship")
	}
}

func TestGetRelationships_CompositeForeignKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a table with composite primary key
	_, err := db.Exec(`CREATE TABLE order_items (
		order_id INTEGER,
		product_id INTEGER,
		quantity INTEGER,
		PRIMARY KEY (order_id, product_id)
	)`)
	if err != nil {
		t.Fatalf("failed to create order_items table: %v", err)
	}

	// Create a table referencing the composite key
	// Note: SQLite handles this as separate FK entries with the same id
	_, err = db.Exec(`CREATE TABLE item_notes (
		id INTEGER PRIMARY KEY,
		order_id INTEGER,
		product_id INTEGER,
		note TEXT,
		FOREIGN KEY (order_id, product_id) REFERENCES order_items(order_id, product_id)
	)`)
	if err != nil {
		t.Fatalf("failed to create item_notes table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// item_notes should have relationship(s) to order_items
	rels, err := cache.GetRelationships("item_notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find at least one relationship to order_items
	found := false
	for _, rel := range rels {
		if rel.ForeignTable == "order_items" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find relationship to order_items")
	}
}

func TestGetRelationships_SelfReference(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a self-referencing table (e.g., employees with manager)
	_, err := db.Exec(`CREATE TABLE employees (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		manager_id INTEGER REFERENCES employees(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create employees table: %v", err)
	}

	cache := NewRelationshipCache(db)

	rels, err := cache.GetRelationships("employees")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have both a many-to-one (to parent) and one-to-many (to children)
	manyToOne := 0
	oneToMany := 0
	for _, rel := range rels {
		if rel.ForeignTable == "employees" {
			if rel.Type == "many-to-one" {
				manyToOne++
			} else if rel.Type == "one-to-many" {
				oneToMany++
			}
		}
	}

	if manyToOne != 1 {
		t.Errorf("expected 1 many-to-one self-reference, got %d", manyToOne)
	}
	if oneToMany != 1 {
		t.Errorf("expected 1 one-to-many self-reference, got %d", oneToMany)
	}
}

func TestGetRelationships_ThreadSafety(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// Run concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, err := cache.GetRelationships("cities")
				if err != nil {
					t.Errorf("concurrent read failed: %v", err)
				}
				_, err = cache.GetRelationships("countries")
				if err != nil {
					t.Errorf("concurrent read failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Also run some cache invalidations
	go func() {
		for i := 0; i < 50; i++ {
			cache.InvalidateCache("cities")
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}
}

func TestIsValidTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple name", "users", true},
		{"valid with underscore", "user_accounts", true},
		{"valid with numbers", "users2", true},
		{"valid mixed", "user_accounts_v2", true},
		{"valid uppercase", "Users", true},
		{"valid mixed case", "UserAccounts", true},
		{"empty string", "", false},
		{"SQL injection with quotes", "users'; DROP TABLE users; --", false},
		{"SQL injection with parentheses", "users())", false},
		{"contains space", "user accounts", false},
		{"contains hyphen", "user-accounts", false},
		{"contains dot", "schema.users", false},
		{"contains semicolon", "users;", false},
		{"contains single quote", "users'", false},
		{"contains double quote", "users\"", false},
		{"starts with number", "2users", true}, // valid identifier in SQLite
		{"unicode letters", "utilisateurs", true},
		{"unicode accents", "usersé", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTableName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidTableName(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetRelationships_SQLInjectionPrevention(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewRelationshipCache(db)

	// Test various SQL injection attempts
	injectionAttempts := []string{
		"users'); DROP TABLE users; --",
		"users' OR '1'='1",
		"users; DELETE FROM users",
		"users); SELECT * FROM sqlite_master; --",
		"users' UNION SELECT * FROM sqlite_master --",
	}

	for _, attempt := range injectionAttempts {
		t.Run(attempt, func(t *testing.T) {
			_, err := cache.GetRelationships(attempt)
			if err == nil {
				t.Errorf("expected error for SQL injection attempt %q, got nil", attempt)
			}
			// Verify the error message indicates invalid table name
			expectedErr := "invalid table name"
			if err != nil && !contains(err.Error(), expectedErr) {
				t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
