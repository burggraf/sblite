// internal/rest/relation_query_test.go
package rest

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupRelationTestDB creates an in-memory SQLite database with test tables for relations.
func setupRelationTestDB(t *testing.T) *sql.DB {
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

// setupCountryCitySchema creates countries and cities tables with FK relationship.
func setupCountryCitySchema(t *testing.T, db *sql.DB) {
	t.Helper()

	// Create countries table
	_, err := db.Exec(`CREATE TABLE countries (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		code TEXT
	)`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	// Create cities table with FK to countries
	_, err = db.Exec(`CREATE TABLE cities (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		population INTEGER,
		country_id INTEGER REFERENCES countries(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}
}

// insertCountryCityData inserts test data.
func insertCountryCityData(t *testing.T, db *sql.DB) {
	t.Helper()

	// Insert countries
	_, err := db.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'United States', 'US')`)
	if err != nil {
		t.Fatalf("failed to insert country: %v", err)
	}
	_, err = db.Exec(`INSERT INTO countries (id, name, code) VALUES (2, 'Canada', 'CA')`)
	if err != nil {
		t.Fatalf("failed to insert country: %v", err)
	}
	_, err = db.Exec(`INSERT INTO countries (id, name, code) VALUES (3, 'Germany', 'DE')`)
	if err != nil {
		t.Fatalf("failed to insert country: %v", err)
	}

	// Insert cities
	_, err = db.Exec(`INSERT INTO cities (id, name, population, country_id) VALUES (1, 'New York', 8336817, 1)`)
	if err != nil {
		t.Fatalf("failed to insert city: %v", err)
	}
	_, err = db.Exec(`INSERT INTO cities (id, name, population, country_id) VALUES (2, 'Los Angeles', 3979576, 1)`)
	if err != nil {
		t.Fatalf("failed to insert city: %v", err)
	}
	_, err = db.Exec(`INSERT INTO cities (id, name, population, country_id) VALUES (3, 'Toronto', 2731571, 2)`)
	if err != nil {
		t.Fatalf("failed to insert city: %v", err)
	}
	_, err = db.Exec(`INSERT INTO cities (id, name, population, country_id) VALUES (4, 'Berlin', 3644826, 3)`)
	if err != nil {
		t.Fatalf("failed to insert city: %v", err)
	}
}

func TestNewRelationQueryExecutor(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
	if exec.db != db {
		t.Error("expected executor to store db reference")
	}
	if exec.relCache != cache {
		t.Error("expected executor to store cache reference")
	}
}

func TestManyToOneRelation(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query: cities with country(name)
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Verify each city has embedded country
	for _, city := range results {
		country, ok := city["countries"]
		if !ok {
			t.Errorf("city %v missing 'countries' field", city["name"])
			continue
		}
		if country == nil {
			t.Errorf("city %v has nil country", city["name"])
			continue
		}

		countryMap, ok := country.(map[string]any)
		if !ok {
			t.Errorf("country should be map[string]any, got %T", country)
			continue
		}

		if _, hasName := countryMap["name"]; !hasName {
			t.Errorf("country missing 'name' field")
		}
	}

	// Verify specific city-country relationships
	nyCity := findByName(results, "New York")
	if nyCity == nil {
		t.Fatal("could not find New York")
	}
	nyCountry := nyCity["countries"].(map[string]any)
	if nyCountry["name"] != "United States" {
		t.Errorf("expected New York's country to be 'United States', got %v", nyCountry["name"])
	}
}

func TestManyToOneRelationWithAlias(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query: cities with aliased country
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, homeland:countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	// Verify alias is used
	for _, city := range results {
		if _, hasHomeland := city["homeland"]; !hasHomeland {
			t.Errorf("city %v missing 'homeland' field (alias)", city["name"])
		}
		if _, hasCountries := city["countries"]; hasCountries {
			t.Error("should not have 'countries' field when alias is used")
		}
	}
}

func TestOneToManyRelation(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query: countries with cities(name)
	q := Query{Table: "countries", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, cities(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify each country has embedded cities array
	for _, country := range results {
		cities, ok := country["cities"]
		if !ok {
			t.Errorf("country %v missing 'cities' field", country["name"])
			continue
		}

		citiesArr, ok := cities.([]map[string]any)
		if !ok {
			t.Errorf("cities should be []map[string]any, got %T", cities)
			continue
		}

		// US should have 2 cities
		if country["name"] == "United States" && len(citiesArr) != 2 {
			t.Errorf("expected US to have 2 cities, got %d", len(citiesArr))
		}

		// Canada should have 1 city
		if country["name"] == "Canada" && len(citiesArr) != 1 {
			t.Errorf("expected Canada to have 1 city, got %d", len(citiesArr))
		}

		// Germany should have 1 city
		if country["name"] == "Germany" && len(citiesArr) != 1 {
			t.Errorf("expected Germany to have 1 city, got %d", len(citiesArr))
		}
	}
}

func TestOneToManyRelationEmpty(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)

	// Insert a country with no cities
	_, err := db.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Empty Country', 'EC')`)
	if err != nil {
		t.Fatalf("failed to insert country: %v", err)
	}

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	q := Query{Table: "countries", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, cities(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Should have empty array, not nil
	cities := results[0]["cities"]
	citiesArr, ok := cities.([]map[string]any)
	if !ok {
		t.Fatalf("cities should be []map[string]any, got %T", cities)
	}
	if len(citiesArr) != 0 {
		t.Errorf("expected empty array, got %d items", len(citiesArr))
	}
}

func TestTwoLevelNesting(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	// Create continents -> countries -> cities schema
	_, err := db.Exec(`CREATE TABLE continents (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create continents table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE countries (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		continent_id INTEGER REFERENCES continents(id)
	)`)
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

	// Insert test data
	_, _ = db.Exec(`INSERT INTO continents (id, name) VALUES (1, 'North America')`)
	_, _ = db.Exec(`INSERT INTO continents (id, name) VALUES (2, 'Europe')`)
	_, _ = db.Exec(`INSERT INTO countries (id, name, continent_id) VALUES (1, 'United States', 1)`)
	_, _ = db.Exec(`INSERT INTO countries (id, name, continent_id) VALUES (2, 'Germany', 2)`)
	_, _ = db.Exec(`INSERT INTO cities (id, name, country_id) VALUES (1, 'New York', 1)`)
	_, _ = db.Exec(`INSERT INTO cities (id, name, country_id) VALUES (2, 'Berlin', 2)`)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query: cities with country(name, continent(name))
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, countries(name, continents(name))")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Find New York and verify nested structure
	ny := findByName(results, "New York")
	if ny == nil {
		t.Fatal("could not find New York")
	}

	country := ny["countries"].(map[string]any)
	if country["name"] != "United States" {
		t.Errorf("expected 'United States', got %v", country["name"])
	}

	continent := country["continents"].(map[string]any)
	if continent["name"] != "North America" {
		t.Errorf("expected 'North America', got %v", continent["name"])
	}

	// Find Berlin and verify
	berlin := findByName(results, "Berlin")
	if berlin == nil {
		t.Fatal("could not find Berlin")
	}

	country = berlin["countries"].(map[string]any)
	if country["name"] != "Germany" {
		t.Errorf("expected 'Germany', got %v", country["name"])
	}

	continent = country["continents"].(map[string]any)
	if continent["name"] != "Europe" {
		t.Errorf("expected 'Europe', got %v", continent["name"])
	}
}

func TestManyToOneWithNullFK(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)

	// Insert a city with NULL country_id
	_, err := db.Exec(`INSERT INTO cities (id, name, population, country_id) VALUES (1, 'Stateless City', 0, NULL)`)
	if err != nil {
		t.Fatalf("failed to insert city: %v", err)
	}

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Should have null for countries
	if results[0]["countries"] != nil {
		t.Errorf("expected nil for countries, got %v", results[0]["countries"])
	}
}

func TestManyToOneSelectSpecificColumns(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Request only specific columns from related table
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("name, countries(name, code)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	// Verify country has both requested fields
	ny := findByName(results, "New York")
	if ny == nil {
		t.Fatal("could not find New York")
	}

	country := ny["countries"].(map[string]any)
	if _, hasName := country["name"]; !hasName {
		t.Error("country missing 'name' field")
	}
	if _, hasCode := country["code"]; !hasCode {
		t.Error("country missing 'code' field")
	}
}

func TestRelationWithFilters(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query with filter
	q := Query{
		Table:  "cities",
		Select: []string{"*"},
		Filters: []Filter{
			{Column: "country_id", Operator: "eq", Value: "1"},
		},
	}
	parsed, err := ParseSelectString("id, name, countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	// Should only get US cities
	if len(results) != 2 {
		t.Fatalf("expected 2 results (US cities), got %d", len(results))
	}

	for _, city := range results {
		country := city["countries"].(map[string]any)
		if country["name"] != "United States" {
			t.Errorf("expected all cities to be from US, got %v", country["name"])
		}
	}
}

func TestRelationWithOrderAndLimit(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query with order and limit
	q := Query{
		Table:  "cities",
		Select: []string{"*"},
		Order:  []OrderBy{{Column: "name", Desc: false}},
		Limit:  2,
	}
	parsed, err := ParseSelectString("id, name, countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First should be Berlin (alphabetically)
	if results[0]["name"] != "Berlin" {
		t.Errorf("expected first city to be 'Berlin', got %v", results[0]["name"])
	}
}

func TestNonExistentRelation(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Try to get a relation that doesn't exist
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, nonexistent(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("should not error for non-existent relation: %v", err)
	}

	// Should have null for the non-existent relation
	for _, city := range results {
		if city["nonexistent"] != nil {
			t.Errorf("expected nil for non-existent relation, got %v", city["nonexistent"])
		}
	}
}

func TestEmptyResults(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	// Don't insert any data

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, countries(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMultipleRelations(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	// Create schema with multiple FKs
	_, _ = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	_, _ = db.Exec(`CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT)`)
	_, _ = db.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY,
		title TEXT,
		author_id INTEGER REFERENCES users(id),
		category_id INTEGER REFERENCES categories(id)
	)`)

	// Insert test data
	_, _ = db.Exec(`INSERT INTO users (id, name) VALUES (1, 'Alice')`)
	_, _ = db.Exec(`INSERT INTO categories (id, name) VALUES (1, 'Tech')`)
	_, _ = db.Exec(`INSERT INTO posts (id, title, author_id, category_id) VALUES (1, 'Hello World', 1, 1)`)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query with multiple relations
	q := Query{Table: "posts", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, title, users(name), categories(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	post := results[0]

	// Check users relation
	user, ok := post["users"].(map[string]any)
	if !ok || user == nil {
		t.Fatal("missing users relation")
	}
	if user["name"] != "Alice" {
		t.Errorf("expected user 'Alice', got %v", user["name"])
	}

	// Check categories relation
	category, ok := post["categories"].(map[string]any)
	if !ok || category == nil {
		t.Fatal("missing categories relation")
	}
	if category["name"] != "Tech" {
		t.Errorf("expected category 'Tech', got %v", category["name"])
	}
}

func TestSelfReferencingRelation(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	// Create self-referencing table
	_, err := db.Exec(`CREATE TABLE employees (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		manager_id INTEGER REFERENCES employees(id)
	)`)
	if err != nil {
		t.Fatalf("failed to create employees table: %v", err)
	}

	// Insert test data
	_, _ = db.Exec(`INSERT INTO employees (id, name, manager_id) VALUES (1, 'CEO', NULL)`)
	_, _ = db.Exec(`INSERT INTO employees (id, name, manager_id) VALUES (2, 'Manager', 1)`)
	_, _ = db.Exec(`INSERT INTO employees (id, name, manager_id) VALUES (3, 'Developer', 2)`)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query: employees with their manager (many-to-one self-reference)
	q := Query{Table: "employees", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, employees(name)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Find Developer and verify manager
	dev := findByName(results, "Developer")
	if dev == nil {
		t.Fatal("could not find Developer")
	}

	// Developer's manager should be Manager
	manager, ok := dev["employees"].(map[string]any)
	if !ok {
		t.Fatal("Developer should have employees relation")
	}
	if manager["name"] != "Manager" {
		t.Errorf("expected Developer's manager to be 'Manager', got %v", manager["name"])
	}

	// CEO should have null manager
	ceo := findByName(results, "CEO")
	if ceo == nil {
		t.Fatal("could not find CEO")
	}
	if ceo["employees"] != nil {
		t.Errorf("CEO should have null manager, got %v", ceo["employees"])
	}
}

func TestRelationWithStarSelect(t *testing.T) {
	db := setupRelationTestDB(t)
	defer db.Close()

	setupCountryCitySchema(t, db)
	insertCountryCityData(t, db)

	cache := NewRelationshipCache(db)
	exec := NewRelationQueryExecutor(db, cache)

	// Query with star in relation
	q := Query{Table: "cities", Select: []string{"*"}}
	parsed, err := ParseSelectString("id, name, countries(*)")
	if err != nil {
		t.Fatalf("failed to parse select: %v", err)
	}

	results, err := exec.ExecuteWithRelations(q, parsed)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	// Verify country has all fields
	ny := findByName(results, "New York")
	if ny == nil {
		t.Fatal("could not find New York")
	}

	country := ny["countries"].(map[string]any)
	if _, hasId := country["id"]; !hasId {
		t.Error("country missing 'id' field")
	}
	if _, hasName := country["name"]; !hasName {
		t.Error("country missing 'name' field")
	}
	if _, hasCode := country["code"]; !hasCode {
		t.Error("country missing 'code' field")
	}
}

// Helper function to find a row by name field
func findByName(results []map[string]any, name string) map[string]any {
	for _, row := range results {
		if row["name"] == name {
			return row
		}
	}
	return nil
}

// Test helper functions
func TestExtractColumnNames(t *testing.T) {
	cols := []SelectColumn{
		{Name: "id"},
		{Name: "name"},
		{Relation: &SelectRelation{Name: "country"}},
		{Name: "email"},
	}

	names := extractColumnNames(cols)

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	expected := []string{"id", "name", "email"}
	for i, exp := range expected {
		if names[i] != exp {
			t.Errorf("name %d: expected %s, got %s", i, exp, names[i])
		}
	}
}

func TestEnsureColumnIncluded(t *testing.T) {
	tests := []struct {
		name     string
		cols     []string
		add      string
		expected []string
	}{
		{
			name:     "add new column",
			cols:     []string{"a", "b"},
			add:      "c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "column already present",
			cols:     []string{"a", "b", "c"},
			add:      "b",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "has star",
			cols:     []string{"*"},
			add:      "id",
			expected: []string{"*"},
		},
		{
			name:     "empty list",
			cols:     []string{},
			add:      "id",
			expected: []string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureColumnIncluded(tt.cols, tt.add)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d columns, got %d", len(tt.expected), len(result))
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("column %d: expected %s, got %s", i, exp, result[i])
				}
			}
		})
	}
}

func TestContainsColumn(t *testing.T) {
	cols := []string{"id", "name", "email"}

	if !containsColumn(cols, "name") {
		t.Error("expected to find 'name'")
	}
	if containsColumn(cols, "notfound") {
		t.Error("should not find 'notfound'")
	}
	if containsColumn([]string{}, "anything") {
		t.Error("empty list should not contain anything")
	}
}
