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

func TestLoginEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// First create a user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Now login
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["access_token"] == nil {
		t.Error("expected access_token to be set")
	}
	if response["refresh_token"] == nil {
		t.Error("expected refresh_token to be set")
	}
	if response["token_type"] != "bearer" {
		t.Errorf("expected token_type=bearer, got %v", response["token_type"])
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	srv := setupTestServer(t)

	// First create a user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Now try to login with wrong password
	loginBody := `{"email": "test@example.com", "password": "wrongpassword"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
