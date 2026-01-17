package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

// setupTestHandler creates a Handler with a test database and returns the path for cleanup
func setupTestHandler(t *testing.T) (*Handler, string) {
	dbPath := t.TempDir() + "/test.db"
	migrationsDir := t.TempDir() + "/migrations"
	database := setupTestDB(t)
	handler := NewHandler(database.DB, migrationsDir)
	return handler, dbPath
}

// setupTestSession creates a session and returns the token
func setupTestSession(t *testing.T, h *Handler) string {
	// Setup password first
	err := h.auth.SetupPassword("testpassword123")
	require.NoError(t, err)

	// Create session
	token, err := h.sessions.Create()
	require.NoError(t, err)
	return token
}

func TestHandlerServesUI(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "sblite") {
		t.Error("expected body to contain 'sblite'")
	}
}

func TestHandlerServesCSS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/style.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("expected text/css content type, got %s", contentType)
	}
}

func TestHandlerServesJS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/app.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "javascript") {
		t.Errorf("expected javascript content type, got %s", contentType)
	}
}

func TestHandlerAuthStatus(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected application/json content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "needs_setup") {
		t.Error("expected body to contain 'needs_setup'")
	}
	if !strings.Contains(body, "authenticated") {
		t.Error("expected body to contain 'authenticated'")
	}
}

func TestHandlerSetupAndLogin(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Test setup
	setupReq := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(`{"password":"testpassword123"}`))
	setupReq.Header.Set("Content-Type", "application/json")
	setupW := httptest.NewRecorder()

	r.ServeHTTP(setupW, setupReq)

	if setupW.Code != http.StatusOK {
		t.Errorf("setup: expected status 200, got %d", setupW.Code)
	}

	// Check cookie was set
	cookies := setupW.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_sblite_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Error("setup: expected session cookie to be set")
	}

	// Test login with correct password
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"password":"testpassword123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()

	r.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Errorf("login: expected status 200, got %d", loginW.Code)
	}

	// Test login with wrong password
	wrongReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"password":"wrongpassword"}`))
	wrongReq.Header.Set("Content-Type", "application/json")
	wrongW := httptest.NewRecorder()

	r.ServeHTTP(wrongW, wrongReq)

	if wrongW.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: expected status 401, got %d", wrongW.Code)
	}
}

func TestHandlerLogout(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Setup first
	setupReq := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(`{"password":"testpassword123"}`))
	setupReq.Header.Set("Content-Type", "application/json")
	setupW := httptest.NewRecorder()
	r.ServeHTTP(setupW, setupReq)

	// Test logout
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutW := httptest.NewRecorder()

	r.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("logout: expected status 200, got %d", logoutW.Code)
	}

	// Check cookie was cleared
	cookies := logoutW.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_sblite_session" && c.MaxAge != -1 {
			t.Error("logout: expected session cookie to be cleared")
		}
	}
}

func TestHandlerSPARouting(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Test various SPA routes return index.html
	routes := []string{"/tables", "/users", "/settings"}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("route %s: expected status 200, got %d", route, w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("route %s: expected text/html content type, got %s", route, contentType)
		}

		body := w.Body.String()
		if !strings.Contains(body, "sblite") {
			t.Errorf("route %s: expected body to contain 'sblite'", route)
		}
	}
}

func TestHandlerStaticNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/nonexistent.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandlerListTables(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create a test table first
	_, err := h.db.Exec(`CREATE TABLE test_items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	// Register in schema
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('test_items', 'id', 'text', false, true)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('test_items', 'name', 'text', true, false)`)
	require.NoError(t, err)

	// Setup session
	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/api/tables", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var tables []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &tables)
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "test_items", tables[0]["name"])
}

func TestHandlerGetTableSchema(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create test table with columns
	_, err := h.db.Exec(`CREATE TABLE products (id TEXT PRIMARY KEY, name TEXT NOT NULL, price INTEGER)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES
		('products', 'id', 'uuid', false, true),
		('products', 'name', 'text', false, false),
		('products', 'price', 'integer', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/api/tables/products", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var schema map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &schema)
	require.NoError(t, err)
	require.Equal(t, "products", schema["name"])

	columns := schema["columns"].([]interface{})
	require.Len(t, columns, 3)
}

func TestHandlerCreateTable(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	token := setupTestSession(t, h)

	body := `{"name":"orders","columns":[{"name":"id","type":"uuid","primary":true},{"name":"total","type":"integer","nullable":true}]}`
	req := httptest.NewRequest("POST", "/api/tables", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Verify table exists
	var count int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'orders'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestHandlerDeleteTable(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create table first
	_, err := h.db.Exec(`CREATE TABLE to_delete (id TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('to_delete', 'id', 'text', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/api/tables/to_delete", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify table is gone
	var count int
	err = h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'to_delete'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestHandlerSelectData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create and populate table
	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'Apple'), ('2', 'Banana'), ('3', 'Cherry')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/api/data/items?limit=2&offset=0", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	rows := result["rows"].([]interface{})
	require.Len(t, rows, 2)
	require.Equal(t, float64(3), result["total"])
}

func TestHandlerInsertData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"id":"new-1","name":"New Item"}`
	req := httptest.NewRequest("POST", "/api/data/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = 'new-1'`).Scan(&count)
	require.Equal(t, 1, count)
}

func TestHandlerUpdateData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'Old Name')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"name":"New Name"}`
	req := httptest.NewRequest("PATCH", "/api/data/items?id=eq.1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var name string
	h.db.QueryRow(`SELECT name FROM items WHERE id = '1'`).Scan(&name)
	require.Equal(t, "New Name", name)
}

func TestHandlerDeleteData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'To Delete')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/api/data/items?id=eq.1", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = '1'`).Scan(&count)
	require.Equal(t, 0, count)
}

func TestHandlerAddColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'id', 'text', false, true)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"name":"description","type":"text","nullable":true}`
	req := httptest.NewRequest("POST", "/api/tables/items/columns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Verify column exists in metadata
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'description'`).Scan(&count)
	require.Equal(t, 1, count)
}

func TestHandlerRenameColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT, old_name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'old_name', 'text', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"new_name":"new_name"}`
	req := httptest.NewRequest("PATCH", "/api/tables/items/columns/old_name", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'new_name'`).Scan(&count)
	require.Equal(t, 1, count)
}

func TestHandlerDropColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, to_drop TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'id', 'text', false, true), ('items', 'to_drop', 'text', true, false)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'value')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/api/tables/items/columns/to_drop", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify column is gone from metadata
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'to_drop'`).Scan(&count)
	require.Equal(t, 0, count)

	// Verify data preserved in remaining columns
	var id string
	h.db.QueryRow(`SELECT id FROM items`).Scan(&id)
	require.Equal(t, "1", id)
}
