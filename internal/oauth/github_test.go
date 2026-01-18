package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubProviderName(t *testing.T) {
	provider := NewGitHubProvider(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/v1/callback",
	})

	assert.Equal(t, "github", provider.Name())
}

func TestGitHubAuthURL(t *testing.T) {
	provider := NewGitHubProvider(Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/v1/callback",
	})

	url := provider.AuthURL("test-state", "test-challenge", "http://localhost:8080/auth/v1/callback")

	assert.Contains(t, url, "github.com/login/oauth/authorize")
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "state=test-state")
	assert.Contains(t, url, "scope=read%3Auser")
}

func TestGitHubProviderImplementsInterface(t *testing.T) {
	provider := NewGitHubProvider(Config{})
	var _ Provider = provider
	require.NotNil(t, provider)
}
