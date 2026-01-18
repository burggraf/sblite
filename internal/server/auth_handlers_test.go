// internal/server/auth_handlers_test.go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// signupAndConfirmUser creates a user and confirms their email.
// Returns the user ID. This helper handles the email confirmation flow.
func signupAndConfirmUser(t *testing.T, srv *Server, email, password string) string {
	t.Helper()

	body := `{"email": "` + email + `", "password": "` + password + `"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("signup failed with status %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse signup response: %v", err)
	}

	// Get user ID from response
	userID, ok := response["id"].(string)
	if !ok {
		t.Fatalf("expected id in signup response, got %v", response)
	}

	// Confirm the user's email using the auth service directly
	user, err := srv.authService.GetUserByID(userID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	// Use VerifyEmailByID to confirm the email
	if err := srv.authService.ConfirmEmail(user.ID); err != nil {
		t.Fatalf("failed to confirm email: %v", err)
	}

	return userID
}

// loginUser logs in a user and returns the access token and refresh token.
func loginUser(t *testing.T, srv *Server, email, password string) (accessToken, refreshToken string) {
	t.Helper()

	body := `{"email": "` + email + `", "password": "` + password + `"}`
	req := httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed with status %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}

	accessToken, _ = response["access_token"].(string)
	refreshToken, _ = response["refresh_token"].(string)
	return accessToken, refreshToken
}

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

	// With email confirmation required, signup returns confirmation info (no tokens)
	if response["id"] == nil {
		t.Error("expected id to be set")
	}
	if response["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", response["email"])
	}
	if response["email_confirmation_required"] != true {
		t.Error("expected email_confirmation_required to be true")
	}
	if response["confirmation_sent_at"] == nil {
		t.Error("expected confirmation_sent_at to be set")
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

func TestLoginWithUnconfirmedEmail(t *testing.T) {
	srv := setupTestServer(t)

	// Create a user (email not confirmed)
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Try to login without confirming email
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for unconfirmed email, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["error"] != "email_not_confirmed" {
		t.Errorf("expected error 'email_not_confirmed', got %v", response["error"])
	}
}

func TestLoginEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Now login
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Now try to login with wrong password
	loginBody := `{"email": "test@example.com", "password": "wrongpassword"}`
	req := httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetUserEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login to get token
	token, _ := loginUser(t, srv, "test@example.com", "password123")

	// Get user
	req := httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login to get token
	token, _ := loginUser(t, srv, "test@example.com", "password123")

	// Update user metadata (uses 'data' field like Supabase client)
	updateBody := `{"data": {"name": "Test User", "age": 30}}`
	req := httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login to get token
	token, _ := loginUser(t, srv, "test@example.com", "password123")

	// Update password
	updateBody := `{"password": "newpassword123"}`
	req := httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Try to login with new password
	loginBody := `{"email": "test@example.com", "password": "newpassword123"}`
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login to get token
	token, _ := loginUser(t, srv, "test@example.com", "password123")

	// Logout
	req := httptest.NewRequest("POST", "/auth/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutScopedLocal(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login twice to create two sessions
	token1, refreshToken1 := loginUser(t, srv, "test@example.com", "password123")
	_, refreshToken2 := loginUser(t, srv, "test@example.com", "password123")

	// Logout session 1 with scope=local
	logoutBody := `{"scope": "local"}`
	req := httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
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
}

func TestLogoutScopedGlobal(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login twice to create two sessions
	token1, refreshToken1 := loginUser(t, srv, "test@example.com", "password123")
	_, refreshToken2 := loginUser(t, srv, "test@example.com", "password123")

	// Logout with scope=global (should revoke ALL sessions)
	logoutBody := `{"scope": "global"}`
	req := httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login twice to create two sessions
	token1, refreshToken1 := loginUser(t, srv, "test@example.com", "password123")
	_, refreshToken2 := loginUser(t, srv, "test@example.com", "password123")

	// Logout with scope=others (should revoke all except current)
	logoutBody := `{"scope": "others"}`
	req := httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
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

	// Create user and confirm email
	signupAndConfirmUser(t, srv, "test@example.com", "password123")

	// Login to get token
	token, _ := loginUser(t, srv, "test@example.com", "password123")

	// Logout with invalid scope
	logoutBody := `{"scope": "invalid"}`
	req := httptest.NewRequest("POST", "/auth/v1/logout", bytes.NewBufferString(logoutBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid scope, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettingsShowsEnabledOAuthProviders(t *testing.T) {
	srv := setupTestServer(t)

	// Enable Google, disable GitHub
	srv.configureOAuthProvider("google", "id", "secret", true)
	srv.configureOAuthProvider("github", "id", "secret", false)

	req := httptest.NewRequest("GET", "/auth/v1/settings", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		External map[string]bool `json:"external"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !resp.External["google"] {
		t.Error("expected google to be enabled")
	}
	if resp.External["github"] {
		t.Error("expected github to be disabled")
	}
	if !resp.External["email"] {
		t.Error("expected email to always be enabled")
	}
}

func TestSettingsShowsMailerAutoconfirm(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/settings", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// By default, email confirmation is required, so mailer_autoconfirm should be false
	if resp["mailer_autoconfirm"] != false {
		t.Errorf("expected mailer_autoconfirm to be false by default, got %v", resp["mailer_autoconfirm"])
	}
}

func TestHandleSignupAnonymous(t *testing.T) {
	// Setup test server
	srv := setupTestServer(t)

	// Empty body = anonymous signup
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify response has tokens
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}

	// Verify user is anonymous
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user in response")
	}
	if user["is_anonymous"] != true {
		t.Errorf("expected is_anonymous to be true, got %v", user["is_anonymous"])
	}
	if user["email"] != nil {
		t.Errorf("expected email to be null, got %v", user["email"])
	}
}

func TestHandleSignupAnonymousWithMetadata(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"data": {"theme": "dark"}}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	user := resp["user"].(map[string]any)
	userMeta := user["user_metadata"].(map[string]any)
	if userMeta["theme"] != "dark" {
		t.Errorf("expected theme to be 'dark', got %v", userMeta["theme"])
	}
}

func TestHandleUpdateUserConvertAnonymous(t *testing.T) {
	srv := setupTestServer(t)

	// Step 1: Create anonymous user via signup
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("anonymous signup failed with status %d: %s", w.Code, w.Body.String())
	}

	var signupResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &signupResp); err != nil {
		t.Fatalf("failed to parse signup response: %v", err)
	}

	accessToken := signupResp["access_token"].(string)
	user := signupResp["user"].(map[string]any)

	// Verify user is initially anonymous
	if user["is_anonymous"] != true {
		t.Fatalf("expected user to be anonymous, got is_anonymous=%v", user["is_anonymous"])
	}

	// Step 2: Convert anonymous user by providing email and password
	updateBody := `{"email": "converted@example.com", "password": "newpassword123"}`
	req = httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update user failed with status %d: %s", w.Code, w.Body.String())
	}

	var updateResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to parse update response: %v", err)
	}

	// Verify is_anonymous is now false
	if updateResp["is_anonymous"] != false {
		t.Errorf("expected is_anonymous to be false after conversion, got %v", updateResp["is_anonymous"])
	}

	// Verify email is set
	if updateResp["email"] != "converted@example.com" {
		t.Errorf("expected email 'converted@example.com', got %v", updateResp["email"])
	}

	// Step 3: Verify the user can now login with email/password
	loginBody := `{"email": "converted@example.com", "password": "newpassword123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("login with converted user failed with status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateUserConvertAnonymousEmailInUse(t *testing.T) {
	srv := setupTestServer(t)

	// Create a regular user first
	signupAndConfirmUser(t, srv, "existing@example.com", "password123")

	// Create anonymous user
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var signupResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &signupResp)
	accessToken := signupResp["access_token"].(string)

	// Try to convert with existing email
	updateBody := `{"email": "existing@example.com", "password": "newpassword123"}`
	req = httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for email in use, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "email_exists" {
		t.Errorf("expected error 'email_exists', got %v", errResp["error"])
	}
}

func TestHandleUpdateUserConvertAnonymousShortPassword(t *testing.T) {
	srv := setupTestServer(t)

	// Create anonymous user
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var signupResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &signupResp)
	accessToken := signupResp["access_token"].(string)

	// Try to convert with short password
	updateBody := `{"email": "short@example.com", "password": "12345"}`
	req = httptest.NewRequest("PUT", "/auth/v1/user", bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for short password, got %d: %s", w.Code, w.Body.String())
	}
}
