// internal/server/server_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	return New(database, "test-secret-key-min-32-characters")
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRESTSelect(t *testing.T) {
	srv := setupTestServer(t)

	// Create test table
	srv.db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0
		)
	`)
	srv.db.Exec(`INSERT INTO todos (title, completed) VALUES ('Test', 0)`)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}
