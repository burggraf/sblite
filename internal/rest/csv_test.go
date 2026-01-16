// internal/rest/csv_test.go
package rest

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
)

func setupTestHandlerForCSV(t *testing.T) (*Handler, *db.DB) {
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
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price REAL,
			quantity INTEGER,
			metadata TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create items table: %v", err)
	}

	handler := NewHandler(database, nil)
	return handler, database
}

func TestCSVResponse(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Banana', 0.75, 25)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type text/csv, got %s", contentType)
	}

	// Parse CSV response
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 2 data rows
	if len(records) != 3 {
		t.Errorf("expected 3 CSV rows (1 header + 2 data), got %d", len(records))
	}

	// Check headers are sorted alphabetically
	headers := records[0]
	expectedHeaders := []string{"id", "metadata", "name", "price", "quantity"}
	if len(headers) != len(expectedHeaders) {
		t.Errorf("expected %d headers, got %d", len(expectedHeaders), len(headers))
	}
	for i, h := range expectedHeaders {
		if i < len(headers) && headers[i] != h {
			t.Errorf("header[%d]: expected %q, got %q", i, h, headers[i])
		}
	}
}

func TestCSVResponseEmpty(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// No data inserted - table is empty

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type text/csv, got %s", contentType)
	}

	// Body should be empty for empty results
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for empty results, got %q", w.Body.String())
	}
}

func TestCSVResponseWithNullValues(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert data with NULL values
	database.Exec(`INSERT INTO items (name, price, quantity, metadata) VALUES ('Test', NULL, NULL, NULL)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse CSV response
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 1 data row
	if len(records) != 2 {
		t.Errorf("expected 2 CSV rows, got %d", len(records))
	}

	// Check that NULL values are represented as empty strings
	dataRow := records[1]
	// Headers are sorted: id, metadata, name, price, quantity
	// metadata (index 1), price (index 3), quantity (index 4) should be empty
	if dataRow[1] != "" {
		t.Errorf("expected empty string for NULL metadata, got %q", dataRow[1])
	}
	if dataRow[3] != "" {
		t.Errorf("expected empty string for NULL price, got %q", dataRow[3])
	}
	if dataRow[4] != "" {
		t.Errorf("expected empty string for NULL quantity, got %q", dataRow[4])
	}
}

func TestCSVResponseWithFilter(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Banana', 0.75, 25)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Cherry', 3.00, 5)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Filter to only get items with price > 1.0
	req := httptest.NewRequest("GET", "/rest/v1/items?price=gt.1.0", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse CSV response
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 2 data rows (Apple and Cherry)
	if len(records) != 3 {
		t.Errorf("expected 3 CSV rows (1 header + 2 data), got %d", len(records))
	}
}

func TestCSVResponseWithSelectColumns(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Select only specific columns
	req := httptest.NewRequest("GET", "/rest/v1/items?select=name,price", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse CSV response
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 1 data row
	if len(records) != 2 {
		t.Errorf("expected 2 CSV rows, got %d", len(records))
	}

	// Check only selected columns are present (sorted alphabetically)
	headers := records[0]
	if len(headers) != 2 {
		t.Errorf("expected 2 columns, got %d: %v", len(headers), headers)
	}
	if headers[0] != "name" || headers[1] != "price" {
		t.Errorf("expected headers [name, price], got %v", headers)
	}
}

func TestCSVResponseJSONPreferred(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Request without Accept header should return JSON (default)
	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestCSVResponseWithAcceptApplicationJSON(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Request with Accept: application/json should return JSON
	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestCSVResponseConsistentColumnOrder(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert multiple rows
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple', 1.50, 10)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Banana', 0.75, 25)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Cherry', 3.00, 5)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Make multiple requests and verify column order is consistent
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/rest/v1/items", nil)
		req.Header.Set("Accept", "text/csv")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		reader := csv.NewReader(strings.NewReader(w.Body.String()))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("iteration %d: failed to parse CSV: %v", i, err)
		}

		// Verify headers are always in alphabetical order
		headers := records[0]
		expectedHeaders := []string{"id", "metadata", "name", "price", "quantity"}
		for j, h := range expectedHeaders {
			if j < len(headers) && headers[j] != h {
				t.Errorf("iteration %d, header[%d]: expected %q, got %q", i, j, h, headers[j])
			}
		}
	}
}

func TestFormatCSVValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil value", nil, ""},
		{"string value", "hello", "hello"},
		{"int value", 42, "42"},
		{"float value", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"byte slice", []byte("data"), "data"},
		{"nested map", map[string]any{"key": "value"}, `{"key":"value"}`},
		{"nested array", []any{1, 2, 3}, "[1,2,3]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCSVValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatCSVValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCSVResponseWithSpecialCharacters(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert data with special CSV characters (comma, quote, newline)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Apple, Red', 1.50, 10)`)
	database.Exec(`INSERT INTO items (name, price, quantity) VALUES ('Banana "Yellow"', 0.75, 25)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/items", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse CSV response - encoding/csv should handle special characters properly
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 2 data rows
	if len(records) != 3 {
		t.Errorf("expected 3 CSV rows, got %d", len(records))
	}

	// Verify special characters are preserved
	// Headers sorted: id, metadata, name, price, quantity
	// name is at index 2
	if records[1][2] != "Apple, Red" {
		t.Errorf("expected 'Apple, Red', got %q", records[1][2])
	}
	if records[2][2] != `Banana "Yellow"` {
		t.Errorf("expected 'Banana \"Yellow\"', got %q", records[2][2])
	}
}

func TestCSVWithLimitAndOffset(t *testing.T) {
	handler, database := setupTestHandlerForCSV(t)
	defer database.Close()

	// Insert test data
	for i := 1; i <= 10; i++ {
		database.Exec(`INSERT INTO items (name, price, quantity) VALUES (?, ?, ?)`, "Item"+string(rune('A'+i-1)), float64(i), i*10)
	}

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	// Request with limit and offset
	req := httptest.NewRequest("GET", "/rest/v1/items?limit=3&offset=2", nil)
	req.Header.Set("Accept", "text/csv")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse CSV response
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// Should have header row + 3 data rows
	if len(records) != 4 {
		t.Errorf("expected 4 CSV rows (1 header + 3 data), got %d", len(records))
	}
}
