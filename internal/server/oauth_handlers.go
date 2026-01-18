// internal/server/oauth_handlers.go
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
		s.oauthError(w, http.StatusBadRequest, "provider parameter is required")
		return
	}

	if redirectTo == "" {
		s.oauthError(w, http.StatusBadRequest, "redirect_to parameter is required")
		return
	}

	// Validate redirect URL is allowed
	if !s.isRedirectURLAllowed(redirectTo) {
		s.oauthError(w, http.StatusBadRequest, "redirect_to URL is not allowed")
		return
	}

	// Get provider
	provider, err := s.oauthRegistry.Get(providerName)
	if err != nil {
		s.oauthError(w, http.StatusBadRequest, "provider not found or not enabled")
		return
	}

	// Generate PKCE values
	codeVerifier, err := oauth.GenerateCodeVerifier()
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "failed to generate code verifier")
		return
	}
	codeChallenge := oauth.GenerateCodeChallenge(codeVerifier)

	// Generate state
	state, err := oauth.GenerateState()
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "failed to generate state")
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
		s.oauthError(w, http.StatusInternalServerError, "failed to save flow state")
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
	if s.baseURL == "" {
		return "/auth/v1/callback"
	}
	return s.baseURL + "/auth/v1/callback"
}

// oauthError writes a JSON error response for OAuth endpoints.
func (s *Server) oauthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             http.StatusText(status),
		"error_description": message,
	})
}
