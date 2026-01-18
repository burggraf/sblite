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

func TestIsJunctionTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create tables for M2M relationship: users <-> user_roles <-> roles
	_, err := db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create roles table: %v", err)
	}

	// Junction table with composite PK containing both FKs
	_, err = db.Exec(`
		CREATE TABLE user_roles (
			user_id TEXT REFERENCES users(id),
			role_id TEXT REFERENCES roles(id),
			PRIMARY KEY (user_id, role_id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create user_roles table: %v", err)
	}

	cache := NewRelationshipCache(db)

	// Test that user_roles is detected as a junction table
	isJunction, table1, table2, err := cache.isJunctionTable("user_roles")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isJunction {
		t.Error("user_roles should be detected as a junction table")
	}
	// Check both tables are detected (order may vary)
	if (table1 != "users" && table1 != "roles") || (table2 != "users" && table2 != "roles") {
		t.Errorf("expected tables users and roles, got %s and %s", table1, table2)
	}
	if table1 == table2 {
		t.Error("junction table should connect two different tables")
	}

	// Test that regular tables are not junction tables
	isJunction, _, _, err = cache.isJunctionTable("users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isJunction {
		t.Error("users should not be a junction table")
	}
}

func TestIsJunctionTable_NotJunction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a table with 2 FKs but they're not both in the PK
	_, err := db.Exec(`CREATE TABLE countries (id TEXT PRIMARY KEY)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE cities (id TEXT PRIMARY KEY)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE city_links (
			id TEXT PRIMARY KEY,
			country_id TEXT REFERENCES countries(id),
			city_id TEXT REFERENCES cities(id)
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	cache := NewRelationshipCache(db)

	// city_links has 2 FKs but they're not both in the PK
	isJunction, _, _, err := cache.isJunctionTable("city_links")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isJunction {
		t.Error("city_links should NOT be a junction table (FKs not in PK)")
	}
}

func TestFindM2MRelationship(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create M2M relationship: users <-> user_roles <-> roles
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE roles (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`
		CREATE TABLE user_roles (
			user_id TEXT REFERENCES users(id),
			role_id TEXT REFERENCES roles(id),
			PRIMARY KEY (user_id, role_id)
		)
	`)

	cache := NewRelationshipCache(db)

	// Find M2M from users to roles
	info, err := cache.FindM2MRelationship("users", "roles")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected to find M2M relationship")
	}

	if info.JunctionTable != "user_roles" {
		t.Errorf("junction table: expected 'user_roles', got '%s'", info.JunctionTable)
	}
	if info.SourceColumn != "user_id" {
		t.Errorf("source column: expected 'user_id', got '%s'", info.SourceColumn)
	}
	if info.TargetColumn != "role_id" {
		t.Errorf("target column: expected 'role_id', got '%s'", info.TargetColumn)
	}

	// Find M2M from roles to users (reverse direction)
	info, err = cache.FindM2MRelationship("roles", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected to find M2M relationship in reverse direction")
	}
	if info.JunctionTable != "user_roles" {
		t.Errorf("junction table: expected 'user_roles', got '%s'", info.JunctionTable)
	}
}

func TestFindM2MRelationship_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create tables with no M2M relationship
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY)`)
	db.Exec(`CREATE TABLE posts (id TEXT PRIMARY KEY)`)

	cache := NewRelationshipCache(db)

	info, err := cache.FindM2MRelationship("users", "posts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected no M2M relationship to be found")
	}
}

func TestFindRelationshipWithHint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create tables with multiple FKs to same table (messages with sender and receiver)
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`
		CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			content TEXT,
			sender_id TEXT REFERENCES users(id),
			receiver_id TEXT REFERENCES users(id)
		)
	`)

	cache := NewRelationshipCache(db)

	// Without hint - should work if only one FK to the table
	// But messages has 2 FKs to users, so let's test with hint

	// Find with sender_id hint
	rel, err := cache.FindRelationshipWithHint("messages", "users", "sender_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected to find relationship with sender_id hint")
	}
	if rel.LocalColumn != "sender_id" {
		t.Errorf("expected LocalColumn 'sender_id', got '%s'", rel.LocalColumn)
	}

	// Find with receiver_id hint
	rel, err = cache.FindRelationshipWithHint("messages", "users", "receiver_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected to find relationship with receiver_id hint")
	}
	if rel.LocalColumn != "receiver_id" {
		t.Errorf("expected LocalColumn 'receiver_id', got '%s'", rel.LocalColumn)
	}

	// Invalid hint should return error with available hints
	_, err = cache.FindRelationshipWithHint("messages", "users", "invalid_column")
	if err == nil {
		t.Error("expected error for invalid hint")
	} else {
		// Error should mention available hints
		if !contains(err.Error(), "sender_id") || !contains(err.Error(), "receiver_id") {
			t.Errorf("error should mention available hints, got: %v", err)
		}
	}
}

func TestFindRelationshipWithHint_EmptyHint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.Exec(`CREATE TABLE countries (id TEXT PRIMARY KEY)`)
	db.Exec(`CREATE TABLE cities (id TEXT PRIMARY KEY, country_id TEXT REFERENCES countries(id))`)

	cache := NewRelationshipCache(db)

	// Empty hint should fall back to FindRelationship
	rel, err := cache.FindRelationshipWithHint("cities", "countries", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected to find relationship with empty hint")
	}
	if rel.Name != "countries" {
		t.Errorf("expected relationship name 'countries', got '%s'", rel.Name)
	}
}
