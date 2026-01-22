package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMailSettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, "")
	r := chi.NewRouter()
	r.Get("/settings/mail", handler.handleGetMailSettings)

	req := httptest.NewRequest("GET", "/settings/mail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MailSettingsResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Check defaults
	assert.Equal(t, mail.ModeLog, resp.Mode)
	assert.Equal(t, "noreply@localhost", resp.From)
	assert.Equal(t, 587, resp.SMTP.Port)
}

func TestUpdateMailSettings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, "")

	// Track if reload was called
	reloadCalled := false
	var reloadedConfig *MailConfig
	handler.SetMailReloadFunc(func(cfg *MailConfig) error {
		reloadCalled = true
		reloadedConfig = cfg
		return nil
	})

	r := chi.NewRouter()
	r.Patch("/settings/mail", handler.handleUpdateMailSettings)

	body := `{"mode": "catch", "from": "test@example.com"}`
	req := httptest.NewRequest("PATCH", "/settings/mail", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, reloadCalled)
	assert.Equal(t, mail.ModeCatch, reloadedConfig.Mode)
	assert.Equal(t, "test@example.com", reloadedConfig.From)
}

func TestUpdateMailSettings_InvalidMode(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, "")
	r := chi.NewRouter()
	r.Patch("/settings/mail", handler.handleUpdateMailSettings)

	body := `{"mode": "invalid"}`
	req := httptest.NewRequest("PATCH", "/settings/mail", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateMailSettings_SMTPConfig(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB, "")
	handler.SetMailReloadFunc(func(cfg *MailConfig) error { return nil })

	r := chi.NewRouter()
	r.Get("/settings/mail", handler.handleGetMailSettings)
	r.Patch("/settings/mail", handler.handleUpdateMailSettings)

	// Update SMTP settings
	body := `{
		"mode": "smtp",
		"smtp": {
			"host": "smtp.example.com",
			"port": 465,
			"user": "user@example.com",
			"password": "secret123"
		}
	}`
	req := httptest.NewRequest("PATCH", "/settings/mail", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify settings were saved
	req = httptest.NewRequest("GET", "/settings/mail", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp MailSettingsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	assert.Equal(t, mail.ModeSMTP, resp.Mode)
	assert.Equal(t, "smtp.example.com", resp.SMTP.Host)
	assert.Equal(t, 465, resp.SMTP.Port)
	assert.Equal(t, "user@example.com", resp.SMTP.User)
	assert.Equal(t, "********", resp.SMTP.Password) // Should be masked
}
