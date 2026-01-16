// internal/rest/handler_test.go
package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
)

func setupTestHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create test table
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0,
			user_id TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("failed to create todos table: %v", err)
	}

	handler := NewHandler(database)
	return handler, database
}

func TestSelectHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 rows, got %d", len(response))
	}
}

func TestSelectWithFilter(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)

	if len(response) != 1 {
		t.Errorf("expected 1 row, got %d", len(response))
	}
}

func TestInsertHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	body := `{"title": "New Todo", "completed": 0}`
	req := httptest.NewRequest("POST", "/rest/v1/todos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (id, title, completed) VALUES (1, 'Test', 0)`)

	r := chi.NewRouter()
	r.Patch("/rest/v1/{table}", handler.HandleUpdate)

	body := `{"completed": 1}`
	req := httptest.NewRequest("PATCH", "/rest/v1/todos?id=eq.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (id, title, completed) VALUES (1, 'Test', 0)`)

	r := chi.NewRouter()
	r.Delete("/rest/v1/{table}", handler.HandleDelete)

	req := httptest.NewRequest("DELETE", "/rest/v1/todos?id=eq.1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}
