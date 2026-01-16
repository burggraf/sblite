// integration_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/server"
)

// generateTestAPIKey creates an API key for testing
func generateTestAPIKey(jwtSecret, role string) string {
	claims := jwt.MapClaims{
		"role": role,
		"iss":  "sblite",
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	key, _ := token.SignedString([]byte(jwtSecret))
	return key
}

func TestFullAuthFlow(t *testing.T) {
	// Setup
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	srv := server.New(database, "test-secret-key-min-32-characters")

	// 1. Signup
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("signup failed: %d %s", w.Code, w.Body.String())
	}

	// 2. Login
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	accessToken := loginResp["access_token"].(string)
	refreshToken := loginResp["refresh_token"].(string)

	// 3. Get user
	req = httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get user failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Refresh token
	refreshBody := map[string]string{"refresh_token": refreshToken}
	refreshJSON, _ := json.Marshal(refreshBody)
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBuffer(refreshJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d %s", w.Code, w.Body.String())
	}

	var refreshResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &refreshResp)
	newAccessToken := refreshResp["access_token"].(string)

	// 5. Logout
	req = httptest.NewRequest("POST", "/auth/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer "+newAccessToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("logout failed: %d %s", w.Code, w.Body.String())
	}

	t.Log("Full auth flow completed successfully")
}

func TestFullRESTFlow(t *testing.T) {
	// Setup
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create test table
	database.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
		)
	`)

	jwtSecret := "test-secret-key-min-32-characters"
	srv := server.New(database, jwtSecret)
	apiKey := generateTestAPIKey(jwtSecret, "anon")

	// 1. Create todo
	createBody := `{"title": "Test Todo", "completed": 0}`
	req := httptest.NewRequest("POST", "/rest/v1/todos", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	req.Header.Set("apikey", apiKey)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
	}

	// 2. Read todos
	req = httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("apikey", apiKey)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("read failed: %d %s", w.Code, w.Body.String())
	}

	var todos []map[string]any
	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// 3. Update todo
	updateBody := `{"completed": 1}`
	req = httptest.NewRequest("PATCH", "/rest/v1/todos?id=eq.1", bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", apiKey)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Read with filter
	req = httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.1", nil)
	req.Header.Set("apikey", apiKey)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("filtered read failed: %d %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 completed todo, got %d", len(todos))
	}

	// 5. Delete todo
	req = httptest.NewRequest("DELETE", "/rest/v1/todos?id=eq.1", nil)
	req.Header.Set("apikey", apiKey)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}

	// 6. Verify deletion
	req = httptest.NewRequest("GET", "/rest/v1/todos", nil)
	req.Header.Set("apikey", apiKey)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 0 {
		t.Fatalf("expected 0 todos after deletion, got %d", len(todos))
	}

	t.Log("Full REST flow completed successfully")
}
