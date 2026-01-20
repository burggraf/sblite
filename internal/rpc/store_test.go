// internal/rpc/store_test.go
package rpc

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Force single connection for in-memory database
	// (each connection to :memory: creates a separate database)
	db.SetMaxOpenConns(1)

	// Enable foreign keys
	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE _rpc_functions (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			language TEXT NOT NULL DEFAULT 'sql',
			return_type TEXT NOT NULL,
			returns_set INTEGER NOT NULL DEFAULT 0,
			volatility TEXT DEFAULT 'VOLATILE',
			security TEXT DEFAULT 'INVOKER',
			source_pg TEXT NOT NULL,
			source_sqlite TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		);
		CREATE TABLE _rpc_function_args (
			id TEXT PRIMARY KEY,
			function_id TEXT NOT NULL REFERENCES _rpc_functions(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			position INTEGER NOT NULL,
			default_value TEXT,
			UNIQUE(function_id, position)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

func TestStore_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "get_user",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		Volatility:   "STABLE",
		Security:     "INVOKER",
		SourcePG:     "SELECT id, email FROM users WHERE id = user_id",
		SourceSQLite: "SELECT id, email FROM users WHERE id = ?",
		Args: []FunctionArg{
			{Name: "user_id", Type: "uuid", Position: 0},
		},
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it was created
	got, err := store.Get("get_user")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Name != "get_user" {
		t.Errorf("Name = %q, want %q", got.Name, "get_user")
	}
	if len(got.Args) != 1 {
		t.Errorf("Args len = %d, want 1", len(got.Args))
	}
}

func TestStore_CreateOrReplace(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "my_func",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT 1",
		SourceSQLite: "SELECT 1",
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update with CreateOrReplace
	def.SourcePG = "SELECT 2"
	def.SourceSQLite = "SELECT 2"
	err = store.CreateOrReplace(def)
	if err != nil {
		t.Fatalf("CreateOrReplace failed: %v", err)
	}

	got, _ := store.Get("my_func")
	if got.SourcePG != "SELECT 2" {
		t.Errorf("SourcePG = %q, want %q", got.SourcePG, "SELECT 2")
	}
}

func TestStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "to_delete",
		Language:     "sql",
		ReturnType:   "void",
		SourcePG:     "SELECT 1",
		SourceSQLite: "SELECT 1",
	}

	store.Create(def)

	err := store.Delete("to_delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("to_delete")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestStore_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	err := store.Create(&FunctionDef{Name: "func1", Language: "sql", ReturnType: "int", SourcePG: "1", SourceSQLite: "1"})
	if err != nil {
		t.Fatalf("Create func1 failed: %v", err)
	}
	err = store.Create(&FunctionDef{Name: "func2", Language: "sql", ReturnType: "int", SourcePG: "2", SourceSQLite: "2"})
	if err != nil {
		t.Fatalf("Create func2 failed: %v", err)
	}

	funcs, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Errorf("List returned %d funcs, want 2", len(funcs))
	}
}

func TestStore_Exists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	if store.Exists("nonexistent") {
		t.Error("Exists returned true for nonexistent function")
	}

	store.Create(&FunctionDef{Name: "exists_test", Language: "sql", ReturnType: "int", SourcePG: "1", SourceSQLite: "1"})

	if !store.Exists("exists_test") {
		t.Error("Exists returned false for existing function")
	}
}

func TestStore_CreateDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "duplicate",
		Language:     "sql",
		ReturnType:   "int",
		SourcePG:     "SELECT 1",
		SourceSQLite: "SELECT 1",
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	// Try to create again - should fail
	err = store.Create(def)
	if err == nil {
		t.Error("expected error when creating duplicate, got nil")
	}
}

func TestStore_DeleteNonexistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent function, got nil")
	}
}

func TestStore_GetWithMultipleArgs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	def := &FunctionDef{
		Name:         "multi_arg",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT $1 + $2",
		SourceSQLite: "SELECT ? + ?",
		Args: []FunctionArg{
			{Name: "a", Type: "integer", Position: 0},
			{Name: "b", Type: "integer", Position: 1},
		},
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get("multi_arg")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(got.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(got.Args))
	}

	// Verify args are in order
	if got.Args[0].Name != "a" || got.Args[0].Position != 0 {
		t.Errorf("Arg[0] = %+v, want name=a, position=0", got.Args[0])
	}
	if got.Args[1].Name != "b" || got.Args[1].Position != 1 {
		t.Errorf("Arg[1] = %+v, want name=b, position=1", got.Args[1])
	}
}

func TestStore_ArgWithDefaultValue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	store := NewStore(db)

	defaultVal := "10"
	def := &FunctionDef{
		Name:         "with_default",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT $1",
		SourceSQLite: "SELECT ?",
		Args: []FunctionArg{
			{Name: "val", Type: "integer", Position: 0, DefaultValue: &defaultVal},
		},
	}

	err := store.Create(def)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get("with_default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Args[0].DefaultValue == nil {
		t.Error("DefaultValue is nil, want non-nil")
	} else if *got.Args[0].DefaultValue != "10" {
		t.Errorf("DefaultValue = %q, want %q", *got.Args[0].DefaultValue, "10")
	}
}
