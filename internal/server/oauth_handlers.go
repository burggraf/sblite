// internal/server/oauth_handlers.go
package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/auth"
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

	// Check if current user is anonymous and should be linked to OAuth
	linkingUserID := s.extractAnonymousUserID(r)
	if linkingUserID != "" {
		flowState.LinkingUserID = linkingUserID
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

// extractAnonymousUserID extracts the user ID from the Authorization header
// if the user is anonymous. Returns empty string if no auth or not anonymous.
func (s *Server) extractAnonymousUserID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := s.authService.ValidateAccessToken(tokenString)
	if err != nil {
		return ""
	}

	userID, ok := (*claims)["sub"].(string)
	if !ok || userID == "" {
		return ""
	}

	// Get the user to check if anonymous
	user, err := s.authService.GetUserByID(userID)
	if err != nil {
		return ""
	}

	if user.IsAnonymous {
		return user.ID
	}

	return ""
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
		s.oauthError(w, http.StatusBadRequest, "code parameter is required")
		return
	}

	if state == "" {
		s.oauthError(w, http.StatusBadRequest, "state parameter is required")
		return
	}

	// Retrieve and validate flow state
	flowState, err := s.oauthStateStore.Get(state)
	if err != nil {
		s.oauthError(w, http.StatusBadRequest, "invalid or expired state")
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

	// Find or create user (pass linkingUserID for anonymous conversion)
	user, err := s.findOrCreateOAuthUser(flowState.Provider, userInfo, flowState.LinkingUserID)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "user_error", "failed to create user")
		return
	}

	// Create session
	session, refreshToken, err := s.authService.CreateSession(user)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "session_error", "failed to create session")
		return
	}

	// Generate access token
	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.redirectWithError(w, r, flowState.RedirectTo, "token_error", "failed to generate access token")
		return
	}

	// Update identity last sign in
	s.authService.UpdateIdentityLastSignIn(flowState.Provider, userInfo.ProviderID)

	// Redirect to client with tokens in fragment
	s.redirectWithTokens(w, r, flowState.RedirectTo, accessToken, refreshToken)
}

// findOrCreateOAuthUser finds an existing user or creates a new one.
// If linkingUserID is provided and that user is anonymous, the anonymous user
// will be converted to a regular user via OAuth instead of creating a new user.
func (s *Server) findOrCreateOAuthUser(provider string, userInfo *oauth.UserInfo, linkingUserID string) (*auth.User, error) {
	// First, check if identity already exists
	identity, err := s.authService.GetIdentityByProvider(provider, userInfo.ProviderID)
	if err == nil {
		// Identity exists, get the user
		return s.authService.GetUserByID(identity.UserID)
	}

	// Check if we're converting an anonymous user
	if linkingUserID != "" {
		anonUser, err := s.authService.GetUserByID(linkingUserID)
		if err == nil && anonUser.IsAnonymous {
			// Convert anonymous user via OAuth
			if err := s.authService.ConvertAnonymousUserViaOAuth(linkingUserID, userInfo.Email, provider); err != nil {
				return nil, err
			}

			// Create identity for the converted user
			identity := &auth.Identity{
				UserID:     linkingUserID,
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

			// Return the converted user
			return s.authService.GetUserByID(linkingUserID)
		}
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
		s.oauthError(w, http.StatusBadRequest, errorDesc)
		return
	}

	fragment := url.Values{
		"error":             {errorCode},
		"error_description": {errorDesc},
	}

	redirectURL := redirectTo + "#" + fragment.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleGetIdentities returns the user's linked OAuth identities.
// GET /auth/v1/user/identities
func (s *Server) handleGetIdentities(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.oauthError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	identities, err := s.authService.GetIdentitiesByUser(user.ID)
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "failed to get identities")
		return
	}

	// Return empty array if no identities (not nil)
	if identities == nil {
		identities = []*auth.Identity{}
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
		s.oauthError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	provider := chi.URLParam(r, "provider")
	if provider == "" {
		s.oauthError(w, http.StatusBadRequest, "provider is required")
		return
	}

	// Get current identities
	identities, err := s.authService.GetIdentitiesByUser(user.ID)
	if err != nil {
		s.oauthError(w, http.StatusInternalServerError, "failed to check identities")
		return
	}

	// Check if the identity to unlink exists
	identityExists := false
	for _, identity := range identities {
		if identity.Provider == provider {
			identityExists = true
			break
		}
	}

	if !identityExists {
		s.oauthError(w, http.StatusNotFound, "identity not found")
		return
	}

	// Check if user has other auth methods
	hasPassword := user.EncryptedPassword != ""

	// Count auth methods (identities count includes the one being removed)
	authMethodCount := len(identities)
	if hasPassword {
		authMethodCount++
	}

	// Check if this is the last auth method (would leave with 0 methods after removal)
	if authMethodCount <= 1 {
		s.oauthError(w, http.StatusBadRequest, "cannot remove last authentication method")
		return
	}

	// Delete the identity
	if err := s.authService.DeleteIdentity(user.ID, provider); err != nil {
		if err == auth.ErrIdentityNotFound {
			s.oauthError(w, http.StatusNotFound, "identity not found")
			return
		}
		s.oauthError(w, http.StatusInternalServerError, "failed to delete identity")
		return
	}

	// Remove provider from app_metadata
	s.authService.RemoveProviderFromUser(user.ID, provider)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "identity unlinked successfully",
	})
}
