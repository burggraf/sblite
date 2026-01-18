package fts

import (
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

var testCounter int64

func setupTestDB(t *testing.T) *sql.DB {
	// Use a unique temp file for each test to avoid locking issues
	counter := atomic.AddInt64(&testCounter, 1)
	tmpFile := fmt.Sprintf("%s/fts_test_%d_%s.db", os.TempDir(), counter, t.Name())

	db, err := sql.Open("sqlite", tmpFile)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Enable WAL mode for better concurrency
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		t.Fatalf("failed to enable WAL mode: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile)
		os.Remove(tmpFile + "-wal")
		os.Remove(tmpFile + "-shm")
	})

	// Create the _fts_indexes metadata table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS _fts_indexes (
			table_name    TEXT NOT NULL,
			index_name    TEXT NOT NULL,
			columns       TEXT NOT NULL,
			tokenizer     TEXT DEFAULT 'unicode61',
			created_at    TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (table_name, index_name)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create _fts_indexes table: %v", err)
	}

	return db
}

func createTestTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			title TEXT,
			body TEXT,
			author TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
}

func TestCreateIndex(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Test creating a basic FTS index
	err := m.CreateIndex("posts", "search", []string{"title", "body"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Verify the index was created
	idx, err := m.GetIndex("posts", "search")
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}

	if idx.TableName != "posts" {
		t.Errorf("expected table_name 'posts', got %q", idx.TableName)
	}
	if idx.IndexName != "search" {
		t.Errorf("expected index_name 'search', got %q", idx.IndexName)
	}
	if len(idx.Columns) != 2 || idx.Columns[0] != "title" || idx.Columns[1] != "body" {
		t.Errorf("expected columns [title, body], got %v", idx.Columns)
	}
	if idx.Tokenizer != "unicode61" {
		t.Errorf("expected tokenizer 'unicode61', got %q", idx.Tokenizer)
	}
}

func TestCreateIndexWithTokenizer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Test creating an index with porter tokenizer
	err := m.CreateIndex("posts", "search_porter", []string{"title"}, "porter")
	if err != nil {
		t.Fatalf("CreateIndex with porter failed: %v", err)
	}

	idx, err := m.GetIndex("posts", "search_porter")
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}

	if idx.Tokenizer != "porter" {
		t.Errorf("expected tokenizer 'porter', got %q", idx.Tokenizer)
	}
}

func TestCreateIndexErrors(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Test with nonexistent table
	err := m.CreateIndex("nonexistent", "search", []string{"title"}, "")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}

	// Test with nonexistent column
	err = m.CreateIndex("posts", "search", []string{"nonexistent_col"}, "")
	if err == nil {
		t.Error("expected error for nonexistent column")
	}

	// Test with no columns
	err = m.CreateIndex("posts", "search", []string{}, "")
	if err == nil {
		t.Error("expected error for empty columns")
	}

	// Test with invalid tokenizer
	err = m.CreateIndex("posts", "search", []string{"title"}, "invalid_tokenizer")
	if err == nil {
		t.Error("expected error for invalid tokenizer")
	}

	// Create a valid index first, then try to create a duplicate
	err = m.CreateIndex("posts", "search", []string{"title"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	err = m.CreateIndex("posts", "search", []string{"body"}, "")
	if err == nil {
		t.Error("expected error for duplicate index name")
	}
}

func TestDropIndex(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Create an index
	err := m.CreateIndex("posts", "search", []string{"title", "body"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Drop the index
	err = m.DropIndex("posts", "search")
	if err != nil {
		t.Fatalf("DropIndex failed: %v", err)
	}

	// Verify the index is gone
	_, err = m.GetIndex("posts", "search")
	if err == nil {
		t.Error("expected error after dropping index")
	}

	// Dropping non-existent index should error
	err = m.DropIndex("posts", "nonexistent")
	if err == nil {
		t.Error("expected error for dropping non-existent index")
	}
}

func TestListIndexes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// List with no indexes
	indexes, err := m.ListIndexes("posts")
	if err != nil {
		t.Fatalf("ListIndexes failed: %v", err)
	}
	if len(indexes) != 0 {
		t.Errorf("expected 0 indexes, got %d", len(indexes))
	}

	// Create some indexes
	m.CreateIndex("posts", "search1", []string{"title"}, "")
	m.CreateIndex("posts", "search2", []string{"body"}, "")

	indexes, err = m.ListIndexes("posts")
	if err != nil {
		t.Fatalf("ListIndexes failed: %v", err)
	}
	if len(indexes) != 2 {
		t.Errorf("expected 2 indexes, got %d", len(indexes))
	}

	// List all indexes (empty table name)
	indexes, err = m.ListIndexes("")
	if err != nil {
		t.Fatalf("ListIndexes (all) failed: %v", err)
	}
	if len(indexes) != 2 {
		t.Errorf("expected 2 indexes total, got %d", len(indexes))
	}
}

func TestFTSSearchBasic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Insert test data first
	_, err := db.Exec(`
		INSERT INTO posts (id, title, body) VALUES
		(1, 'Hello World', 'This is a test post about programming'),
		(2, 'Go Tutorial', 'Learn Go programming language'),
		(3, 'Python Basics', 'Introduction to Python programming')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Create FTS index
	err = m.CreateIndex("posts", "search", []string{"title", "body"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Test basic search
	ftsTable := GetFTSTableName("posts", "search")
	rows, err := db.Query(`SELECT rowid FROM `+ftsTable+` WHERE `+ftsTable+` MATCH ?`, "programming")
	if err != nil {
		t.Fatalf("FTS query failed: %v", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 matches for 'programming', got %d", len(ids))
	}

	// Test search for specific term
	rows, err = db.Query(`SELECT rowid FROM `+ftsTable+` WHERE `+ftsTable+` MATCH ?`, "Go")
	if err != nil {
		t.Fatalf("FTS query failed: %v", err)
	}
	defer rows.Close()

	ids = nil
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 match for 'Go', got %d", len(ids))
	}
}

func TestFTSSyncTriggers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Create FTS index (this creates sync triggers)
	err := m.CreateIndex("posts", "search", []string{"title", "body"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	ftsTable := GetFTSTableName("posts", "search")

	// Test INSERT trigger - data inserted AFTER index creation should be indexed
	_, err = db.Exec(`INSERT INTO posts (id, title, body) VALUES (1, 'Test Title', 'Test body content')`)
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Test'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 match after INSERT, got %d", count)
	}

	// Test UPDATE trigger
	_, err = db.Exec(`UPDATE posts SET title = 'Updated Title' WHERE id = 1`)
	if err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Updated'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 match for 'Updated' after UPDATE, got %d", count)
	}

	// Old term should no longer match
	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Test'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 1 {
		// Still matches because 'Test' is in body
		t.Logf("'Test' matches %d (expected 1 from body)", count)
	}

	// Test DELETE trigger
	_, err = db.Exec(`DELETE FROM posts WHERE id = 1`)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}

	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Updated'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 matches after DELETE, got %d", count)
	}
}

func TestHasIndex(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Check non-existent index
	has, err := m.HasIndex("posts", "search")
	if err != nil {
		t.Fatalf("HasIndex failed: %v", err)
	}
	if has {
		t.Error("expected HasIndex to return false")
	}

	// Create index
	m.CreateIndex("posts", "search", []string{"title"}, "")

	// Check again
	has, err = m.HasIndex("posts", "search")
	if err != nil {
		t.Fatalf("HasIndex failed: %v", err)
	}
	if !has {
		t.Error("expected HasIndex to return true")
	}
}

func TestFindIndexForColumn(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Create index on title and body
	m.CreateIndex("posts", "search", []string{"title", "body"}, "")

	// Find index for 'title' column
	idx, err := m.FindIndexForColumn("posts", "title")
	if err != nil {
		t.Fatalf("FindIndexForColumn failed: %v", err)
	}
	if idx == nil {
		t.Fatal("expected to find index for 'title' column")
	}
	if idx.IndexName != "search" {
		t.Errorf("expected index name 'search', got %q", idx.IndexName)
	}

	// Find index for non-indexed column
	idx, err = m.FindIndexForColumn("posts", "author")
	if err != nil {
		t.Fatalf("FindIndexForColumn failed: %v", err)
	}
	if idx != nil {
		t.Error("expected nil for non-indexed column")
	}
}

func TestRebuildIndex(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	createTestTable(t, db)

	m := NewManager(db)

	// Insert data before creating index
	_, err := db.Exec(`
		INSERT INTO posts (id, title, body) VALUES
		(1, 'First Post', 'Content one'),
		(2, 'Second Post', 'Content two')
	`)
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	// Create index (this populates with existing data)
	err = m.CreateIndex("posts", "search", []string{"title", "body"}, "")
	if err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	ftsTable := GetFTSTableName("posts", "search")

	// Verify initial data is indexed
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Post'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}

	// Rebuild the index
	err = m.RebuildIndex("posts", "search")
	if err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	// Verify data is still indexed after rebuild
	err = db.QueryRow(`SELECT COUNT(*) FROM ` + ftsTable + ` WHERE ` + ftsTable + ` MATCH 'Post'`).Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 matches after rebuild, got %d", count)
	}
}
