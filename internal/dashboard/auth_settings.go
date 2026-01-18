package dashboard

import (
	"encoding/json"
	"net/http"
)

// AuthConfig holds authentication configuration settings.
type AuthConfig struct {
	RequireEmailConfirmation bool `json:"require_email_confirmation"`
	AllowAnonymous           bool `json:"allow_anonymous"`
	AnonymousUserCount       int  `json:"anonymous_user_count,omitempty"`
}

// handleGetAuthConfig returns authentication configuration settings.
// GET /_/api/settings/auth-config
func (h *Handler) handleGetAuthConfig(w http.ResponseWriter, r *http.Request) {
	// Count anonymous users
	var anonymousCount int
	err := h.db.QueryRow("SELECT COUNT(*) FROM auth_users WHERE is_anonymous = 1").Scan(&anonymousCount)
	if err != nil {
		// If query fails, default to 0
		anonymousCount = 0
	}

	config := AuthConfig{
		RequireEmailConfirmation: h.GetRequireEmailConfirmation(),
		AllowAnonymous:           h.GetAllowAnonymous(),
		AnonymousUserCount:       anonymousCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// authConfigUpdate holds partial updates for authentication configuration.
// We use pointers to distinguish between "not provided" and "false".
type authConfigUpdate struct {
	RequireEmailConfirmation *bool `json:"require_email_confirmation"`
	AllowAnonymous           *bool `json:"allow_anonymous"`
}

// handlePatchAuthConfig updates authentication configuration settings.
// PATCH /_/api/settings/auth-config
func (h *Handler) handlePatchAuthConfig(w http.ResponseWriter, r *http.Request) {
	var updates authConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Update only the fields that were provided
	if updates.RequireEmailConfirmation != nil {
		h.store.Set("auth_require_email_confirmation", boolToString(*updates.RequireEmailConfirmation))
	}
	if updates.AllowAnonymous != nil {
		h.store.Set("auth_allow_anonymous", boolToString(*updates.AllowAnonymous))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// GetRequireEmailConfirmation returns whether email confirmation is required for new signups.
// Default is true (require confirmation), matching Supabase behavior.
func (h *Handler) GetRequireEmailConfirmation() bool {
	val, _ := h.store.Get("auth_require_email_confirmation")
	// Default to true if not set
	return val != "false"
}

// GetAllowAnonymous returns whether anonymous sign-in is allowed.
// Default is true (anonymous enabled), making it easy to get started.
func (h *Handler) GetAllowAnonymous() bool {
	val, _ := h.store.Get("auth_allow_anonymous")
	// Default to true if not set
	return val != "false"
}
