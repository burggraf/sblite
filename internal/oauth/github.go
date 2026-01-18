package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
// Note: GitHub does not support PKCE, so codeChallenge is ignored.
func (g *GitHubProvider) AuthURL(state, codeChallenge, redirectURI string) string {
	redirect := g.redirectURL
	if redirectURI != "" {
		redirect = redirectURI
	}

	params := url.Values{}
	params.Set("client_id", g.clientID)
	params.Set("redirect_uri", redirect)
	params.Set("state", state)
	params.Set("scope", "read:user user:email")

	return githubAuthorizeURL + "?" + params.Encode()
}

// ExchangeCode exchanges the authorization code for tokens.
// Note: GitHub does not support PKCE, so codeVerifier is ignored.
func (g *GitHubProvider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Tokens, error) {
	redirect := g.redirectURL
	if redirectURI != "" {
		redirect = redirectURI
	}

	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("client_secret", g.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirect)

	req, err := http.NewRequestWithContext(ctx, "POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read github token response: %w", err)
	}

	// GitHub returns 200 even on errors, so check the response body
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode github token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("github returned empty access token")
	}

	return &Tokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
	}, nil
}

// GetUserInfo fetches user information from GitHub.
func (g *GitHubProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user returned %d: %s", resp.StatusCode, body)
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode github user: %w", err)
	}

	// Use login as name if name is empty
	name := githubUser.Name
	if name == "" {
		name = githubUser.Login
	}

	email := githubUser.Email
	emailVerified := false

	// If email is not public, fetch from /user/emails endpoint
	if email == "" {
		var err error
		email, emailVerified, err = g.fetchPrimaryEmail(ctx, accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch github email: %w", err)
		}
	} else {
		// Public emails are considered verified by GitHub
		emailVerified = true
	}

	return &UserInfo{
		ProviderID:    strconv.FormatInt(githubUser.ID, 10),
		Email:         email,
		Name:          name,
		AvatarURL:     githubUser.AvatarURL,
		EmailVerified: emailVerified,
	}, nil
}

// fetchPrimaryEmail fetches the primary verified email from GitHub's /user/emails endpoint.
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("github emails returned %d: %s", resp.StatusCode, body)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("failed to decode github emails: %w", err)
	}

	// Look for primary verified email
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

	// Fall back to any email
	for _, e := range emails {
		if e.Email != "" {
			return e.Email, false, nil
		}
	}

	return "", false, fmt.Errorf("no email found in github account")
}
