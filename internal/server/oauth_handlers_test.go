// internal/server/oauth_handlers_test.go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/oauth"
	"github.com/stretchr/testify/assert"
)

// mockProvider implements oauth.Provider for testing
type mockProvider struct {
	name          string
	exchangeErr   error
	userInfoErr   error
	tokens        *oauth.Tokens
	userInfo      *oauth.UserInfo
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) AuthURL(state, codeChallenge, redirectURI string) string {
	return "https://mock-provider.com/auth?state=" + state
}

func (m *mockProvider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*oauth.Tokens, error) {
	if m.exchangeErr != nil {
		return nil, m.exchangeErr
	}
	return m.tokens, nil
}

func (m *mockProvider) GetUserInfo(ctx context.Context, accessToken string) (*oauth.UserInfo, error) {
	if m.userInfoErr != nil {
		return nil, m.userInfoErr
	}
	return m.userInfo, nil
}

func TestAuthorizeEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Configure Google provider
	srv.configureOAuthProvider("google", "test-client-id", "test-secret", true)

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000/callback", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	location := w.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com")
	assert.Contains(t, location, "client_id=test-client-id")
	assert.Contains(t, location, "code_challenge=")
	assert.Contains(t, location, "state=")
}

func TestAuthorizeEndpointMissingProvider(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/authorize", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointDisabledProvider(t *testing.T) {
	srv := setupTestServer(t)

	// Don't configure any providers
	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointInvalidRedirect(t *testing.T) {
	srv := setupTestServer(t)

	srv.configureOAuthProvider("google", "test-client-id", "test-secret", true)
	// Set allowed redirects
	srv.setAllowedRedirectURLs([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://evil.com/callback", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointMissingRedirectTo(t *testing.T) {
	srv := setupTestServer(t)

	srv.configureOAuthProvider("google", "test-client-id", "test-secret", true)

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointAllowedRedirectWithPath(t *testing.T) {
	srv := setupTestServer(t)

	srv.configureOAuthProvider("google", "test-client-id", "test-secret", true)
	// Set allowed redirects with specific path
	srv.setAllowedRedirectURLs([]string{"http://localhost:3000/auth"})

	// Should allow paths that start with /auth
	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000/auth/callback", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
}

func TestAuthorizeEndpointDevelopmentModeAllowsAllRedirects(t *testing.T) {
	srv := setupTestServer(t)

	srv.configureOAuthProvider("google", "test-client-id", "test-secret", true)
	// Don't set any allowed redirects (development mode)

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://any-domain.com/callback", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
}

// Callback endpoint tests

func TestCallbackEndpointInvalidState(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/callback?code=test-code&state=invalid-state", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallbackEndpointMissingCode(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/callback?state=test-state", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallbackEndpointMissingState(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/callback?code=test-code", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallbackEndpointOAuthError(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/callback?error=access_denied&error_description=User+denied+access", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// With no redirect_to, should return JSON error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallbackEndpointSuccessNewUser(t *testing.T) {
	srv := setupTestServer(t)

	// Create and register mock provider
	mock := &mockProvider{
		name: "mock",
		tokens: &oauth.Tokens{
			AccessToken:  "mock-access-token",
			RefreshToken: "mock-refresh-token",
		},
		userInfo: &oauth.UserInfo{
			ProviderID:    "12345",
			Email:         "test@example.com",
			Name:          "Test User",
			AvatarURL:     "https://example.com/avatar.jpg",
			EmailVerified: true,
		},
	}
	srv.oauthRegistry.Register(mock)

	// Store a valid flow state
	flowState := &oauth.FlowState{
		ID:           "valid-state-id",
		Provider:     "mock",
		CodeVerifier: "test-verifier",
		RedirectTo:   "http://localhost:3000/callback",
	}
	err := srv.oauthStateStore.Save(flowState)
	assert.NoError(t, err)

	// Make callback request
	req := httptest.NewRequest("GET", "/auth/v1/callback?code=auth-code&state=valid-state-id", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Should redirect with tokens in fragment
	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "http://localhost:3000/callback#"))
	assert.Contains(t, location, "access_token=")
	assert.Contains(t, location, "refresh_token=")
	assert.Contains(t, location, "token_type=bearer")
	assert.Contains(t, location, "expires_in=3600")
}

func TestCallbackEndpointAutoLinkExistingUser(t *testing.T) {
	srv := setupTestServer(t)

	// First create a user via email signup
	_, err := srv.authService.CreateUserWithOptions("test@example.com", "password123", nil, true)
	assert.NoError(t, err)

	// Create and register mock provider
	mock := &mockProvider{
		name: "mock",
		tokens: &oauth.Tokens{
			AccessToken: "mock-access-token",
		},
		userInfo: &oauth.UserInfo{
			ProviderID:    "oauth-id-12345",
			Email:         "test@example.com", // Same email as existing user
			Name:          "Test User",
			EmailVerified: true,
		},
	}
	srv.oauthRegistry.Register(mock)

	// Store a valid flow state
	flowState := &oauth.FlowState{
		ID:           "valid-state-id",
		Provider:     "mock",
		CodeVerifier: "test-verifier",
		RedirectTo:   "http://localhost:3000/callback",
	}
	err = srv.oauthStateStore.Save(flowState)
	assert.NoError(t, err)

	// Make callback request
	req := httptest.NewRequest("GET", "/auth/v1/callback?code=auth-code&state=valid-state-id", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Should redirect with tokens (auto-linked to existing user)
	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "access_token=")

	// Verify the identity was linked to the existing user
	user, err := srv.authService.GetUserByEmail("test@example.com")
	assert.NoError(t, err)

	identities, err := srv.authService.GetIdentitiesByUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, identities, 1)
	assert.Equal(t, "mock", identities[0].Provider)
	assert.Equal(t, "oauth-id-12345", identities[0].ProviderID)
}

func TestCallbackEndpointExistingIdentity(t *testing.T) {
	srv := setupTestServer(t)

	// Create and register mock provider
	mock := &mockProvider{
		name: "mock",
		tokens: &oauth.Tokens{
			AccessToken: "mock-access-token",
		},
		userInfo: &oauth.UserInfo{
			ProviderID:    "existing-oauth-id",
			Email:         "oauth-user@example.com",
			Name:          "OAuth User",
			EmailVerified: true,
		},
	}
	srv.oauthRegistry.Register(mock)

	// First OAuth sign-in (create user)
	flowState1 := &oauth.FlowState{
		ID:           "state-1",
		Provider:     "mock",
		CodeVerifier: "verifier-1",
		RedirectTo:   "http://localhost:3000/callback",
	}
	srv.oauthStateStore.Save(flowState1)

	req1 := httptest.NewRequest("GET", "/auth/v1/callback?code=code1&state=state-1", nil)
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusFound, w1.Code)

	// Second OAuth sign-in (same identity, should find existing user)
	flowState2 := &oauth.FlowState{
		ID:           "state-2",
		Provider:     "mock",
		CodeVerifier: "verifier-2",
		RedirectTo:   "http://localhost:3000/callback",
	}
	srv.oauthStateStore.Save(flowState2)

	req2 := httptest.NewRequest("GET", "/auth/v1/callback?code=code2&state=state-2", nil)
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)

	// Should succeed (returning user found by identity)
	assert.Equal(t, http.StatusFound, w2.Code)
	location := w2.Header().Get("Location")
	assert.Contains(t, location, "access_token=")
}

func TestCallbackEndpointProviderUnavailable(t *testing.T) {
	srv := setupTestServer(t)

	// Store flow state for a provider that will be removed
	flowState := &oauth.FlowState{
		ID:           "orphan-state",
		Provider:     "nonexistent",
		CodeVerifier: "test-verifier",
		RedirectTo:   "http://localhost:3000/callback",
	}
	srv.oauthStateStore.Save(flowState)

	// Make callback request - provider doesn't exist
	req := httptest.NewRequest("GET", "/auth/v1/callback?code=auth-code&state=orphan-state", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Should redirect with error
	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "error=provider_error")
}

func TestCallbackEndpointOAuthErrorWithRedirect(t *testing.T) {
	srv := setupTestServer(t)

	// Register a mock provider
	mock := &mockProvider{name: "mock"}
	srv.oauthRegistry.Register(mock)

	// Store flow state
	flowState := &oauth.FlowState{
		ID:           "error-state",
		Provider:     "mock",
		CodeVerifier: "test-verifier",
		RedirectTo:   "http://localhost:3000/callback",
	}
	srv.oauthStateStore.Save(flowState)

	// OAuth error with state that has redirect_to - but errors from provider come before state lookup
	// So error without state should return JSON
	req := httptest.NewRequest("GET", "/auth/v1/callback?error=access_denied&error_description=User+denied", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Without redirect_to context, returns JSON error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Identity list and unlink endpoint tests

func TestGetIdentitiesEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and identity
	user, err := srv.authService.CreateUserWithOptions("test@example.com", "password123", nil, true)
	assert.NoError(t, err)

	err = srv.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	assert.NoError(t, err)

	// Create access token
	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/auth/v1/user/identities", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Identities []*auth.Identity `json:"identities"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Identities, 1)
	assert.Equal(t, "google", resp.Identities[0].Provider)
}

func TestGetIdentitiesEndpointNoIdentities(t *testing.T) {
	srv := setupTestServer(t)

	// Create user without any OAuth identities
	user, err := srv.authService.CreateUserWithOptions("test@example.com", "password123", nil, true)
	assert.NoError(t, err)

	// Create access token
	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/auth/v1/user/identities", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Identities []*auth.Identity `json:"identities"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Identities, 0)
}

func TestGetIdentitiesEndpointUnauthorized(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/user/identities", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUnlinkIdentityEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user with password (so unlinking OAuth is allowed)
	user, err := srv.authService.CreateUserWithOptions("test@example.com", "password123", nil, true)
	assert.NoError(t, err)

	err = srv.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	assert.NoError(t, err)

	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify identity is deleted
	identities, err := srv.authService.GetIdentitiesByUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, identities, 0)
}

func TestUnlinkIdentityEndpointWithMultipleIdentities(t *testing.T) {
	srv := setupTestServer(t)

	// Create OAuth-only user (no password) but with multiple identities
	user, err := srv.authService.CreateOAuthUser("test@example.com", "google", nil)
	assert.NoError(t, err)

	// Create two identities
	err = srv.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	assert.NoError(t, err)

	err = srv.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "github",
		ProviderID: "github-456",
	})
	assert.NoError(t, err)

	// Add github provider to app_metadata
	srv.authService.AddProviderToUser(user.ID, "github")

	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	// Unlinking one identity should be allowed since there's another
	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify only github identity remains
	identities, err := srv.authService.GetIdentitiesByUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, identities, 1)
	assert.Equal(t, "github", identities[0].Provider)
}

func TestUnlinkLastIdentityBlocked(t *testing.T) {
	srv := setupTestServer(t)

	// Create OAuth-only user (no password)
	user, err := srv.authService.CreateOAuthUser("test@example.com", "google", nil)
	assert.NoError(t, err)

	err = srv.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	assert.NoError(t, err)

	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Should be blocked - can't remove last auth method
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify identity still exists
	identities, err := srv.authService.GetIdentitiesByUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, identities, 1)
}

func TestUnlinkIdentityNotFound(t *testing.T) {
	srv := setupTestServer(t)

	// Create user with password
	user, err := srv.authService.CreateUserWithOptions("test@example.com", "password123", nil, true)
	assert.NoError(t, err)

	session, _, err := srv.authService.CreateSession(user)
	assert.NoError(t, err)
	accessToken, err := srv.authService.GenerateAccessToken(user, session.ID)
	assert.NoError(t, err)

	// Try to unlink identity that doesn't exist
	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUnlinkIdentityUnauthorized(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
