package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/markb/sblite/internal/mail"
)

// MailSMTPConfig holds SMTP configuration for the dashboard API.
type MailSMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// MailConfig holds mail configuration for the reload callback.
type MailConfig struct {
	Mode     string
	From     string
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
}

// MailSettingsResponse is returned by GET /settings/mail.
type MailSettingsResponse struct {
	Mode string         `json:"mode"`
	From string         `json:"from"`
	SMTP MailSMTPConfig `json:"smtp"`
}

// MailSettingsUpdate is the request body for PATCH /settings/mail.
type MailSettingsUpdate struct {
	Mode string          `json:"mode,omitempty"`
	From string          `json:"from,omitempty"`
	SMTP *MailSMTPConfig `json:"smtp,omitempty"`
}

// handleGetMailSettings returns mail configuration.
// GET /_/api/settings/mail
func (h *Handler) handleGetMailSettings(w http.ResponseWriter, r *http.Request) {
	mode, _ := h.store.Get("mail_mode")
	if mode == "" {
		mode = mail.ModeLog
	}
	from, _ := h.store.Get("mail_from")
	if from == "" {
		from = "noreply@localhost"
	}

	smtpHost, _ := h.store.Get("mail_smtp_host")
	smtpPortStr, _ := h.store.Get("mail_smtp_port")
	smtpUser, _ := h.store.Get("mail_smtp_user")
	smtpPass, _ := h.store.Get("mail_smtp_password")

	smtpPort := 587
	if smtpPortStr != "" {
		if p, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = p
		}
	}

	resp := MailSettingsResponse{
		Mode: mode,
		From: from,
		SMTP: MailSMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			User:     smtpUser,
			Password: maskSecret(smtpPass),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleUpdateMailSettings updates mail configuration.
// PATCH /_/api/settings/mail
func (h *Handler) handleUpdateMailSettings(w http.ResponseWriter, r *http.Request) {
	var req MailSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Update mode
	if req.Mode != "" {
		if req.Mode != mail.ModeLog && req.Mode != mail.ModeCatch && req.Mode != mail.ModeSMTP {
			http.Error(w, "mode must be 'log', 'catch', or 'smtp'", http.StatusBadRequest)
			return
		}
		h.store.Set("mail_mode", req.Mode)
	}

	// Update from address
	if req.From != "" {
		h.store.Set("mail_from", req.From)
	}

	// Update SMTP settings
	if req.SMTP != nil {
		if req.SMTP.Host != "" {
			h.store.Set("mail_smtp_host", req.SMTP.Host)
		}
		if req.SMTP.Port > 0 {
			h.store.Set("mail_smtp_port", strconv.Itoa(req.SMTP.Port))
		}
		if req.SMTP.User != "" {
			h.store.Set("mail_smtp_user", req.SMTP.User)
		}
		// Only update password if not masked
		if req.SMTP.Password != "" && req.SMTP.Password != "********" {
			h.store.Set("mail_smtp_password", req.SMTP.Password)
		}
	}

	// Trigger hot-reload if callback registered
	if h.onMailReload != nil {
		cfg := h.buildMailConfig()
		if err := h.onMailReload(cfg); err != nil {
			http.Error(w, "failed to reload mail: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// buildMailConfig creates a MailConfig from dashboard settings.
func (h *Handler) buildMailConfig() *MailConfig {
	mode, _ := h.store.Get("mail_mode")
	if mode == "" {
		mode = mail.ModeLog
	}
	from, _ := h.store.Get("mail_from")
	if from == "" {
		from = "noreply@localhost"
	}

	smtpHost, _ := h.store.Get("mail_smtp_host")
	smtpPortStr, _ := h.store.Get("mail_smtp_port")
	smtpUser, _ := h.store.Get("mail_smtp_user")
	smtpPass, _ := h.store.Get("mail_smtp_password")

	smtpPort := 587
	if smtpPortStr != "" {
		if p, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = p
		}
	}

	return &MailConfig{
		Mode:     mode,
		From:     from,
		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		SMTPUser: smtpUser,
		SMTPPass: smtpPass,
	}
}

// GetMailMode returns the configured mail mode from the store.
func (h *Handler) GetMailMode() string {
	mode, _ := h.store.Get("mail_mode")
	if mode == "" {
		return mail.ModeLog
	}
	return mode
}

// GetMailFrom returns the configured from address from the store.
func (h *Handler) GetMailFrom() string {
	from, _ := h.store.Get("mail_from")
	if from == "" {
		return "noreply@localhost"
	}
	return from
}

// GetSMTPConfig returns the SMTP configuration from the store.
func (h *Handler) GetSMTPConfig() (host string, port int, user string, pass string) {
	host, _ = h.store.Get("mail_smtp_host")
	portStr, _ := h.store.Get("mail_smtp_port")
	user, _ = h.store.Get("mail_smtp_user")
	pass, _ = h.store.Get("mail_smtp_password")

	port = 587
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	return host, port, user, pass
}
