package dashboard

import (
	"encoding/json"
	"net/http"
)

// AuthConfig holds authentication configuration settings.
type AuthConfig struct {
	RequireEmailConfirmation bool `json:"require_email_confirmation"`
}

// handleGetAuthConfig returns authentication configuration settings.
// GET /_/api/settings/auth-config
func (h *Handler) handleGetAuthConfig(w http.ResponseWriter, r *http.Request) {
	config := AuthConfig{
		RequireEmailConfirmation: h.GetRequireEmailConfirmation(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handlePatchAuthConfig updates authentication configuration settings.
// PATCH /_/api/settings/auth-config
func (h *Handler) handlePatchAuthConfig(w http.ResponseWriter, r *http.Request) {
	var updates AuthConfig
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Store the setting
	h.store.Set("auth_require_email_confirmation", boolToString(updates.RequireEmailConfirmation))

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
