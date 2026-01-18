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
	googleClientID, _ := h.store.Get("oauth_google_client_id")
	googleClientSecret, _ := h.store.Get("oauth_google_client_secret")
	googleEnabled, _ := h.store.Get("oauth_google_enabled")

	githubClientID, _ := h.store.Get("oauth_github_client_id")
	githubClientSecret, _ := h.store.Get("oauth_github_client_secret")
	githubEnabled, _ := h.store.Get("oauth_github_enabled")

	settings := map[string]OAuthProviderConfig{
		"google": {
			ClientID:     googleClientID,
			ClientSecret: maskSecret(googleClientSecret),
			Enabled:      googleEnabled == "true",
		},
		"github": {
			ClientID:     githubClientID,
			ClientSecret: maskSecret(githubClientSecret),
			Enabled:      githubEnabled == "true",
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

	// Notify server to reload OAuth config (if callback registered)
	if h.oauthReloadFunc != nil {
		h.oauthReloadFunc()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// handleGetRedirectURLs returns allowed OAuth redirect URLs.
// GET /_/api/settings/oauth/redirect-urls
func (h *Handler) handleGetRedirectURLs(w http.ResponseWriter, r *http.Request) {
	urlsJSON, _ := h.store.Get("oauth_redirect_urls")
	var urls []string
	if urlsJSON != "" {
		json.Unmarshal([]byte(urlsJSON), &urls)
	}
	if urls == nil {
		urls = []string{}
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
	urlsJSON, _ := h.store.Get("oauth_redirect_urls")
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
	w.Header().Set("Content-Type", "application/json")
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

	urlsJSON, _ := h.store.Get("oauth_redirect_urls")
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
