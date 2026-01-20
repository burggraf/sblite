package pgtranslate

import (
	"database/sql"
	"regexp"
	"testing"

	_ "modernc.org/sqlite"
)

// TestUUIDv4Generation_Integration tests that the generated UUID SQL actually works in SQLite
// and produces valid RFC 4122 UUID v4 values.
func TestUUIDv4Generation_Integration(t *testing.T) {
	// Open in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tr := NewTranslator()

	// Test 1: Generate a UUID using the translated SQL
	pgSQL := "SELECT gen_random_uuid() as id"
	sqliteSQL := tr.Translate(pgSQL)

	var uuid string
	err = db.QueryRow(sqliteSQL).Scan(&uuid)
	if err != nil {
		t.Fatalf("Failed to generate UUID: %v\nSQL: %s", err, sqliteSQL)
	}

	// Validate UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where y is one of [8, 9, a, b]
	uuidv4Pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidv4Pattern.MatchString(uuid) {
		t.Errorf("Generated UUID does not match RFC 4122 v4 format.\nGot: %s\nExpected pattern: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (where y is 8, 9, a, or b)", uuid)
	}

	// Verify specific requirements
	if len(uuid) != 36 {
		t.Errorf("UUID length should be 36, got %d: %s", len(uuid), uuid)
	}

	// Check version field (character at position 14, 0-indexed)
	if uuid[14] != '4' {
		t.Errorf("UUID version should be 4, got %c at position 14: %s", uuid[14], uuid)
	}

	// Check variant field (character at position 19, 0-indexed)
	variantChar := uuid[19]
	if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
		t.Errorf("UUID variant should be 8, 9, a, or b, got %c at position 19: %s", variantChar, uuid)
	}

	// Check hyphens are in the right positions
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Errorf("UUID hyphens are not in correct positions: %s", uuid)
	}

	t.Logf("Generated valid UUID v4: %s", uuid)

	// Test 2: Generate multiple UUIDs and verify they're different (randomness check)
	uuids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		var id string
		err = db.QueryRow(sqliteSQL).Scan(&id)
		if err != nil {
			t.Fatalf("Failed to generate UUID #%d: %v", i, err)
		}

		if uuids[id] {
			t.Errorf("Generated duplicate UUID: %s", id)
		}
		uuids[id] = true

		if !uuidv4Pattern.MatchString(id) {
			t.Errorf("UUID #%d does not match v4 format: %s", i, id)
		}
	}

	// Test 3: Use in CREATE TABLE with DEFAULT
	// NOTE: SQLite doesn't support SELECT subqueries in DEFAULT expressions.
	// The translation produces valid SQL syntax, but it won't work as a DEFAULT.
	// This is a known limitation documented in postgres-translation.md.
	// The workaround is to use gen_random_uuid() in INSERT statements explicitly.
	t.Log("Skipping CREATE TABLE DEFAULT test - SQLite doesn't support SELECT in DEFAULT expressions")
	t.Log("Use gen_random_uuid() in INSERT statements or use triggers for auto-generation")

	// Create table without the computed default
	_, err = db.Exec("CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test 4: Use in INSERT statement (the supported approach)
	insertPG := "INSERT INTO users (id, name) VALUES (gen_random_uuid(), 'Alice')"
	insertSQLite := tr.Translate(insertPG)

	_, err = db.Exec(insertSQLite)
	if err != nil {
		t.Fatalf("Failed to insert with explicit UUID: %v\nSQL: %s", err, insertSQLite)
	}

	var aliceID string
	err = db.QueryRow("SELECT id FROM users WHERE name = 'Alice'").Scan(&aliceID)
	if err != nil {
		t.Fatalf("Failed to query Alice's row: %v", err)
	}

	if !uuidv4Pattern.MatchString(aliceID) {
		t.Errorf("INSERT-generated UUID does not match v4 format: %s", aliceID)
	}

	t.Logf("INSERT-generated UUID for Alice: %s", aliceID)

	// Test 5: Insert another row to verify different UUIDs are generated
	insertPG2 := "INSERT INTO users (id, name) VALUES (gen_random_uuid(), 'Bob')"
	insertSQLite2 := tr.Translate(insertPG2)

	_, err = db.Exec(insertSQLite2)
	if err != nil {
		t.Fatalf("Failed to insert Bob: %v\nSQL: %s", err, insertSQLite2)
	}

	var bobID string
	err = db.QueryRow("SELECT id FROM users WHERE name = 'Bob'").Scan(&bobID)
	if err != nil {
		t.Fatalf("Failed to query Bob's row: %v", err)
	}

	if !uuidv4Pattern.MatchString(bobID) {
		t.Errorf("INSERT-generated UUID does not match v4 format: %s", bobID)
	}

	if aliceID == bobID {
		t.Errorf("Alice and Bob have the same UUID: %s", aliceID)
	}

	t.Logf("INSERT-generated UUID for Bob: %s", bobID)
}

// TestUUIDv4Uniqueness tests that generated UUIDs are sufficiently random
func TestUUIDv4Uniqueness(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tr := NewTranslator()
	sqliteSQL := tr.Translate("SELECT gen_random_uuid()")

	// Generate 1000 UUIDs and check for duplicates
	uuids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		var id string
		err = db.QueryRow(sqliteSQL).Scan(&id)
		if err != nil {
			t.Fatalf("Failed to generate UUID #%d: %v", i, err)
		}

		if uuids[id] {
			t.Fatalf("Generated duplicate UUID at iteration %d: %s", i, id)
		}
		uuids[id] = true
	}

	t.Logf("Successfully generated 1000 unique UUIDs")
}

// BenchmarkUUIDv4Generation benchmarks UUID generation performance
func BenchmarkUUIDv4Generation(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tr := NewTranslator()
	sqliteSQL := tr.Translate("SELECT gen_random_uuid()")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var id string
		err = db.QueryRow(sqliteSQL).Scan(&id)
		if err != nil {
			b.Fatalf("Failed to generate UUID: %v", err)
		}
	}
}
