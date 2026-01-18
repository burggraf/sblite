package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleProviderName(t *testing.T) {
	provider := NewGoogleProvider(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/v1/callback",
	})

	assert.Equal(t, "google", provider.Name())
}

func TestGoogleAuthURL(t *testing.T) {
	provider := NewGoogleProvider(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/v1/callback",
	})

	url := provider.AuthURL("test-state", "test-challenge", "http://localhost:8080/auth/v1/callback")

	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "state=test-state")
	assert.Contains(t, url, "code_challenge=test-challenge")
	assert.Contains(t, url, "code_challenge_method=S256")
	assert.Contains(t, url, "scope=openid")
	assert.Contains(t, url, "email")
	assert.Contains(t, url, "profile")
}

func TestGoogleProviderImplementsInterface(t *testing.T) {
	provider := NewGoogleProvider(Config{})
	var _ Provider = provider
	require.NotNil(t, provider)
}
