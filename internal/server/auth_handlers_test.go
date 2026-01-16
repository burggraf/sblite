// internal/server/auth_handlers_test.go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignupEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", response["email"])
	}
	if response["id"] == nil {
		t.Error("expected id to be set")
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"email": "test@example.com", "password": "password123"}`

	// First signup
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Second signup with same email
	req = httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for duplicate email, got %d", w.Code)
	}
}
