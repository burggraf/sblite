// internal/rpc/executor_test.go
package rpc

import (
	"database/sql"
	"testing"
)

func setupExecutorTestDB(t *testing.T) *sql.DB {
	db := setupTestDB(t)

	// Create test table
	_, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, active INTEGER DEFAULT 1)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, email) VALUES ('u1', 'a@test.com'), ('u2', 'b@test.com')`)
	if err != nil {
		t.Fatalf("insert data: %v", err)
	}

	return db
}

func TestExecutor_ScalarReturn(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "count_users",
		Language:     "sql",
		ReturnType:   "integer",
		SourcePG:     "SELECT count(*) FROM users",
		SourceSQLite: "SELECT count(*) FROM users",
	})

	result, err := exec.Execute("count_users", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsScalar {
		t.Error("expected scalar result")
	}
	// Result should be 2
	if result.Data != int64(2) {
		t.Errorf("Data = %v, want 2", result.Data)
	}
}

func TestExecutor_TableReturn(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "get_all_users",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		SourcePG:     "SELECT id, email FROM users",
		SourceSQLite: "SELECT id, email FROM users",
	})

	result, err := exec.Execute("get_all_users", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.IsSet {
		t.Error("expected set result")
	}
	rows, ok := result.Data.([]map[string]interface{})
	if !ok {
		t.Fatalf("Data type = %T, want []map[string]interface{}", result.Data)
	}
	if len(rows) != 2 {
		t.Errorf("len(rows) = %d, want 2", len(rows))
	}
}

func TestExecutor_WithParameter(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "get_user_by_id",
		Language:     "sql",
		ReturnType:   "TABLE(id TEXT, email TEXT)",
		ReturnsSet:   true,
		SourcePG:     "SELECT id, email FROM users WHERE id = user_id",
		SourceSQLite: "SELECT id, email FROM users WHERE id = :user_id",
		Args:         []FunctionArg{{Name: "user_id", Type: "text", Position: 0}},
	})

	result, err := exec.Execute("get_user_by_id", map[string]interface{}{"user_id": "u1"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	rows := result.Data.([]map[string]interface{})
	if len(rows) != 1 {
		t.Errorf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0]["email"] != "a@test.com" {
		t.Errorf("email = %v, want a@test.com", rows[0]["email"])
	}
}

func TestExecutor_MissingRequiredArg(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	store.Create(&FunctionDef{
		Name:         "needs_arg",
		Language:     "sql",
		ReturnType:   "integer",
		SourceSQLite: "SELECT 1",
		Args:         []FunctionArg{{Name: "required_arg", Type: "text", Position: 0}},
	})

	_, err := exec.Execute("needs_arg", nil, nil)
	if err == nil {
		t.Error("expected error for missing arg")
	}
}

func TestExecutor_DefaultArg(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	defaultVal := "10"
	store.Create(&FunctionDef{
		Name:         "with_default",
		Language:     "sql",
		ReturnType:   "integer",
		SourceSQLite: "SELECT :limit_val",
		Args:         []FunctionArg{{Name: "limit_val", Type: "integer", Position: 0, DefaultValue: &defaultVal}},
	})

	result, err := exec.Execute("with_default", nil, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Data != "10" {
		t.Errorf("Data = %v, want 10", result.Data)
	}
}

func TestExecutor_FunctionNotFound(t *testing.T) {
	db := setupExecutorTestDB(t)
	defer db.Close()
	store := NewStore(db)
	exec := NewExecutor(db, store)

	_, err := exec.Execute("nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for nonexistent function")
	}
}
