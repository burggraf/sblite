package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOAuthSettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Set up session
	sessionToken := setupTestSession(t, handler)

	req := httptest.NewRequest("GET", "/api/settings/oauth", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "google")
	assert.Contains(t, resp, "github")
}

func TestGetOAuthSettingsUnauthorized(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/settings/oauth", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateOAuthSettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	body := `{
		"google": {
			"client_id": "test-google-id",
			"client_secret": "test-google-secret",
			"enabled": true
		}
	}`

	req := httptest.NewRequest("PATCH", "/api/settings/oauth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify settings were saved
	req2 := httptest.NewRequest("GET", "/api/settings/oauth", nil)
	req2.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	var resp map[string]interface{}
	err := json.NewDecoder(w2.Body).Decode(&resp)
	require.NoError(t, err)
	google := resp["google"].(map[string]interface{})
	assert.Equal(t, "test-google-id", google["client_id"])
	assert.True(t, google["enabled"].(bool))
	// Secret should be masked
	assert.Equal(t, "********", google["client_secret"])
}

func TestUpdateOAuthSettingsPreserveSecret(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	// First, set the original secret
	body1 := `{
		"google": {
			"client_id": "test-google-id",
			"client_secret": "original-secret",
			"enabled": true
		}
	}`

	req1 := httptest.NewRequest("PATCH", "/api/settings/oauth", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w1 := httptest.NewRecorder()

	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Now update with masked secret (should preserve original)
	body2 := `{
		"google": {
			"client_id": "new-google-id",
			"client_secret": "********",
			"enabled": false
		}
	}`

	req2 := httptest.NewRequest("PATCH", "/api/settings/oauth", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify client_id was updated but secret preserved (still shows masked)
	req3 := httptest.NewRequest("GET", "/api/settings/oauth", nil)
	req3.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w3 := httptest.NewRecorder()

	r.ServeHTTP(w3, req3)

	var resp map[string]interface{}
	err := json.NewDecoder(w3.Body).Decode(&resp)
	require.NoError(t, err)
	google := resp["google"].(map[string]interface{})
	assert.Equal(t, "new-google-id", google["client_id"])
	assert.False(t, google["enabled"].(bool))
	// Secret should still be masked (original preserved)
	assert.Equal(t, "********", google["client_secret"])

	// Verify the actual secret was preserved in storage
	secret, err := handler.store.Get("oauth_google_client_secret")
	require.NoError(t, err)
	assert.Equal(t, "original-secret", secret)
}

func TestRedirectURLManagement(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	// Add redirect URL
	body := `{"url": "http://localhost:3000/callback"}`
	req := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// List redirect URLs
	req2 := httptest.NewRequest("GET", "/api/settings/oauth/redirect-urls", nil)
	req2.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	var resp struct {
		URLs []string `json:"urls"`
	}
	err := json.NewDecoder(w2.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp.URLs, "http://localhost:3000/callback")
}

func TestRedirectURLDuplicate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	// Add redirect URL
	body := `{"url": "http://localhost:3000/callback"}`
	req := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Try to add same URL again
	req2 := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestRedirectURLDelete(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	// Add two redirect URLs
	body1 := `{"url": "http://localhost:3000/callback"}`
	req1 := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	body2 := `{"url": "http://localhost:4000/callback"}`
	req2 := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)

	// Delete the first URL
	deleteBody := `{"url": "http://localhost:3000/callback"}`
	req3 := httptest.NewRequest("DELETE", "/api/settings/oauth/redirect-urls", strings.NewReader(deleteBody))
	req3.Header.Set("Content-Type", "application/json")
	req3.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	// Verify only second URL remains
	req4 := httptest.NewRequest("GET", "/api/settings/oauth/redirect-urls", nil)
	req4.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)

	var resp struct {
		URLs []string `json:"urls"`
	}
	err := json.NewDecoder(w4.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp.URLs, 1)
	assert.Contains(t, resp.URLs, "http://localhost:4000/callback")
	assert.NotContains(t, resp.URLs, "http://localhost:3000/callback")
}

func TestRedirectURLEmptyURL(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, t.TempDir()+"/migrations")

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	sessionToken := setupTestSession(t, handler)

	// Try to add empty URL
	body := `{"url": ""}`
	req := httptest.NewRequest("POST", "/api/settings/oauth/redirect-urls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: sessionToken})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"secret123", "********"},
		{"a", "********"},
	}

	for _, tt := range tests {
		result := maskSecret(tt.input)
		assert.Equal(t, tt.expected, result, "maskSecret(%q) should be %q", tt.input, tt.expected)
	}
}

func TestBoolToString(t *testing.T) {
	assert.Equal(t, "true", boolToString(true))
	assert.Equal(t, "false", boolToString(false))
}
