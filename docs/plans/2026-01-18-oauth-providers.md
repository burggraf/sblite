# OAuth Providers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Google and GitHub OAuth authentication support, compatible with Supabase's `@supabase/supabase-js` client.

**Architecture:** New `internal/oauth/` package with Provider interface. PKCE flow with server-side state storage. Auto-link by email, store identities in separate table. Dashboard UI for configuration.

**Tech Stack:** Go, `golang.org/x/oauth2`, Chi router, SQLite

---

## Task 1: Add Database Schema

**Files:**
- Modify: `internal/db/migrations.go`
- Test: `internal/db/db_test.go`

**Step 1: Write the failing test**

Add to `internal/db/db_test.go`:

```go
func TestOAuthTablesExist(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Check auth_identities table exists
	var identitiesExists int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_identities'").Scan(&identitiesExists)
	require.NoError(t, err)
	assert.Equal(t, 1, identitiesExists, "auth_identities table should exist")

	// Check auth_flow_state table exists
	var flowStateExists int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_flow_state'").Scan(&flowStateExists)
	require.NoError(t, err)
	assert.Equal(t, 1, flowStateExists, "auth_flow_state table should exist")

	// Verify auth_identities columns
	_, err = db.Exec(`INSERT INTO auth_identities (id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at)
		VALUES ('test-id', 'user-id', 'google', 'google-123', '{}', datetime('now'), datetime('now'), datetime('now'))`)
	// Will fail due to foreign key, but proves columns exist
	assert.Error(t, err) // FK constraint

	// Verify auth_flow_state columns
	_, err = db.Exec(`INSERT INTO auth_flow_state (id, provider, code_verifier, redirect_to, created_at, expires_at)
		VALUES ('state-123', 'google', 'verifier', 'https://example.com', datetime('now'), datetime('now', '+10 minutes'))`)
	require.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -run TestOAuthTablesExist -v`
Expected: FAIL - tables don't exist

**Step 3: Add schema migrations**

In `internal/db/migrations.go`, add to the `migrations` slice:

```go
// OAuth: auth_identities table
`CREATE TABLE IF NOT EXISTS auth_identities (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	provider TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	identity_data TEXT,
	last_sign_in_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_identities_user ON auth_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_identities_provider ON auth_identities(provider, provider_id);`,

// OAuth: auth_flow_state table (PKCE state storage)
`CREATE TABLE IF NOT EXISTS auth_flow_state (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	code_verifier TEXT NOT NULL,
	redirect_to TEXT,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);`,
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -run TestOAuthTablesExist -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/migrations.go internal/db/db_test.go
git commit -m "feat(oauth): add auth_identities and auth_flow_state tables"
```

---

## Task 2: Create PKCE Implementation

**Files:**
- Create: `internal/oauth/pkce.go`
- Create: `internal/oauth/pkce_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/pkce_test.go`:

```go
package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	require.NoError(t, err)

	// RFC 7636: verifier must be 43-128 characters
	assert.GreaterOrEqual(t, len(verifier), 43)
	assert.LessOrEqual(t, len(verifier), 128)

	// Should be URL-safe base64
	for _, c := range verifier {
		assert.True(t, isURLSafeBase64Char(c), "character %c should be URL-safe", c)
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)

	// Manually compute expected challenge
	hash := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])

	assert.Equal(t, expected, challenge)
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	require.NoError(t, err)

	// State should be at least 32 characters for security
	assert.GreaterOrEqual(t, len(state), 32)
}

func isURLSafeBase64Char(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_'
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -v`
Expected: FAIL - package doesn't exist

**Step 3: Write minimal implementation**

Create `internal/oauth/pkce.go`:

```go
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateCodeVerifier generates a cryptographically random code verifier
// for PKCE (RFC 7636). Returns a 43-character URL-safe string.
func GenerateCodeVerifier() (string, error) {
	// 32 bytes = 43 characters in base64url (without padding)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge computes the S256 code challenge from a verifier.
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// GenerateState generates a cryptographically random state parameter
// for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/pkce.go internal/oauth/pkce_test.go
git commit -m "feat(oauth): add PKCE code verifier and challenge generation"
```

---

## Task 3: Create Flow State Storage

**Files:**
- Create: `internal/oauth/state.go`
- Create: `internal/oauth/state_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/state_test.go`:

```go
package oauth

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	database, err := db.New(":memory:")
	require.NoError(t, err)
	return database, func() { database.Close() }
}

func TestStoreAndRetrieveFlowState(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database)

	state := &FlowState{
		ID:           "test-state-123",
		Provider:     "google",
		CodeVerifier: "test-verifier-abc",
		RedirectTo:   "https://example.com/callback",
	}

	err := store.Save(state)
	require.NoError(t, err)

	retrieved, err := store.Get("test-state-123")
	require.NoError(t, err)
	assert.Equal(t, state.Provider, retrieved.Provider)
	assert.Equal(t, state.CodeVerifier, retrieved.CodeVerifier)
	assert.Equal(t, state.RedirectTo, retrieved.RedirectTo)
}

func TestFlowStateExpiry(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database)

	// Create expired state by manipulating time
	state := &FlowState{
		ID:           "expired-state",
		Provider:     "google",
		CodeVerifier: "verifier",
		RedirectTo:   "https://example.com",
	}
	err := store.Save(state)
	require.NoError(t, err)

	// Manually expire it
	_, err = database.Exec("UPDATE auth_flow_state SET expires_at = datetime('now', '-1 minute') WHERE id = ?", state.ID)
	require.NoError(t, err)

	// Should not be retrievable
	_, err = store.Get("expired-state")
	assert.Error(t, err)
}

func TestDeleteFlowState(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStateStore(database)

	state := &FlowState{
		ID:           "to-delete",
		Provider:     "github",
		CodeVerifier: "verifier",
	}
	err := store.Save(state)
	require.NoError(t, err)

	err = store.Delete("to-delete")
	require.NoError(t, err)

	_, err = store.Get("to-delete")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -run TestStore -v`
Expected: FAIL - types not defined

**Step 3: Write minimal implementation**

Create `internal/oauth/state.go`:

```go
package oauth

import (
	"database/sql"
	"errors"
	"time"
)

var ErrStateNotFound = errors.New("oauth state not found or expired")

// FlowState represents the OAuth flow state stored during PKCE flow.
type FlowState struct {
	ID           string
	Provider     string
	CodeVerifier string
	RedirectTo   string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// StateStore manages OAuth flow state in the database.
type StateStore struct {
	db *sql.DB
}

// NewStateStore creates a new state store.
func NewStateStore(db *sql.DB) *StateStore {
	return &StateStore{db: db}
}

// Save stores a new flow state with 10-minute expiry.
func (s *StateStore) Save(state *FlowState) error {
	now := time.Now().UTC()
	expiresAt := now.Add(10 * time.Minute)

	_, err := s.db.Exec(`
		INSERT INTO auth_flow_state (id, provider, code_verifier, redirect_to, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		state.ID, state.Provider, state.CodeVerifier, state.RedirectTo,
		now.Format(time.RFC3339), expiresAt.Format(time.RFC3339))
	return err
}

// Get retrieves and validates a flow state. Returns error if expired or not found.
func (s *StateStore) Get(id string) (*FlowState, error) {
	var state FlowState
	var createdAt, expiresAt string

	err := s.db.QueryRow(`
		SELECT id, provider, code_verifier, redirect_to, created_at, expires_at
		FROM auth_flow_state
		WHERE id = ? AND expires_at > datetime('now')`,
		id).Scan(&state.ID, &state.Provider, &state.CodeVerifier, &state.RedirectTo, &createdAt, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, err
	}

	state.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	state.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)

	return &state, nil
}

// Delete removes a flow state (called after successful exchange).
func (s *StateStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM auth_flow_state WHERE id = ?", id)
	return err
}

// CleanupExpired removes all expired flow states.
func (s *StateStore) CleanupExpired() error {
	_, err := s.db.Exec("DELETE FROM auth_flow_state WHERE expires_at <= datetime('now')")
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -run TestStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/state.go internal/oauth/state_test.go
git commit -m "feat(oauth): add flow state storage for PKCE"
```

---

## Task 4: Create Provider Interface and Types

**Files:**
- Create: `internal/oauth/oauth.go`
- Create: `internal/oauth/oauth_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/oauth_test.go`:

```go
package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserInfoValidation(t *testing.T) {
	tests := []struct {
		name    string
		info    *UserInfo
		wantErr bool
	}{
		{
			name: "valid user info",
			info: &UserInfo{
				ProviderID:    "123456",
				Email:         "user@example.com",
				Name:          "Test User",
				EmailVerified: true,
			},
			wantErr: false,
		},
		{
			name: "missing provider ID",
			info: &UserInfo{
				Email: "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "missing email",
			info: &UserInfo{
				ProviderID: "123456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -run TestUserInfo -v`
Expected: FAIL - types not defined

**Step 3: Write minimal implementation**

Create `internal/oauth/oauth.go`:

```go
package oauth

import (
	"context"
	"errors"
)

var (
	ErrProviderNotFound    = errors.New("oauth provider not found")
	ErrProviderNotEnabled  = errors.New("oauth provider not enabled")
	ErrInvalidUserInfo     = errors.New("invalid user info from provider")
	ErrEmailRequired       = errors.New("email is required from oauth provider")
	ErrProviderIDRequired  = errors.New("provider ID is required")
)

// Provider defines the interface for OAuth providers.
type Provider interface {
	// Name returns the provider identifier (e.g., "google", "github").
	Name() string

	// AuthURL generates the authorization URL for initiating OAuth flow.
	AuthURL(state, codeChallenge, redirectURI string) string

	// ExchangeCode exchanges an authorization code for tokens.
	ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Tokens, error)

	// GetUserInfo fetches user information using the access token.
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
}

// Tokens holds the OAuth tokens returned by the provider.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int
}

// UserInfo holds user information from the OAuth provider.
type UserInfo struct {
	ProviderID    string `json:"provider_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url"`
	EmailVerified bool   `json:"email_verified"`
}

// Validate checks that required fields are present.
func (u *UserInfo) Validate() error {
	if u.ProviderID == "" {
		return ErrProviderIDRequired
	}
	if u.Email == "" {
		return ErrEmailRequired
	}
	return nil
}

// Config holds OAuth provider configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	Enabled      bool
	RedirectURL  string // sblite's callback URL
}

// Identity represents a stored OAuth identity linked to a user.
type Identity struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Provider     string    `json:"provider"`
	ProviderID   string    `json:"provider_id"`
	IdentityData *UserInfo `json:"identity_data"`
	LastSignInAt string    `json:"last_sign_in_at"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -run TestUserInfo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/oauth.go internal/oauth/oauth_test.go
git commit -m "feat(oauth): add provider interface and core types"
```

---

## Task 5: Implement Google Provider

**Files:**
- Create: `internal/oauth/google.go`
- Create: `internal/oauth/google_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/google_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -run TestGoogle -v`
Expected: FAIL - NewGoogleProvider not defined

**Step 3: Write minimal implementation**

First, add the dependency:

```bash
go get golang.org/x/oauth2
```

Create `internal/oauth/google.go`:

```go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleProvider implements OAuth for Google.
type GoogleProvider struct {
	config *oauth2.Config
}

// NewGoogleProvider creates a new Google OAuth provider.
func NewGoogleProvider(cfg Config) *GoogleProvider {
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// Name returns "google".
func (g *GoogleProvider) Name() string {
	return "google"
}

// AuthURL generates the Google authorization URL with PKCE.
func (g *GoogleProvider) AuthURL(state, codeChallenge, redirectURI string) string {
	cfg := *g.config
	if redirectURI != "" {
		cfg.RedirectURL = redirectURI
	}

	return cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.AccessTypeOffline,
	)
}

// ExchangeCode exchanges the authorization code for tokens using PKCE.
func (g *GoogleProvider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Tokens, error) {
	cfg := *g.config
	if redirectURI != "" {
		cfg.RedirectURL = redirectURI
	}

	token, err := cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("google token exchange failed: %w", err)
	}

	return &Tokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      token.Extra("id_token").(string),
		TokenType:    token.TokenType,
	}, nil
}

// GetUserInfo fetches user information from Google.
func (g *GoogleProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google userinfo returned %d: %s", resp.StatusCode, body)
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode google userinfo: %w", err)
	}

	return &UserInfo{
		ProviderID:    googleUser.ID,
		Email:         googleUser.Email,
		Name:          googleUser.Name,
		AvatarURL:     googleUser.Picture,
		EmailVerified: googleUser.VerifiedEmail,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -run TestGoogle -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/google.go internal/oauth/google_test.go go.mod go.sum
git commit -m "feat(oauth): implement Google provider with PKCE"
```

---

## Task 6: Implement GitHub Provider

**Files:**
- Create: `internal/oauth/github.go`
- Create: `internal/oauth/github_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/github_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -run TestGitHub -v`
Expected: FAIL - NewGitHubProvider not defined

**Step 3: Write minimal implementation**

Create `internal/oauth/github.go`:

```go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
	githubEmailsURL    = "https://api.github.com/user/emails"
)

// GitHubProvider implements OAuth for GitHub.
type GitHubProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
}

// NewGitHubProvider creates a new GitHub OAuth provider.
func NewGitHubProvider(cfg Config) *GitHubProvider {
	return &GitHubProvider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
	}
}

// Name returns "github".
func (g *GitHubProvider) Name() string {
	return "github"
}

// AuthURL generates the GitHub authorization URL.
// Note: GitHub doesn't support PKCE, but we include state for CSRF protection.
func (g *GitHubProvider) AuthURL(state, codeChallenge, redirectURI string) string {
	redirect := g.redirectURL
	if redirectURI != "" {
		redirect = redirectURI
	}

	params := url.Values{
		"client_id":    {g.clientID},
		"redirect_uri": {redirect},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}

	return githubAuthorizeURL + "?" + params.Encode()
}

// ExchangeCode exchanges the authorization code for tokens.
func (g *GitHubProvider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Tokens, error) {
	redirect := g.redirectURL
	if redirectURI != "" {
		redirect = redirectURI
	}

	data := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"code":          {code},
		"redirect_uri":  {redirect},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode github token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &Tokens{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
	}, nil
}

// GetUserInfo fetches user information from GitHub.
func (g *GitHubProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	// Fetch basic user info
	userReq, err := http.NewRequestWithContext(ctx, "GET", githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	userReq.Header.Set("Authorization", "Bearer "+accessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(userResp.Body)
		return nil, fmt.Errorf("github user returned %d: %s", userResp.StatusCode, body)
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode github user: %w", err)
	}

	// If email is not public, fetch from emails endpoint
	email := githubUser.Email
	verified := false
	if email == "" {
		email, verified, err = g.fetchPrimaryEmail(ctx, accessToken)
		if err != nil {
			return nil, err
		}
	} else {
		verified = true // Public emails are verified
	}

	name := githubUser.Name
	if name == "" {
		name = githubUser.Login
	}

	return &UserInfo{
		ProviderID:    fmt.Sprintf("%d", githubUser.ID),
		Email:         email,
		Name:          name,
		AvatarURL:     githubUser.AvatarURL,
		EmailVerified: verified,
	}, nil
}

// fetchPrimaryEmail gets the user's primary verified email from GitHub.
func (g *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubEmailsURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("github emails request failed: %w", err)
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("failed to decode github emails: %w", err)
	}

	// Find primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, true, nil
		}
	}

	// Fall back to any verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, true, nil
		}
	}

	return "", false, ErrEmailRequired
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -run TestGitHub -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/github.go internal/oauth/github_test.go
git commit -m "feat(oauth): implement GitHub provider"
```

---

## Task 7: Add Identity Storage to Auth Package

**Files:**
- Modify: `internal/auth/user.go`
- Create: `internal/auth/identity.go`
- Create: `internal/auth/identity_test.go`

**Step 1: Write the failing test**

Create `internal/auth/identity_test.go`:

```go
package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIdentity(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	// Create a user first
	user, err := svc.CreateUser("test@example.com", "password123", nil, nil, true)
	require.NoError(t, err)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123456",
		IdentityData: map[string]interface{}{
			"email":      "test@example.com",
			"name":       "Test User",
			"avatar_url": "https://example.com/avatar.jpg",
		},
	}

	err = svc.CreateIdentity(identity)
	require.NoError(t, err)
	assert.NotEmpty(t, identity.ID)
}

func TestGetIdentityByProvider(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUser("test@example.com", "password123", nil, nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "github",
		ProviderID: "github-789",
		IdentityData: map[string]interface{}{
			"email": "test@example.com",
		},
	}
	svc.CreateIdentity(identity)

	found, err := svc.GetIdentityByProvider("github", "github-789")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.UserID)
}

func TestGetIdentitiesByUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUser("test@example.com", "password123", nil, nil, true)

	svc.CreateIdentity(&Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})
	svc.CreateIdentity(&Identity{
		UserID:     user.ID,
		Provider:   "github",
		ProviderID: "github-456",
	})

	identities, err := svc.GetIdentitiesByUser(user.ID)
	require.NoError(t, err)
	assert.Len(t, identities, 2)
}

func TestDeleteIdentity(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	user, _ := svc.CreateUser("test@example.com", "password123", nil, nil, true)

	identity := &Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	}
	svc.CreateIdentity(identity)

	err := svc.DeleteIdentity(user.ID, "google")
	require.NoError(t, err)

	_, err = svc.GetIdentityByProvider("google", "google-123")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestIdentity -v`
Expected: FAIL - Identity type not defined

**Step 3: Write minimal implementation**

Create `internal/auth/identity.go`:

```go
package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrIdentityNotFound     = errors.New("identity not found")
	ErrIdentityAlreadyExists = errors.New("identity already exists")
)

// Identity represents an OAuth identity linked to a user.
type Identity struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	Provider     string                 `json:"provider"`
	ProviderID   string                 `json:"provider_id"`
	IdentityData map[string]interface{} `json:"identity_data"`
	LastSignInAt *time.Time             `json:"last_sign_in_at"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// CreateIdentity stores a new OAuth identity.
func (s *Service) CreateIdentity(identity *Identity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	identity.CreatedAt = now
	identity.UpdatedAt = now

	identityDataJSON, err := json.Marshal(identity.IdentityData)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO auth_identities (id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		identity.ID, identity.UserID, identity.Provider, identity.ProviderID,
		string(identityDataJSON), nil,
		now.Format(time.RFC3339), now.Format(time.RFC3339))

	return err
}

// GetIdentityByProvider finds an identity by provider and provider ID.
func (s *Service) GetIdentityByProvider(provider, providerID string) (*Identity, error) {
	var identity Identity
	var identityDataJSON string
	var lastSignInAt, createdAt, updatedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at
		FROM auth_identities
		WHERE provider = ? AND provider_id = ?`,
		provider, providerID).Scan(
		&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderID,
		&identityDataJSON, &lastSignInAt, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrIdentityNotFound
	}
	if err != nil {
		return nil, err
	}

	if identityDataJSON != "" {
		json.Unmarshal([]byte(identityDataJSON), &identity.IdentityData)
	}
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		identity.CreatedAt = t
	}
	if updatedAt.Valid {
		t, _ := time.Parse(time.RFC3339, updatedAt.String)
		identity.UpdatedAt = t
	}
	if lastSignInAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
		identity.LastSignInAt = &t
	}

	return &identity, nil
}

// GetIdentitiesByUser returns all identities for a user.
func (s *Service) GetIdentitiesByUser(userID string) ([]*Identity, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, provider, provider_id, identity_data, last_sign_in_at, created_at, updated_at
		FROM auth_identities
		WHERE user_id = ?
		ORDER BY created_at`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []*Identity
	for rows.Next() {
		var identity Identity
		var identityDataJSON string
		var lastSignInAt, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderID,
			&identityDataJSON, &lastSignInAt, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		if identityDataJSON != "" {
			json.Unmarshal([]byte(identityDataJSON), &identity.IdentityData)
		}
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			identity.CreatedAt = t
		}
		if updatedAt.Valid {
			t, _ := time.Parse(time.RFC3339, updatedAt.String)
			identity.UpdatedAt = t
		}
		if lastSignInAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
			identity.LastSignInAt = &t
		}

		identities = append(identities, &identity)
	}

	return identities, nil
}

// DeleteIdentity removes an OAuth identity.
func (s *Service) DeleteIdentity(userID, provider string) error {
	result, err := s.db.Exec(`
		DELETE FROM auth_identities
		WHERE user_id = ? AND provider = ?`,
		userID, provider)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrIdentityNotFound
	}

	return nil
}

// UpdateIdentityLastSignIn updates the last sign in time for an identity.
func (s *Service) UpdateIdentityLastSignIn(provider, providerID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE auth_identities
		SET last_sign_in_at = ?, updated_at = ?
		WHERE provider = ? AND provider_id = ?`,
		now, now, provider, providerID)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/... -run TestIdentity -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/identity.go internal/auth/identity_test.go
git commit -m "feat(oauth): add identity storage to auth package"
```

---

## Task 8: Create Provider Registry

**Files:**
- Create: `internal/oauth/registry.go`
- Create: `internal/oauth/registry_test.go`

**Step 1: Write the failing test**

Create `internal/oauth/registry_test.go`:

```go
package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryGetProvider(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{
		ClientID:     "google-id",
		ClientSecret: "google-secret",
		Enabled:      true,
	}))

	provider, err := registry.Get("google")
	require.NoError(t, err)
	assert.Equal(t, "google", provider.Name())
}

func TestRegistryProviderNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestRegistryListEnabled(t *testing.T) {
	registry := NewRegistry()

	registry.Register(NewGoogleProvider(Config{Enabled: true}))
	registry.Register(NewGitHubProvider(Config{Enabled: true}))

	providers := registry.ListEnabled()
	assert.Len(t, providers, 2)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/... -run TestRegistry -v`
Expected: FAIL - Registry not defined

**Step 3: Write minimal implementation**

Create `internal/oauth/registry.go`:

```go
package oauth

import (
	"sync"
)

// Registry manages available OAuth providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	enabled   map[string]bool
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		enabled:   make(map[string]bool),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
	r.enabled[provider.Name()] = true
}

// RegisterWithConfig adds a provider with explicit enabled state.
func (r *Registry) RegisterWithConfig(provider Provider, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
	r.enabled[provider.Name()] = enabled
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}

	if !r.enabled[name] {
		return nil, ErrProviderNotEnabled
	}

	return provider, nil
}

// IsEnabled checks if a provider is enabled.
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[name]
}

// SetEnabled enables or disables a provider.
func (r *Registry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[name] = enabled
}

// ListEnabled returns names of all enabled providers.
func (r *Registry) ListEnabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, enabled := range r.enabled {
		if enabled {
			names = append(names, name)
		}
	}
	return names
}

// ListAll returns names of all registered providers.
func (r *Registry) ListAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/... -run TestRegistry -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/oauth/registry.go internal/oauth/registry_test.go
git commit -m "feat(oauth): add provider registry"
```

---

## Task 9: Add OAuth Handlers - Authorize Endpoint

**Files:**
- Modify: `internal/server/server.go`
- Create: `internal/server/oauth_handlers.go`
- Create: `internal/server/oauth_handlers_test.go`

**Step 1: Write the failing test**

Create `internal/server/oauth_handlers_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Configure Google provider
	s.configureOAuthProvider("google", "test-client-id", "test-secret", true)

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000/callback", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	location := w.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com")
	assert.Contains(t, location, "client_id=test-client-id")
	assert.Contains(t, location, "code_challenge=")
	assert.Contains(t, location, "state=")
}

func TestAuthorizeEndpointMissingProvider(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/v1/authorize", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointDisabledProvider(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Don't configure any providers
	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthorizeEndpointInvalidRedirect(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	s.configureOAuthProvider("google", "test-client-id", "test-secret", true)
	// Set allowed redirects
	s.setAllowedRedirectURLs([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/auth/v1/authorize?provider=google&redirect_to=http://evil.com/callback", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -run TestAuthorize -v`
Expected: FAIL - handlers not defined

**Step 3: Write minimal implementation**

Create `internal/server/oauth_handlers.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/markb/sblite/internal/oauth"
)

// handleAuthorize initiates the OAuth flow.
// GET /auth/v1/authorize?provider=google&redirect_to=...
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	redirectTo := r.URL.Query().Get("redirect_to")

	if providerName == "" {
		s.jsonError(w, http.StatusBadRequest, "provider parameter is required")
		return
	}

	if redirectTo == "" {
		s.jsonError(w, http.StatusBadRequest, "redirect_to parameter is required")
		return
	}

	// Validate redirect URL is allowed
	if !s.isRedirectURLAllowed(redirectTo) {
		s.jsonError(w, http.StatusBadRequest, "redirect_to URL is not allowed")
		return
	}

	// Get provider
	provider, err := s.oauthRegistry.Get(providerName)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "provider not found or not enabled")
		return
	}

	// Generate PKCE values
	codeVerifier, err := oauth.GenerateCodeVerifier()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to generate code verifier")
		return
	}
	codeChallenge := oauth.GenerateCodeChallenge(codeVerifier)

	// Generate state
	state, err := oauth.GenerateState()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	// Store flow state
	flowState := &oauth.FlowState{
		ID:           state,
		Provider:     providerName,
		CodeVerifier: codeVerifier,
		RedirectTo:   redirectTo,
	}
	if err := s.oauthStateStore.Save(flowState); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to save flow state")
		return
	}

	// Generate auth URL and redirect
	callbackURL := s.getCallbackURL()
	authURL := provider.AuthURL(state, codeChallenge, callbackURL)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// isRedirectURLAllowed checks if the redirect URL is in the allowed list.
func (s *Server) isRedirectURLAllowed(redirectURL string) bool {
	// If no allowed URLs configured, allow all (development mode)
	if len(s.allowedRedirectURLs) == 0 {
		return true
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	for _, allowed := range s.allowedRedirectURLs {
		allowedParsed, err := url.Parse(allowed)
		if err != nil {
			continue
		}

		// Match scheme and host
		if parsed.Scheme == allowedParsed.Scheme && parsed.Host == allowedParsed.Host {
			// If allowed URL has a path, redirect must start with it
			if allowedParsed.Path != "" && allowedParsed.Path != "/" {
				if strings.HasPrefix(parsed.Path, allowedParsed.Path) {
					return true
				}
			} else {
				return true
			}
		}
	}

	return false
}

// getCallbackURL returns the OAuth callback URL for this server.
func (s *Server) getCallbackURL() string {
	return s.baseURL + "/auth/v1/callback"
}

// jsonError writes a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             http.StatusText(status),
		"error_description": message,
	})
}
```

Add to `internal/server/server.go` - add fields to Server struct:

```go
// Add to Server struct
oauthRegistry       *oauth.Registry
oauthStateStore     *oauth.StateStore
allowedRedirectURLs []string
baseURL             string
```

Add route registration in `setupRoutes()`:

```go
// In the /auth/v1 route group, add:
r.Get("/authorize", s.handleAuthorize)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -run TestAuthorize -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/oauth_handlers.go internal/server/oauth_handlers_test.go internal/server/server.go
git commit -m "feat(oauth): add authorize endpoint"
```

---

## Task 10: Add OAuth Handlers - Callback Endpoint

**Files:**
- Modify: `internal/server/oauth_handlers.go`
- Modify: `internal/server/oauth_handlers_test.go`

**Step 1: Write the failing test**

Add to `internal/server/oauth_handlers_test.go`:

```go
func TestCallbackEndpointInvalidState(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/v1/callback?code=test-code&state=invalid-state", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallbackEndpointMissingCode(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/v1/callback?state=test-state", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -run TestCallback -v`
Expected: FAIL - callback handler not defined

**Step 3: Write minimal implementation**

Add to `internal/server/oauth_handlers.go`:

```go
// handleCallback handles the OAuth provider callback.
// GET /auth/v1/callback?code=...&state=...
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	// Handle OAuth error from provider
	if errorParam != "" {
		s.redirectWithError(w, r, "", errorParam, errorDesc)
		return
	}

	if code == "" {
		s.jsonError(w, http.StatusBadRequest, "code parameter is required")
		return
	}

	if state == "" {
		s.jsonError(w, http.StatusBadRequest, "state parameter is required")
		return
	}

	// Retrieve and validate flow state
	flowState, err := s.oauthStateStore.Get(state)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}

	// Delete state immediately to prevent replay
	s.oauthStateStore.Delete(state)

	// Get provider
	provider, err := s.oauthRegistry.Get(flowState.Provider)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "provider_error", "provider not available")
		return
	}

	// Exchange code for tokens
	callbackURL := s.getCallbackURL()
	tokens, err := provider.ExchangeCode(r.Context(), code, flowState.CodeVerifier, callbackURL)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "token_error", "failed to exchange code")
		return
	}

	// Get user info from provider
	userInfo, err := provider.GetUserInfo(r.Context(), tokens.AccessToken)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "userinfo_error", "failed to get user info")
		return
	}

	if err := userInfo.Validate(); err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "validation_error", err.Error())
		return
	}

	// Find or create user
	user, err := s.findOrCreateOAuthUser(flowState.Provider, userInfo)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "user_error", "failed to create user")
		return
	}

	// Create session
	session, err := s.authService.CreateSession(user.ID)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "session_error", "failed to create session")
		return
	}

	// Generate tokens
	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "token_error", "failed to generate access token")
		return
	}

	refreshToken, err := s.authService.GenerateRefreshToken(user.ID, session.ID)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "token_error", "failed to generate refresh token")
		return
	}

	// Update identity last sign in
	s.authService.UpdateIdentityLastSignIn(flowState.Provider, userInfo.ProviderID)

	// Redirect to client with tokens in fragment
	s.redirectWithTokens(w, r, flowState.RedirectTo, accessToken, refreshToken)
}

// findOrCreateOAuthUser finds an existing user or creates a new one.
func (s *Server) findOrCreateOAuthUser(provider string, userInfo *oauth.UserInfo) (*auth.User, error) {
	// First, check if identity already exists
	identity, err := s.authService.GetIdentityByProvider(provider, userInfo.ProviderID)
	if err == nil {
		// Identity exists, get the user
		return s.authService.GetUserByID(identity.UserID)
	}

	// Identity doesn't exist, check if user with email exists
	user, err := s.authService.GetUserByEmail(userInfo.Email)
	if err == nil {
		// User exists, link the identity (auto-link by email)
		identity := &auth.Identity{
			UserID:     user.ID,
			Provider:   provider,
			ProviderID: userInfo.ProviderID,
			IdentityData: map[string]interface{}{
				"email":      userInfo.Email,
				"name":       userInfo.Name,
				"avatar_url": userInfo.AvatarURL,
			},
		}
		if err := s.authService.CreateIdentity(identity); err != nil {
			return nil, err
		}

		// Update app_metadata to add provider
		s.authService.AddProviderToUser(user.ID, provider)

		return user, nil
	}

	// Create new user
	user, err = s.authService.CreateOAuthUser(userInfo.Email, provider, map[string]interface{}{
		"name":       userInfo.Name,
		"avatar_url": userInfo.AvatarURL,
	})
	if err != nil {
		return nil, err
	}

	// Create identity
	identity = &auth.Identity{
		UserID:     user.ID,
		Provider:   provider,
		ProviderID: userInfo.ProviderID,
		IdentityData: map[string]interface{}{
			"email":      userInfo.Email,
			"name":       userInfo.Name,
			"avatar_url": userInfo.AvatarURL,
		},
	}
	if err := s.authService.CreateIdentity(identity); err != nil {
		return nil, err
	}

	return user, nil
}

// redirectWithTokens redirects to the client with tokens in the URL fragment.
func (s *Server) redirectWithTokens(w http.ResponseWriter, r *http.Request, redirectTo, accessToken, refreshToken string) {
	fragment := url.Values{
		"access_token":  {accessToken},
		"refresh_token": {refreshToken},
		"token_type":    {"bearer"},
		"expires_in":    {"3600"},
	}

	redirectURL := redirectTo + "#" + fragment.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// redirectWithError redirects to the client with an error in the URL fragment.
func (s *Server) redirectWithError(w http.ResponseWriter, r *http.Request, redirectTo, errorCode, errorDesc string) {
	if redirectTo == "" {
		s.jsonError(w, http.StatusBadRequest, errorDesc)
		return
	}

	fragment := url.Values{
		"error":             {errorCode},
		"error_description": {errorDesc},
	}

	redirectURL := redirectTo + "#" + fragment.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
```

Add route in `setupRoutes()`:

```go
r.Get("/callback", s.handleCallback)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -run TestCallback -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/oauth_handlers.go internal/server/server.go
git commit -m "feat(oauth): add callback endpoint with user creation/linking"
```

---

## Task 11: Add Identity List and Unlink Endpoints

**Files:**
- Modify: `internal/server/oauth_handlers.go`
- Modify: `internal/server/oauth_handlers_test.go`

**Step 1: Write the failing test**

Add to `internal/server/oauth_handlers_test.go`:

```go
func TestGetIdentitiesEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create user and identity
	user, _ := s.authService.CreateUser("test@example.com", "password123", nil, nil, true)
	s.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})

	// Create access token
	session, _ := s.authService.CreateSession(user.ID)
	accessToken, _ := s.authService.GenerateAccessToken(user, session.ID)

	req := httptest.NewRequest("GET", "/auth/v1/user/identities", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Identities []auth.Identity `json:"identities"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Len(t, resp.Identities, 1)
	assert.Equal(t, "google", resp.Identities[0].Provider)
}

func TestUnlinkIdentityEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create user with password (so unlinking OAuth is allowed)
	user, _ := s.authService.CreateUser("test@example.com", "password123", nil, nil, true)
	s.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})

	session, _ := s.authService.CreateSession(user.ID)
	accessToken, _ := s.authService.GenerateAccessToken(user, session.ID)

	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify identity is deleted
	identities, _ := s.authService.GetIdentitiesByUser(user.ID)
	assert.Len(t, identities, 0)
}

func TestUnlinkLastIdentityBlocked(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create OAuth-only user (no password)
	user, _ := s.authService.CreateOAuthUser("test@example.com", "google", nil)
	s.authService.CreateIdentity(&auth.Identity{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: "google-123",
	})

	session, _ := s.authService.CreateSession(user.ID)
	accessToken, _ := s.authService.GenerateAccessToken(user, session.ID)

	req := httptest.NewRequest("DELETE", "/auth/v1/user/identities/google", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	// Should be blocked - can't remove last auth method
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -run TestIdentities -v`
Expected: FAIL - endpoints not defined

**Step 3: Write minimal implementation**

Add to `internal/server/oauth_handlers.go`:

```go
// handleGetIdentities returns the user's linked OAuth identities.
// GET /auth/v1/user/identities
func (s *Server) handleGetIdentities(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	identities, err := s.authService.GetIdentitiesByUser(user.ID)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get identities")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"identities": identities,
	})
}

// handleUnlinkIdentity removes an OAuth identity from the user.
// DELETE /auth/v1/user/identities/{provider}
func (s *Server) handleUnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	provider := chi.URLParam(r, "provider")
	if provider == "" {
		s.jsonError(w, http.StatusBadRequest, "provider is required")
		return
	}

	// Check if user has other auth methods
	hasPassword := user.EncryptedPassword != ""
	identities, err := s.authService.GetIdentitiesByUser(user.ID)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to check identities")
		return
	}

	// Count auth methods
	authMethodCount := len(identities)
	if hasPassword {
		authMethodCount++
	}

	// Check if this is the last auth method
	if authMethodCount <= 1 {
		s.jsonError(w, http.StatusBadRequest, "cannot remove last authentication method")
		return
	}

	// Delete the identity
	if err := s.authService.DeleteIdentity(user.ID, provider); err != nil {
		if err == auth.ErrIdentityNotFound {
			s.jsonError(w, http.StatusNotFound, "identity not found")
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to delete identity")
		return
	}

	// Remove provider from app_metadata
	s.authService.RemoveProviderFromUser(user.ID, provider)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "identity unlinked successfully",
	})
}
```

Add routes in `setupRoutes()` (in protected group):

```go
// In the auth-protected group:
r.Get("/user/identities", s.handleGetIdentities)
r.Delete("/user/identities/{provider}", s.handleUnlinkIdentity)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -run TestIdentities -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/oauth_handlers.go internal/server/server.go
git commit -m "feat(oauth): add identities list and unlink endpoints"
```

---

## Task 12: Add Dashboard OAuth Configuration API

**Files:**
- Modify: `internal/dashboard/handler.go`
- Create: `internal/dashboard/oauth.go`
- Create: `internal/dashboard/oauth_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/oauth_test.go`:

```go
package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOAuthSettings(t *testing.T) {
	h, cleanup := setupTestHandler(t)
	defer cleanup()

	// Set up session
	sessionToken := setupDashboardSession(t, h)

	req := httptest.NewRequest("GET", "/_/api/settings/oauth", nil)
	req.AddCookie(&http.Cookie{Name: "dashboard_session", Value: sessionToken})
	w := httptest.NewRecorder()

	h.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Contains(t, resp, "google")
	assert.Contains(t, resp, "github")
}

func TestUpdateOAuthSettings(t *testing.T) {
	h, cleanup := setupTestHandler(t)
	defer cleanup()

	sessionToken := setupDashboardSession(t, h)

	body := `{
		"google": {
			"client_id": "test-google-id",
			"client_secret": "test-google-secret",
			"enabled": true
		}
	}`

	req := httptest.NewRequest("PATCH", "/_/api/settings/oauth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "dashboard_session", Value: sessionToken})
	w := httptest.NewRecorder()

	h.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify settings were saved
	req2 := httptest.NewRequest("GET", "/_/api/settings/oauth", nil)
	req2.AddCookie(&http.Cookie{Name: "dashboard_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	h.router.ServeHTTP(w2, req2)

	var resp map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&resp)
	google := resp["google"].(map[string]interface{})
	assert.Equal(t, "test-google-id", google["client_id"])
	assert.True(t, google["enabled"].(bool))
	// Secret should be masked
	assert.Equal(t, "********", google["client_secret"])
}

func TestRedirectURLManagement(t *testing.T) {
	h, cleanup := setupTestHandler(t)
	defer cleanup()

	sessionToken := setupDashboardSession(t, h)

	// Add redirect URL
	body := `{"url": "http://localhost:3000/callback"}`
	req := httptest.NewRequest("POST", "/_/api/settings/oauth/redirect-urls", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "dashboard_session", Value: sessionToken})
	w := httptest.NewRecorder()

	h.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// List redirect URLs
	req2 := httptest.NewRequest("GET", "/_/api/settings/oauth/redirect-urls", nil)
	req2.AddCookie(&http.Cookie{Name: "dashboard_session", Value: sessionToken})
	w2 := httptest.NewRecorder()

	h.router.ServeHTTP(w2, req2)

	var resp struct {
		URLs []string `json:"urls"`
	}
	json.NewDecoder(w2.Body).Decode(&resp)
	assert.Contains(t, resp.URLs, "http://localhost:3000/callback")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestOAuth -v`
Expected: FAIL - handlers not defined

**Step 3: Write minimal implementation**

Create `internal/dashboard/oauth.go`:

```go
package dashboard

import (
	"encoding/json"
	"net/http"
)

// OAuthProviderConfig holds configuration for an OAuth provider.
type OAuthProviderConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Enabled      bool   `json:"enabled"`
}

// handleGetOAuthSettings returns OAuth provider configuration.
// GET /_/api/settings/oauth
func (h *Handler) handleGetOAuthSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]OAuthProviderConfig{
		"google": {
			ClientID:     h.store.Get("oauth_google_client_id"),
			ClientSecret: maskSecret(h.store.Get("oauth_google_client_secret")),
			Enabled:      h.store.Get("oauth_google_enabled") == "true",
		},
		"github": {
			ClientID:     h.store.Get("oauth_github_client_id"),
			ClientSecret: maskSecret(h.store.Get("oauth_github_client_secret")),
			Enabled:      h.store.Get("oauth_github_enabled") == "true",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// handleUpdateOAuthSettings updates OAuth provider configuration.
// PATCH /_/api/settings/oauth
func (h *Handler) handleUpdateOAuthSettings(w http.ResponseWriter, r *http.Request) {
	var updates map[string]OAuthProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	for provider, config := range updates {
		prefix := "oauth_" + provider + "_"

		if config.ClientID != "" {
			h.store.Set(prefix+"client_id", config.ClientID)
		}
		// Only update secret if not masked
		if config.ClientSecret != "" && config.ClientSecret != "********" {
			h.store.Set(prefix+"client_secret", config.ClientSecret)
		}
		h.store.Set(prefix+"enabled", boolToString(config.Enabled))
	}

	// Notify server to reload OAuth config
	if h.oauthReloadFunc != nil {
		h.oauthReloadFunc()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// handleGetRedirectURLs returns allowed OAuth redirect URLs.
// GET /_/api/settings/oauth/redirect-urls
func (h *Handler) handleGetRedirectURLs(w http.ResponseWriter, r *http.Request) {
	urlsJSON := h.store.Get("oauth_redirect_urls")
	var urls []string
	if urlsJSON != "" {
		json.Unmarshal([]byte(urlsJSON), &urls)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"urls": urls})
}

// handleAddRedirectURL adds an allowed OAuth redirect URL.
// POST /_/api/settings/oauth/redirect-urls
func (h *Handler) handleAddRedirectURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// Get existing URLs
	urlsJSON := h.store.Get("oauth_redirect_urls")
	var urls []string
	if urlsJSON != "" {
		json.Unmarshal([]byte(urlsJSON), &urls)
	}

	// Add new URL if not already present
	for _, u := range urls {
		if u == req.URL {
			http.Error(w, "URL already exists", http.StatusConflict)
			return
		}
	}
	urls = append(urls, req.URL)

	// Save
	newJSON, _ := json.Marshal(urls)
	h.store.Set("oauth_redirect_urls", string(newJSON))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

// handleDeleteRedirectURL removes an allowed OAuth redirect URL.
// DELETE /_/api/settings/oauth/redirect-urls
func (h *Handler) handleDeleteRedirectURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	urlsJSON := h.store.Get("oauth_redirect_urls")
	var urls []string
	if urlsJSON != "" {
		json.Unmarshal([]byte(urlsJSON), &urls)
	}

	// Remove URL
	var newURLs []string
	for _, u := range urls {
		if u != req.URL {
			newURLs = append(newURLs, u)
		}
	}

	newJSON, _ := json.Marshal(newURLs)
	h.store.Set("oauth_redirect_urls", string(newJSON))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// maskSecret returns a masked version of a secret.
func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	return "********"
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
```

Add routes in `RegisterRoutes()`:

```go
// OAuth settings routes
r.Get("/api/settings/oauth", h.requireAuth(h.handleGetOAuthSettings))
r.Patch("/api/settings/oauth", h.requireAuth(h.handleUpdateOAuthSettings))
r.Get("/api/settings/oauth/redirect-urls", h.requireAuth(h.handleGetRedirectURLs))
r.Post("/api/settings/oauth/redirect-urls", h.requireAuth(h.handleAddRedirectURL))
r.Delete("/api/settings/oauth/redirect-urls", h.requireAuth(h.handleDeleteRedirectURL))
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestOAuth -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/oauth.go internal/dashboard/oauth_test.go internal/dashboard/handler.go
git commit -m "feat(oauth): add dashboard OAuth configuration API"
```

---

## Task 13: Update Settings Endpoint for Dynamic Providers

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/auth_handlers_test.go`

**Step 1: Write the failing test**

Add to `internal/server/auth_handlers_test.go`:

```go
func TestSettingsShowsEnabledOAuthProviders(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Enable Google, disable GitHub
	s.configureOAuthProvider("google", "id", "secret", true)
	s.configureOAuthProvider("github", "id", "secret", false)

	req := httptest.NewRequest("GET", "/auth/v1/settings", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		External map[string]bool `json:"external"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	assert.True(t, resp.External["google"])
	assert.False(t, resp.External["github"])
	assert.True(t, resp.External["email"]) // Email always enabled
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -run TestSettingsShowsEnabled -v`
Expected: FAIL - settings doesn't reflect OAuth config

**Step 3: Write minimal implementation**

Update `handleSettings` in `internal/server/auth_handlers.go`:

```go
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	settings := map[string]interface{}{
		"external": map[string]bool{
			"email":    true, // Always enabled
			"phone":    false,
			"google":   s.oauthRegistry != nil && s.oauthRegistry.IsEnabled("google"),
			"github":   s.oauthRegistry != nil && s.oauthRegistry.IsEnabled("github"),
			"facebook": false,
			"twitter":  false,
			"apple":    false,
			"discord":  false,
			"twitch":   false,
		},
		"disable_signup":     false,
		"mailer_autoconfirm": s.authService.AutoConfirmEmail,
		"phone_autoconfirm":  false,
		"sms_provider":       "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -run TestSettingsShowsEnabled -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/auth_handlers.go internal/server/auth_handlers_test.go
git commit -m "feat(oauth): update settings endpoint to show enabled providers"
```

---

## Task 14: Add E2E Tests

**Files:**
- Create: `e2e/tests/auth/oauth.test.ts`

**Step 1: Create E2E test file**

Create `e2e/tests/auth/oauth.test.ts`:

```typescript
import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'

describe('OAuth', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(
      process.env.SUPABASE_URL || 'http://localhost:8080',
      process.env.SUPABASE_ANON_KEY || 'test-anon-key'
    )
  })

  describe('signInWithOAuth', () => {
    it('returns authorization URL for enabled provider', async () => {
      const { data, error } = await supabase.auth.signInWithOAuth({
        provider: 'google',
        options: {
          redirectTo: 'http://localhost:3000/callback',
          skipBrowserRedirect: true,
        },
      })

      // If Google is not configured, this will error
      // In test mode, we just verify the URL structure
      if (data?.url) {
        expect(data.url).toContain('/auth/v1/authorize')
        expect(data.url).toContain('provider=google')
      }
    })

    it('returns error for disabled provider', async () => {
      const { data, error } = await supabase.auth.signInWithOAuth({
        provider: 'facebook' as any, // Not enabled
        options: {
          skipBrowserRedirect: true,
        },
      })

      // Should indicate provider not available
      expect(error || !data?.url).toBeTruthy()
    })
  })

  describe('settings', () => {
    it('shows OAuth provider status', async () => {
      const response = await fetch('http://localhost:8080/auth/v1/settings')
      const settings = await response.json()

      expect(settings.external).toBeDefined()
      expect(typeof settings.external.google).toBe('boolean')
      expect(typeof settings.external.github).toBe('boolean')
      expect(settings.external.email).toBe(true) // Always enabled
    })
  })

  describe('identities', () => {
    it('returns empty array for email-only user', async () => {
      // Sign up a new user
      const email = `test-${Date.now()}@example.com`
      const { data: signUpData } = await supabase.auth.signUp({
        email,
        password: 'testpassword123',
      })

      if (signUpData.session) {
        // Get identities
        const { data: userData } = await supabase.auth.getUser()

        // Email users don't have OAuth identities
        expect(userData.user?.identities || []).toHaveLength(0)
      }
    })
  })

  describe('callback validation', () => {
    it('rejects invalid state parameter', async () => {
      const response = await fetch(
        'http://localhost:8080/auth/v1/callback?code=test&state=invalid',
        { redirect: 'manual' }
      )

      // Should return error (either 400 or redirect with error)
      expect(response.status === 400 || response.status === 302).toBe(true)
    })

    it('rejects missing code parameter', async () => {
      const response = await fetch(
        'http://localhost:8080/auth/v1/callback?state=test',
        { redirect: 'manual' }
      )

      expect(response.status).toBe(400)
    })
  })
})
```

**Step 2: Run E2E tests**

Run: `cd e2e && npm test -- --grep OAuth`
Expected: Tests pass (some may skip if providers not configured)

**Step 3: Commit**

```bash
git add e2e/tests/auth/oauth.test.ts
git commit -m "test(oauth): add E2E tests for OAuth endpoints"
```

---

## Task 15: Add Dashboard UI for OAuth Configuration

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/index.html`

**Step 1: Add OAuth settings section to dashboard**

Add to `internal/dashboard/static/app.js`:

```javascript
// OAuth Settings Component
function renderOAuthSettings() {
  return `
    <div class="oauth-settings">
      <h3>OAuth Providers</h3>
      <div id="oauth-providers"></div>

      <h3>Allowed Redirect URLs</h3>
      <div id="redirect-urls"></div>
      <div class="add-url-form">
        <input type="url" id="new-redirect-url" placeholder="https://example.com/callback">
        <button onclick="addRedirectURL()">Add URL</button>
      </div>
    </div>
  `
}

async function loadOAuthSettings() {
  const [settingsRes, urlsRes] = await Promise.all([
    fetch('/_/api/settings/oauth'),
    fetch('/_/api/settings/oauth/redirect-urls')
  ])

  const settings = await settingsRes.json()
  const { urls } = await urlsRes.json()

  renderOAuthProviders(settings)
  renderRedirectURLs(urls)
}

function renderOAuthProviders(settings) {
  const container = document.getElementById('oauth-providers')
  container.innerHTML = Object.entries(settings).map(([provider, config]) => `
    <div class="oauth-provider">
      <h4>${provider.charAt(0).toUpperCase() + provider.slice(1)}</h4>
      <label>
        <input type="checkbox"
               ${config.enabled ? 'checked' : ''}
               onchange="toggleProvider('${provider}', this.checked)">
        Enabled
      </label>
      <div class="form-group">
        <label>Client ID</label>
        <input type="text"
               value="${config.client_id || ''}"
               onchange="updateProviderField('${provider}', 'client_id', this.value)">
      </div>
      <div class="form-group">
        <label>Client Secret</label>
        <input type="password"
               value="${config.client_secret || ''}"
               placeholder="Enter to change"
               onchange="updateProviderField('${provider}', 'client_secret', this.value)">
      </div>
    </div>
  `).join('')
}

function renderRedirectURLs(urls) {
  const container = document.getElementById('redirect-urls')
  container.innerHTML = urls.length ? urls.map(url => `
    <div class="redirect-url">
      <span>${url}</span>
      <button onclick="removeRedirectURL('${url}')">Remove</button>
    </div>
  `).join('') : '<p>No redirect URLs configured. All URLs allowed in development mode.</p>'
}

async function toggleProvider(provider, enabled) {
  await fetch('/_/api/settings/oauth', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ [provider]: { enabled } })
  })
}

async function updateProviderField(provider, field, value) {
  await fetch('/_/api/settings/oauth', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ [provider]: { [field]: value } })
  })
}

async function addRedirectURL() {
  const input = document.getElementById('new-redirect-url')
  const url = input.value.trim()
  if (!url) return

  await fetch('/_/api/settings/oauth/redirect-urls', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url })
  })

  input.value = ''
  loadOAuthSettings()
}

async function removeRedirectURL(url) {
  await fetch('/_/api/settings/oauth/redirect-urls', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url })
  })
  loadOAuthSettings()
}
```

**Step 2: Add navigation and styles**

Add OAuth settings to the settings navigation and add corresponding CSS.

**Step 3: Test manually**

Navigate to `http://localhost:8080/_` and verify the OAuth settings section works.

**Step 4: Commit**

```bash
git add internal/dashboard/static/
git commit -m "feat(oauth): add dashboard UI for OAuth configuration"
```

---

## Task 16: Integration and Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `e2e/COMPATIBILITY.md`

**Step 1: Update CLAUDE.md**

Add OAuth to the implemented features list and document the new endpoints.

**Step 2: Update COMPATIBILITY.md**

Add OAuth compatibility notes to the E2E compatibility matrix.

**Step 3: Final integration test**

Run full test suite:
```bash
go test ./...
cd e2e && npm test
```

**Step 4: Commit**

```bash
git add CLAUDE.md e2e/COMPATIBILITY.md
git commit -m "docs: add OAuth to documentation and compatibility matrix"
```

---

## Summary

This plan implements OAuth support in 16 tasks:

1. **Database schema** - auth_identities and auth_flow_state tables
2. **PKCE** - Code verifier and challenge generation
3. **Flow state** - Server-side state storage
4. **Types** - Provider interface and core types
5. **Google provider** - Full OAuth implementation
6. **GitHub provider** - Full OAuth implementation
7. **Identity storage** - CRUD for OAuth identities
8. **Provider registry** - Manage enabled providers
9. **Authorize endpoint** - Start OAuth flow
10. **Callback endpoint** - Handle provider response
11. **Identity endpoints** - List and unlink
12. **Dashboard API** - OAuth configuration
13. **Settings update** - Dynamic provider status
14. **E2E tests** - End-to-end validation
15. **Dashboard UI** - Configuration interface
16. **Documentation** - Update docs

Each task follows TDD with explicit test → implement → verify → commit steps.
