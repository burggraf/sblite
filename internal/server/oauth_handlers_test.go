// internal/server/oauth_handlers_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
