// internal/rest/handler_test.go
package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/schema"
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

	handler := NewHandler(database, nil, nil) // nil enforcer for tests without RLS
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
		name            string
		prefer          string
		expectedCount   string
		expectedExplain bool
	}{
		{"count=exact", "count=exact", "exact", false},
		{"count=planned", "count=planned", "planned", false},
		{"count=estimated", "count=estimated", "estimated", false},
		{"no count", "return=representation", "", false},
		{"empty", "", "", false},
		{"count=exact with other options", "return=representation, count=exact", "exact", false},
		{"explain only", "explain", "", true},
		{"explain=true", "explain=true", "", true},
		{"explain with count", "count=exact, explain", "exact", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, _, explain := parsePreferHeader(tt.prefer)
			if count != tt.expectedCount {
				t.Errorf("parsePreferHeader(%q) count = %q, want %q", tt.prefer, count, tt.expectedCount)
			}
			if explain != tt.expectedExplain {
				t.Errorf("parsePreferHeader(%q) explain = %v, want %v", tt.prefer, explain, tt.expectedExplain)
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

// setupTestHandlerWithRelations creates a handler with related tables (countries and cities)
func setupTestHandlerWithRelations(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create countries table
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS countries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			code TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create countries table: %v", err)
	}

	// Create cities table with foreign key to countries
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS cities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			population INTEGER,
			country_id INTEGER REFERENCES countries(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create cities table: %v", err)
	}

	handler := NewHandler(database, nil, nil)
	return handler, database
}

func TestFilterOnRelatedTable(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US'), (3, 'Mexico', 'MX')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Vancouver', 631486, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Los Angeles', 3979576, 2)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Mexico City', 8918653, 3)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// GET /cities?countries.name=eq.Canada should return only Toronto and Vancouver
	req := httptest.NewRequest("GET", "/rest/v1/cities?countries.name=eq.Canada", nil)
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
		t.Errorf("expected 2 cities (Canadian cities), got %d", len(response))
	}

	// Verify the cities are Canadian
	for _, city := range response {
		name := city["name"].(string)
		if name != "Toronto" && name != "Vancouver" {
			t.Errorf("unexpected city: %s", name)
		}
	}
}

func TestFilterOnRelatedTableWithMultipleFilters(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Vancouver', 631486, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Filter by related country AND local population
	req := httptest.NewRequest("GET", "/rest/v1/cities?countries.name=eq.Canada&population=gt.1000000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Only Toronto has population > 1M and is in Canada
	if len(response) != 1 {
		t.Errorf("expected 1 city, got %d", len(response))
	}

	if len(response) > 0 {
		name := response[0]["name"].(string)
		if name != "Toronto" {
			t.Errorf("expected Toronto, got %s", name)
		}
	}
}

func TestOrderByRelatedColumn(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US'), (3, 'Mexico', 'MX')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Mexico City', 8918653, 3)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Order cities by country name ascending
	req := httptest.NewRequest("GET", "/rest/v1/cities?order=countries(name)", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 3 {
		t.Fatalf("expected 3 cities, got %d", len(response))
	}

	// Order should be: Canada (Toronto), Mexico (Mexico City), USA (New York)
	expectedOrder := []string{"Toronto", "Mexico City", "New York"}
	for i, city := range response {
		name := city["name"].(string)
		if name != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], name)
		}
	}
}

func TestOrderByRelatedColumnDesc(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US'), (3, 'Mexico', 'MX')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Mexico City', 8918653, 3)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Order cities by country name descending
	req := httptest.NewRequest("GET", "/rest/v1/cities?order=countries(name).desc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 3 {
		t.Fatalf("expected 3 cities, got %d", len(response))
	}

	// Order should be: USA (New York), Mexico (Mexico City), Canada (Toronto)
	expectedOrder := []string{"New York", "Mexico City", "Toronto"}
	for i, city := range response {
		name := city["name"].(string)
		if name != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], name)
		}
	}
}

func TestFilterAndOrderOnRelatedTable(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US'), (3, 'Mexico', 'MX')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Vancouver', 631486, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Mexico City', 8918653, 3)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Filter by population > 1M and order by country name
	req := httptest.NewRequest("GET", "/rest/v1/cities?population=gt.1000000&order=countries(name)", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Cities with pop > 1M ordered by country: Toronto (Canada), Mexico City (Mexico), New York (USA)
	if len(response) != 3 {
		t.Fatalf("expected 3 cities, got %d", len(response))
	}

	expectedOrder := []string{"Toronto", "Mexico City", "New York"}
	for i, city := range response {
		name := city["name"].(string)
		if name != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], name)
		}
	}
}

func TestExplainModifier(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("Prefer", "explain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check that response contains sql, args, and plan
	if _, ok := response["sql"]; !ok {
		t.Error("expected response to contain 'sql' field")
	}
	if _, ok := response["args"]; !ok {
		t.Error("expected response to contain 'args' field")
	}
	if _, ok := response["plan"]; !ok {
		t.Error("expected response to contain 'plan' field")
	}

	// Verify sql field contains SELECT statement
	sql, ok := response["sql"].(string)
	if !ok {
		t.Fatal("sql field is not a string")
	}
	if !strings.Contains(sql, "SELECT") {
		t.Errorf("expected sql to contain SELECT, got: %s", sql)
	}
	if !strings.Contains(sql, "todos") {
		t.Errorf("expected sql to contain table name 'todos', got: %s", sql)
	}

	// Verify plan is an array
	plan, ok := response["plan"].([]any)
	if !ok {
		t.Fatal("plan field is not an array")
	}
	if len(plan) == 0 {
		t.Error("expected plan to have at least one entry")
	}

	// Verify plan entries have expected fields
	if len(plan) > 0 {
		entry, ok := plan[0].(map[string]any)
		if !ok {
			t.Fatal("plan entry is not a map")
		}
		if _, ok := entry["id"]; !ok {
			t.Error("expected plan entry to have 'id' field")
		}
		if _, ok := entry["parent"]; !ok {
			t.Error("expected plan entry to have 'parent' field")
		}
		if _, ok := entry["detail"]; !ok {
			t.Error("expected plan entry to have 'detail' field")
		}
	}
}

func TestExplainModifierWithFilter(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.0", nil)
	req.Header.Set("Prefer", "explain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify sql contains the filter
	sql, ok := response["sql"].(string)
	if !ok {
		t.Fatal("sql field is not a string")
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("expected sql to contain WHERE clause, got: %s", sql)
	}

	// Verify args contains the filter value
	args, ok := response["args"].([]any)
	if !ok {
		t.Fatal("args field is not an array")
	}
	if len(args) == 0 {
		t.Error("expected args to contain filter value")
	}
}

func TestExplainModifierWithOrderAndLimit(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1), ('Test 3', 0)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos?order=title.desc&limit=2", nil)
	req.Header.Set("Prefer", "explain=true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify sql contains ORDER BY and LIMIT
	sql, ok := response["sql"].(string)
	if !ok {
		t.Fatal("sql field is not a string")
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("expected sql to contain ORDER BY, got: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT") {
		t.Errorf("expected sql to contain LIMIT, got: %s", sql)
	}
}

func TestExplainModifierWithRelatedFilter(t *testing.T) {
	handler, database := setupTestHandlerWithRelations(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO countries (id, name, code) VALUES (1, 'Canada', 'CA'), (2, 'USA', 'US')`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('Toronto', 2731571, 1)`)
	database.Exec(`INSERT INTO cities (name, population, country_id) VALUES ('New York', 8336817, 2)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Filter by related table
	req := httptest.NewRequest("GET", "/rest/v1/cities?countries.name=eq.Canada", nil)
	req.Header.Set("Prefer", "explain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify sql references the related table (could be via JOIN or EXISTS subquery)
	sql, ok := response["sql"].(string)
	if !ok {
		t.Fatal("sql field is not a string")
	}
	if !strings.Contains(sql, "countries") {
		t.Errorf("expected sql to reference countries table, got: %s", sql)
	}
	// The implementation uses EXISTS with a subquery for related filters
	if !strings.Contains(sql, "EXISTS") && !strings.Contains(sql, "JOIN") {
		t.Errorf("expected sql to contain EXISTS or JOIN for related table filter, got: %s", sql)
	}
}

func TestParseUpsertOptions(t *testing.T) {
	tests := []struct {
		name                    string
		prefer                  string
		expectedOnConflict      []string
		expectedIgnoreDuplicates bool
	}{
		{
			name:                    "merge-duplicates only",
			prefer:                  "resolution=merge-duplicates",
			expectedOnConflict:      nil,
			expectedIgnoreDuplicates: false,
		},
		{
			name:                    "ignore-duplicates",
			prefer:                  "resolution=ignore-duplicates",
			expectedOnConflict:      nil,
			expectedIgnoreDuplicates: true,
		},
		{
			name:                    "merge-duplicates with on-conflict single column",
			prefer:                  "resolution=merge-duplicates,on-conflict=email",
			expectedOnConflict:      []string{"email"},
			expectedIgnoreDuplicates: false,
		},
		{
			name:                    "merge-duplicates with on-conflict multiple columns",
			prefer:                  "resolution=merge-duplicates,on-conflict=user_id,date",
			expectedOnConflict:      []string{"user_id", "date"},
			expectedIgnoreDuplicates: false,
		},
		{
			name:                    "ignore-duplicates with on-conflict",
			prefer:                  "resolution=ignore-duplicates,on-conflict=email",
			expectedOnConflict:      []string{"email"},
			expectedIgnoreDuplicates: true,
		},
		{
			name:                    "with return=representation",
			prefer:                  "return=representation,resolution=merge-duplicates,on-conflict=id",
			expectedOnConflict:      []string{"id"},
			expectedIgnoreDuplicates: false,
		},
		{
			name:                    "empty prefer",
			prefer:                  "",
			expectedOnConflict:      nil,
			expectedIgnoreDuplicates: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onConflict, ignoreDuplicates := parseUpsertOptions(tt.prefer)

			if ignoreDuplicates != tt.expectedIgnoreDuplicates {
				t.Errorf("parseUpsertOptions(%q) ignoreDuplicates = %v, want %v",
					tt.prefer, ignoreDuplicates, tt.expectedIgnoreDuplicates)
			}

			if len(onConflict) != len(tt.expectedOnConflict) {
				t.Errorf("parseUpsertOptions(%q) onConflict = %v, want %v",
					tt.prefer, onConflict, tt.expectedOnConflict)
				return
			}

			for i, col := range onConflict {
				if col != tt.expectedOnConflict[i] {
					t.Errorf("parseUpsertOptions(%q) onConflict[%d] = %q, want %q",
						tt.prefer, i, col, tt.expectedOnConflict[i])
				}
			}
		})
	}
}

// setupTestHandlerWithUpsertTable creates a handler with a users table for upsert testing
func setupTestHandlerWithUpsertTable(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create users table with unique constraint on email
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			status TEXT DEFAULT 'active'
		)
	`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	handler := NewHandler(database, nil, nil)
	return handler, database
}

func TestUpsertMergeDuplicates(t *testing.T) {
	handler, database := setupTestHandlerWithUpsertTable(t)
	defer database.Close()

	// Insert initial user
	database.Exec(`INSERT INTO users (id, email, name, status) VALUES (1, 'test@example.com', 'Original Name', 'active')`)

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	// Upsert with resolution=merge-duplicates (should update existing row)
	body := `{"id": 1, "email": "test@example.com", "name": "Updated Name", "status": "active"}`
	req := httptest.NewRequest("POST", "/rest/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=representation")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the row was updated
	var name string
	err := database.QueryRow(`SELECT name FROM users WHERE id = 1`).Scan(&name)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %q", name)
	}

	// Verify only one row exists
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestUpsertOnConflictEmail(t *testing.T) {
	handler, database := setupTestHandlerWithUpsertTable(t)
	defer database.Close()

	// Insert initial user
	database.Exec(`INSERT INTO users (id, email, name, status) VALUES (1, 'test@example.com', 'Original Name', 'active')`)

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	// Upsert with on-conflict=email (should match by email and update)
	body := `{"email": "test@example.com", "name": "Updated via Email", "status": "inactive"}`
	req := httptest.NewRequest("POST", "/rest/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,on-conflict=email")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the row was updated
	var name, status string
	err := database.QueryRow(`SELECT name, status FROM users WHERE email = 'test@example.com'`).Scan(&name, &status)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "Updated via Email" {
		t.Errorf("expected name 'Updated via Email', got %q", name)
	}
	if status != "inactive" {
		t.Errorf("expected status 'inactive', got %q", status)
	}

	// Verify only one row exists
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestUpsertIgnoreDuplicates(t *testing.T) {
	handler, database := setupTestHandlerWithUpsertTable(t)
	defer database.Close()

	// Insert initial user
	database.Exec(`INSERT INTO users (id, email, name, status) VALUES (1, 'test@example.com', 'Original Name', 'active')`)

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	// Upsert with resolution=ignore-duplicates (should NOT update existing row)
	body := `{"id": 1, "email": "test@example.com", "name": "This Should Not Update", "status": "inactive"}`
	req := httptest.NewRequest("POST", "/rest/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=ignore-duplicates")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the row was NOT updated (original values preserved)
	var name, status string
	err := database.QueryRow(`SELECT name, status FROM users WHERE id = 1`).Scan(&name, &status)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "Original Name" {
		t.Errorf("expected name 'Original Name', got %q (row should not have been updated)", name)
	}
	if status != "active" {
		t.Errorf("expected status 'active', got %q (row should not have been updated)", status)
	}

	// Verify only one row exists
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestUpsertIgnoreDuplicatesWithOnConflict(t *testing.T) {
	handler, database := setupTestHandlerWithUpsertTable(t)
	defer database.Close()

	// Insert initial user
	database.Exec(`INSERT INTO users (id, email, name, status) VALUES (1, 'test@example.com', 'Original Name', 'active')`)

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	// Upsert with resolution=ignore-duplicates and on-conflict=email
	body := `{"email": "test@example.com", "name": "This Should Not Update", "status": "inactive"}`
	req := httptest.NewRequest("POST", "/rest/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=ignore-duplicates,on-conflict=email")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the row was NOT updated
	var name string
	err := database.QueryRow(`SELECT name FROM users WHERE email = 'test@example.com'`).Scan(&name)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "Original Name" {
		t.Errorf("expected name 'Original Name', got %q", name)
	}
}

func TestUpsertNewRowWithIgnoreDuplicates(t *testing.T) {
	handler, database := setupTestHandlerWithUpsertTable(t)
	defer database.Close()

	// No existing users

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	// Insert new user with ignore-duplicates (should insert since no conflict)
	body := `{"email": "new@example.com", "name": "New User", "status": "active"}`
	req := httptest.NewRequest("POST", "/rest/v1/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=ignore-duplicates,on-conflict=email")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the row was inserted
	var name string
	err := database.QueryRow(`SELECT name FROM users WHERE email = 'new@example.com'`).Scan(&name)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "New User" {
		t.Errorf("expected name 'New User', got %q", name)
	}
}

// setupTestHandlerWithSchema creates a handler with a schema for validation tests
func setupTestHandlerWithSchema(t *testing.T) (*Handler, *db.DB) {
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
		CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			age INTEGER,
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		t.Fatalf("failed to create profiles table: %v", err)
	}

	// Create schema and register columns
	s := schema.New(database.DB)

	// Register columns with schema metadata
	err = s.RegisterColumn(schema.Column{
		TableName:  "profiles",
		ColumnName: "id",
		PgType:     "uuid",
		IsNullable: false,
		IsPrimary:  true,
	})
	if err != nil {
		t.Fatalf("failed to register id column: %v", err)
	}

	err = s.RegisterColumn(schema.Column{
		TableName:  "profiles",
		ColumnName: "name",
		PgType:     "text",
		IsNullable: false,
	})
	if err != nil {
		t.Fatalf("failed to register name column: %v", err)
	}

	err = s.RegisterColumn(schema.Column{
		TableName:  "profiles",
		ColumnName: "age",
		PgType:     "integer",
		IsNullable: true,
	})
	if err != nil {
		t.Fatalf("failed to register age column: %v", err)
	}

	err = s.RegisterColumn(schema.Column{
		TableName:  "profiles",
		ColumnName: "active",
		PgType:     "boolean",
		IsNullable: true,
	})
	if err != nil {
		t.Fatalf("failed to register active column: %v", err)
	}

	handler := NewHandler(database, nil, s)
	return handler, database
}

func TestValidateRow(t *testing.T) {
	t.Run("nil schema - should pass", func(t *testing.T) {
		handler, database := setupTestHandler(t)
		defer database.Close()

		// Handler from setupTestHandler has nil schema
		err := handler.validateRow("todos", map[string]any{
			"title":     "Test",
			"completed": 0,
		})
		if err != nil {
			t.Errorf("expected nil schema to pass validation, got error: %v", err)
		}
	})

	t.Run("table with no schema metadata (raw SQL) - should pass", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		// The todos table exists in the database but has no schema metadata
		// (only profiles has schema metadata)
		err := handler.validateRow("todos", map[string]any{
			"title":     "Test",
			"completed": 0,
		})
		if err != nil {
			t.Errorf("expected table without schema metadata to pass validation, got error: %v", err)
		}
	})

	t.Run("valid data passing validation", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":     "550e8400-e29b-41d4-a716-446655440000",
			"name":   "John Doe",
			"age":    30,
			"active": 1,
		})
		if err != nil {
			t.Errorf("expected valid data to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid UUID format - should fail", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":   "not-a-valid-uuid",
			"name": "John Doe",
		})
		if err == nil {
			t.Error("expected invalid UUID to fail validation")
		}
		if !strings.Contains(err.Error(), "id") || !strings.Contains(err.Error(), "uuid") {
			t.Errorf("expected error to mention column 'id' and 'uuid', got: %v", err)
		}
	})

	t.Run("null value in non-nullable column - should fail", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":   "550e8400-e29b-41d4-a716-446655440000",
			"name": nil, // name is non-nullable
		})
		if err == nil {
			t.Error("expected null in non-nullable column to fail validation")
		}
		if !strings.Contains(err.Error(), "name") || !strings.Contains(err.Error(), "null") {
			t.Errorf("expected error to mention column 'name' and 'null', got: %v", err)
		}
	})

	t.Run("unknown column - should pass (let SQLite handle it)", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":            "550e8400-e29b-41d4-a716-446655440000",
			"name":          "John Doe",
			"unknown_field": "some value",
		})
		if err != nil {
			t.Errorf("expected unknown column to pass validation (let SQLite handle), got error: %v", err)
		}
	})

	t.Run("null value in nullable column - should pass", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":   "550e8400-e29b-41d4-a716-446655440000",
			"name": "John Doe",
			"age":  nil, // age is nullable
		})
		if err != nil {
			t.Errorf("expected null in nullable column to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid type for column - should fail", func(t *testing.T) {
		handler, database := setupTestHandlerWithSchema(t)
		defer database.Close()

		err := handler.validateRow("profiles", map[string]any{
			"id":   "550e8400-e29b-41d4-a716-446655440000",
			"name": "John Doe",
			"age":  "not-an-integer",
		})
		if err == nil {
			t.Error("expected invalid type for integer column to fail validation")
		}
		if !strings.Contains(err.Error(), "age") {
			t.Errorf("expected error to mention column 'age', got: %v", err)
		}
	})
}
