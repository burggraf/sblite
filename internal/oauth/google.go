package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

	// Safe extraction of id_token - may not be present in all responses
	var idToken string
	if idTokenValue := token.Extra("id_token"); idTokenValue != nil {
		idToken, _ = idTokenValue.(string)
	}

	return &Tokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      idToken,
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
