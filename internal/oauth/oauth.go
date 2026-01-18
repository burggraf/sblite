package oauth

import (
	"context"
	"errors"
)

var (
	ErrProviderNotFound   = errors.New("oauth provider not found")
	ErrProviderNotEnabled = errors.New("oauth provider not enabled")
	ErrInvalidUserInfo    = errors.New("invalid user info from provider")
	ErrEmailRequired      = errors.New("email is required from oauth provider")
	ErrProviderIDRequired = errors.New("provider ID is required")
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
