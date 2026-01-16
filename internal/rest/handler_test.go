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

	handler := NewHandler(database, nil) // nil enforcer for tests without RLS
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

func TestCountExact(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header to be present")
	}
	// With 3 rows, offset 0, Content-Range should be "0-2/3"
	expected := "0-2/3"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}
}

func TestCountExactWithFilter(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Filter to get only completed=0 (2 rows)
	req := httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.0", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header - should be "0-1/2"
	contentRange := w.Header().Get("Content-Range")
	expected := "0-1/2"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}
}

func TestCountExactWithLimit(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Limit to 1 row, but count should still be total (3)
	req := httptest.NewRequest("GET", "/rest/v1/todos?limit=1", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header - only 1 row returned but total is 3
	contentRange := w.Header().Get("Content-Range")
	expected := "0-0/3"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}

	// Check that only 1 row is returned in body
	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(response) != 1 {
		t.Errorf("expected 1 row, got %d", len(response))
	}
}

func TestCountExactWithOffset(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Offset=1, limit=2 - should return 2 rows starting at offset 1
	req := httptest.NewRequest("GET", "/rest/v1/todos?limit=2&offset=1", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header - range is 1-2, total is 3
	contentRange := w.Header().Get("Content-Range")
	expected := "1-2/3"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}
}

func TestCountPlanned(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "count=planned")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// count=planned should also return Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header to be present for count=planned")
	}
}

func TestCountEstimated(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "count=estimated")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// count=estimated should also return Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header to be present for count=estimated")
	}
}

func TestNoCountHeader(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// No Prefer header - should not have Content-Range
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should NOT have Content-Range header when count not requested
	contentRange := w.Header().Get("Content-Range")
	if contentRange != "" {
		t.Errorf("expected no Content-Range header, got %q", contentRange)
	}
}

func TestHeadRequest(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Head("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("HEAD", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check Content-Range header
	contentRange := w.Header().Get("Content-Range")
	expected := "0-0/3"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}

	// Body should be empty for HEAD request
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for HEAD request, got %d bytes", w.Body.Len())
	}
}

func TestHeadRequestWithFilter(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Head("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("HEAD", "/rest/v1/todos?completed=eq.0", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check Content-Range header - should be count of filtered rows (2)
	contentRange := w.Header().Get("Content-Range")
	expected := "0-0/2"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}
}

func TestCountEmptyTable(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// No data inserted - table is empty

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header for empty result
	contentRange := w.Header().Get("Content-Range")
	expected := "0-0/0"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}
}

func TestParsePreferHeader(t *testing.T) {
	tests := []struct {
		name          string
		prefer        string
		expectedCount string
	}{
		{"count=exact", "count=exact", "exact"},
		{"count=planned", "count=planned", "planned"},
		{"count=estimated", "count=estimated", "estimated"},
		{"no count", "return=representation", ""},
		{"empty", "", ""},
		{"count=exact with other options", "return=representation, count=exact", "exact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, _ := parsePreferHeader(tt.prefer)
			if count != tt.expectedCount {
				t.Errorf("parsePreferHeader(%q) = %q, want %q", tt.prefer, count, tt.expectedCount)
			}
		})
	}
}

func TestParseRangeHeader(t *testing.T) {
	tests := []struct {
		name          string
		rangeHeader   string
		expectedStart int
		expectedEnd   int
		expectedOk    bool
	}{
		{"basic range", "0-9", 0, 9, true},
		{"range with items prefix", "items=0-24", 0, 24, true},
		{"offset range", "10-19", 10, 19, true},
		{"single item", "5-5", 5, 5, true},
		{"empty string", "", 0, 0, false},
		{"missing end", "0-", 0, 0, false},
		{"missing start", "-9", 0, 0, false},
		{"non-numeric start", "abc-9", 0, 0, false},
		{"non-numeric end", "0-xyz", 0, 0, false},
		{"negative start", "-1-9", 0, 0, false},
		{"end less than start", "10-5", 0, 0, false},
		{"invalid format", "0,9", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, ok := parseRangeHeader(tt.rangeHeader)
			if ok != tt.expectedOk {
				t.Errorf("parseRangeHeader(%q) ok = %v, want %v", tt.rangeHeader, ok, tt.expectedOk)
			}
			if ok && (start != tt.expectedStart || end != tt.expectedEnd) {
				t.Errorf("parseRangeHeader(%q) = (%d, %d), want (%d, %d)",
					tt.rangeHeader, start, end, tt.expectedStart, tt.expectedEnd)
			}
		})
	}
}

func TestRangeHeader(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (20 rows)
	for i := 1; i <= 20; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test Range: 0-9 (first 10 items)
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Range", "0-9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 206 Partial Content because there are more results
	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 10 {
		t.Errorf("expected 10 rows, got %d", len(response))
	}
}

func TestRangeHeaderWithItemsPrefix(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (20 rows)
	for i := 1; i <= 20; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test Range: items=5-14 (items 5-14, 10 items starting at offset 5)
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Range", "items=5-14")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 206 Partial Content
	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 10 {
		t.Errorf("expected 10 rows, got %d", len(response))
	}
}

func TestRangeHeaderLastPage(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (15 rows)
	for i := 1; i <= 15; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test Range: 10-19 (last 5 items, requesting 10 but only 5 remain)
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Range", "10-19")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 200 OK because we got fewer results than the limit (end of data)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 5 {
		t.Errorf("expected 5 rows, got %d", len(response))
	}
}

func TestRangeHeaderInvalid(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test invalid Range header - should be ignored and return all results
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Range", "invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should return all rows since Range header was invalid
	if len(response) != 2 {
		t.Errorf("expected 2 rows, got %d", len(response))
	}
}

func TestRangeHeaderDoesNotOverrideExplicitLimit(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (20 rows)
	for i := 1; i <= 20; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test: Range header requests 0-9 (10 items) but explicit limit=5 should take precedence
	req := httptest.NewRequest("GET", "/rest/v1/todos?limit=5", nil)
	req.Header.Set("Range", "0-9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 200 OK (not 206) because Range header was ignored
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should return 5 rows (from explicit limit), not 10 (from Range header)
	if len(response) != 5 {
		t.Errorf("expected 5 rows (from explicit limit), got %d", len(response))
	}
}

func TestRangeHeaderDoesNotOverrideExplicitOffset(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (20 rows)
	for i := 1; i <= 20; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test: Range header requests 0-9 but explicit offset=5 with explicit limit=3 should take precedence
	// (offset alone without limit doesn't apply in current implementation)
	req := httptest.NewRequest("GET", "/rest/v1/todos?offset=5&limit=3", nil)
	req.Header.Set("Range", "0-9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 200 OK (not 206) because Range header was ignored
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should return 3 rows (from explicit limit=3, starting at offset=5)
	// NOT 10 rows from Range header 0-9
	if len(response) != 3 {
		t.Errorf("expected 3 rows (from explicit limit=3), got %d", len(response))
	}
}

func TestRangeHeaderWithCount(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data (20 rows)
	for i := 1; i <= 20; i++ {
		database.Exec(`INSERT INTO todos (title, completed) VALUES (?, ?)`, "Test "+string(rune('A'+i-1)), i%2)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Test Range with count=exact
	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Range", "0-9")
	req.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 206 Partial Content
	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Range header - should show 0-9/20
	contentRange := w.Header().Get("Content-Range")
	expected := "0-9/20"
	if contentRange != expected {
		t.Errorf("expected Content-Range %q, got %q", expected, contentRange)
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 10 {
		t.Errorf("expected 10 rows, got %d", len(response))
	}
}
