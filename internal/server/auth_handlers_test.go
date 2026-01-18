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

	// Now signup returns a TokenResponse with user nested
	user, ok := response["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in response, got %v", response)
	}
	if user["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", user["email"])
	}
	if user["id"] == nil {
		t.Error("expected id to be set")
	}
	if response["access_token"] == nil {
		t.Error("expected access_token in signup response")
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

func TestGetUserEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login to get token
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Get user
	req = httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var userResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &userResp)
	if userResp["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", userResp["email"])
	}
}

func TestGetUserUnauthorized(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/user", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestUpdateUserMetadata(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login to get token
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Update user metadata (uses 'data' field like Supabase client)
	updateBody := `{"data": {"name": "Test User", "age": 30}}`
	req = httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var userResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &userResp)
	metadata, ok := userResp["user_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected user_metadata to be map, got %T", userResp["user_metadata"])
	}
	if metadata["name"] != "Test User" {
		t.Errorf("expected name 'Test User', got %v", metadata["name"])
	}
}

func TestUpdateUserPassword(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login to get token
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Update password
	updateBody := `{"password": "newpassword123"}`
	req = httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Try to login with new password
	loginBody = `{"email": "test@example.com", "password": "newpassword123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for new password login, got %d: %s", w.Code, w.Body.String())
	}

	// Try to login with old password (should fail)
	loginBody = `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for old password login, got %d", w.Code)
	}
}

func TestLogoutEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Logout
	req = httptest.NewRequest("POST", "/auth/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutScopedLocal(t *testing.T) {
	srv := setupTestServer(t)

	// Create user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Login twice to create two sessions
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp1 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp1)
	token1 := loginResp1["access_token"].(string)
	refreshToken1 := loginResp1["refresh_token"].(string)

	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp2 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp2)
	token2 := loginResp2["access_token"].(string)
	refreshToken2 := loginResp2["refresh_token"].(string)

	// Logout session 1 with scope=local
	logoutBody := `{"scope": "local"}`
	req = httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Session 1's refresh token should be revoked
	refreshBody := `{"refresh_token": "` + refreshToken1 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected session 1 refresh to fail with 401, got %d", w.Code)
	}

	// Session 2 should still work
	refreshBody = `{"refresh_token": "` + refreshToken2 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected session 2 refresh to succeed with 200, got %d: %s", w.Code, w.Body.String())
	}

	// Session 2's access token should still work
	req = httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected session 2 to still be valid with 200, got %d", w.Code)
	}
}

func TestLogoutScopedGlobal(t *testing.T) {
	srv := setupTestServer(t)

	// Create user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Login twice to create two sessions
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp1 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp1)
	token1 := loginResp1["access_token"].(string)
	refreshToken1 := loginResp1["refresh_token"].(string)

	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp2 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp2)
	refreshToken2 := loginResp2["refresh_token"].(string)

	// Logout with scope=global (should revoke ALL sessions)
	logoutBody := `{"scope": "global"}`
	req = httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Both refresh tokens should be revoked
	refreshBody := `{"refresh_token": "` + refreshToken1 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected session 1 refresh to fail with 401, got %d", w.Code)
	}

	refreshBody = `{"refresh_token": "` + refreshToken2 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected session 2 refresh to fail with 401, got %d", w.Code)
	}
}

func TestLogoutScopedOthers(t *testing.T) {
	srv := setupTestServer(t)

	// Create user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Login twice to create two sessions
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp1 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp1)
	token1 := loginResp1["access_token"].(string)
	refreshToken1 := loginResp1["refresh_token"].(string)

	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp2 map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp2)
	refreshToken2 := loginResp2["refresh_token"].(string)

	// Logout with scope=others (should revoke all except current)
	logoutBody := `{"scope": "others"}`
	req = httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Session 1's refresh token should still work (current session)
	refreshBody := `{"refresh_token": "` + refreshToken1 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected session 1 refresh to succeed with 200, got %d: %s", w.Code, w.Body.String())
	}

	// Session 2's refresh token should be revoked
	refreshBody = `{"refresh_token": "` + refreshToken2 + `"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBufferString(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected session 2 refresh to fail with 401, got %d", w.Code)
	}
}

func TestLogoutInvalidScope(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Logout with invalid scope
	logoutBody := `{"scope": "invalid"}`
	req = httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid scope, got %d: %s", w.Code, w.Body.String())
	}
}
