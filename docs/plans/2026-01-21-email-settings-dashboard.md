# Email Settings Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add email mode and SMTP configuration to the dashboard Settings page with hot-reload support.

**Architecture:** New `mail_settings.go` handler file following the storage_settings.go pattern. Dashboard store persists settings. Callback pattern notifies server to recreate mailer on config change. Frontend adds collapsible "Email" section after Auth.

**Tech Stack:** Go (Chi router, dashboard store), JavaScript (vanilla SPA), existing mail package.

---

## Task 1: Backend - Mail Settings Handler

**Files:**
- Create: `internal/dashboard/mail_settings.go`
- Modify: `internal/dashboard/handler.go:55-56` (add callback field)
- Modify: `internal/dashboard/handler.go:125` (add setter method)
- Modify: `internal/dashboard/handler.go:212-216` (register routes)

**Step 1: Create mail_settings.go with types and GET handler**

```go
// internal/dashboard/mail_settings.go
package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/markb/sblite/internal/mail"
)

// MailConfig holds mail configuration for dashboard API.
type MailConfig struct {
	Mode     string `json:"mode"`
	From     string `json:"from"`
	SMTPHost string `json:"smtp_host,omitempty"`
	SMTPPort int    `json:"smtp_port,omitempty"`
	SMTPUser string `json:"smtp_user,omitempty"`
	SMTPPass string `json:"smtp_pass,omitempty"`
}

// MailSettingsResponse is returned by GET /settings/mail.
type MailSettingsResponse struct {
	Mode     string `json:"mode"`
	From     string `json:"from"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"` // Masked
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
	smtpHost, _ := h.store.Get("smtp_host")
	smtpPortStr, _ := h.store.Get("smtp_port")
	smtpPort := 587
	if smtpPortStr != "" {
		if p, err := parseInt(smtpPortStr); err == nil {
			smtpPort = p
		}
	}
	smtpUser, _ := h.store.Get("smtp_user")
	smtpPass, _ := h.store.Get("smtp_pass")

	resp := MailSettingsResponse{
		Mode:     mode,
		From:     from,
		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		SMTPUser: smtpUser,
		SMTPPass: maskSecret(smtpPass),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
```

**Step 2: Add PATCH handler for updating mail settings**

Add to `internal/dashboard/mail_settings.go`:

```go
// MailSettingsUpdate is the request body for PATCH /settings/mail.
type MailSettingsUpdate struct {
	Mode     *string `json:"mode,omitempty"`
	From     *string `json:"from,omitempty"`
	SMTPHost *string `json:"smtp_host,omitempty"`
	SMTPPort *int    `json:"smtp_port,omitempty"`
	SMTPUser *string `json:"smtp_user,omitempty"`
	SMTPPass *string `json:"smtp_pass,omitempty"`
}

// handleUpdateMailSettings updates mail configuration.
// PATCH /_/api/settings/mail
func (h *Handler) handleUpdateMailSettings(w http.ResponseWriter, r *http.Request) {
	var req MailSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate mode if provided
	if req.Mode != nil {
		if *req.Mode != mail.ModeLog && *req.Mode != mail.ModeCatch && *req.Mode != mail.ModeSMTP {
			http.Error(w, "mode must be 'log', 'catch', or 'smtp'", http.StatusBadRequest)
			return
		}
		h.store.Set("mail_mode", *req.Mode)
	}

	if req.From != nil {
		h.store.Set("mail_from", *req.From)
	}
	if req.SMTPHost != nil {
		h.store.Set("smtp_host", *req.SMTPHost)
	}
	if req.SMTPPort != nil {
		h.store.Set("smtp_port", fmt.Sprintf("%d", *req.SMTPPort))
	}
	if req.SMTPUser != nil {
		h.store.Set("smtp_user", *req.SMTPUser)
	}
	// Only update password if not masked
	if req.SMTPPass != nil && *req.SMTPPass != "" && *req.SMTPPass != "********" {
		h.store.Set("smtp_pass", *req.SMTPPass)
	}

	// Trigger hot-reload if callback registered
	if h.onMailReload != nil {
		cfg := h.buildMailConfig()
		if err := h.onMailReload(cfg); err != nil {
			http.Error(w, "failed to reload mail config: "+err.Error(), http.StatusInternalServerError)
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
	smtpHost, _ := h.store.Get("smtp_host")
	smtpPortStr, _ := h.store.Get("smtp_port")
	smtpPort := 587
	if smtpPortStr != "" {
		if p, err := parseInt(smtpPortStr); err == nil {
			smtpPort = p
		}
	}
	smtpUser, _ := h.store.Get("smtp_user")
	smtpPass, _ := h.store.Get("smtp_pass")

	return &MailConfig{
		Mode:     mode,
		From:     from,
		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		SMTPUser: smtpUser,
		SMTPPass: smtpPass,
	}
}

// GetMailMode returns the configured mail mode from dashboard settings.
// Returns empty string if not set (caller should use default).
func (h *Handler) GetMailMode() string {
	mode, _ := h.store.Get("mail_mode")
	return mode
}

// GetMailFrom returns the configured mail from address from dashboard settings.
func (h *Handler) GetMailFrom() string {
	from, _ := h.store.Get("mail_from")
	return from
}

// GetSMTPConfig returns SMTP configuration from dashboard settings.
func (h *Handler) GetSMTPConfig() (host string, port int, user, pass string) {
	host, _ = h.store.Get("smtp_host")
	portStr, _ := h.store.Get("smtp_port")
	port = 587
	if portStr != "" {
		if p, err := parseInt(portStr); err == nil {
			port = p
		}
	}
	user, _ = h.store.Get("smtp_user")
	pass, _ = h.store.Get("smtp_pass")
	return
}
```

**Step 3: Add callback field and setter to handler.go**

Edit `internal/dashboard/handler.go` - add field after line 55:

```go
	onMailReload      func(*MailConfig) error
```

Add setter method after `SetStorageReloadFunc` (around line 122):

```go
// SetMailReloadFunc sets the callback function for mail configuration changes.
func (h *Handler) SetMailReloadFunc(f func(*MailConfig) error) {
	h.onMailReload = f
}
```

**Step 4: Register routes in handler.go**

Edit `internal/dashboard/handler.go` around line 216, after storage routes:

```go
			// Mail settings routes
			r.Get("/mail", h.handleGetMailSettings)
			r.Patch("/mail", h.handleUpdateMailSettings)
```

**Step 5: Add missing import to mail_settings.go**

Ensure `"fmt"` is imported for `fmt.Sprintf`.

**Step 6: Run tests to verify compilation**

```bash
go build ./...
```

Expected: Build succeeds

**Step 7: Commit backend handlers**

```bash
git add internal/dashboard/mail_settings.go internal/dashboard/handler.go
git commit -m "feat(dashboard): add mail settings API endpoints"
```

---

## Task 2: Server - Hot-Reload Support

**Files:**
- Modify: `internal/server/server.go:36-38` (need mutable mailer)
- Modify: `internal/server/server.go:165-180` (register callback)
- Modify: `cmd/serve.go:64-76` (apply persisted settings on startup)

**Step 1: Add ReloadMail method to server**

Add to `internal/server/server.go` after `initMail()` method (around line 230):

```go
// ReloadMail recreates the mailer with new configuration.
// Called by dashboard when mail settings change.
func (s *Server) ReloadMail(cfg *dashboard.MailConfig) error {
	// Update mail config
	s.mailConfig.Mode = cfg.Mode
	s.mailConfig.From = cfg.From
	s.mailConfig.SMTPHost = cfg.SMTPHost
	s.mailConfig.SMTPPort = cfg.SMTPPort
	s.mailConfig.SMTPUser = cfg.SMTPUser
	s.mailConfig.SMTPPass = cfg.SMTPPass

	// Reinitialize mailer
	s.initMail()

	log.Info("mail configuration reloaded",
		"mode", cfg.Mode,
		"from", cfg.From,
	)
	return nil
}
```

**Step 2: Register mail reload callback**

Edit `internal/server/server.go` in `NewWithConfig`, after the storage reload setup (around line 190):

```go
	// Register mail reload callback
	s.dashboardHandler.SetMailReloadFunc(func(cfg *dashboard.MailConfig) error {
		return s.ReloadMail(cfg)
	})
```

**Step 3: Apply persisted mail settings on startup**

Edit `internal/server/server.go` `applyPersistedSettings()` method to include mail settings:

```go
func (s *Server) applyPersistedSettings() {
	// Apply persisted SiteURL if configured
	if siteURL := s.dashboardHandler.GetSiteURL(); siteURL != "" {
		s.mailConfig.SiteURL = siteURL
	}

	// Apply persisted mail settings if configured
	if mode := s.dashboardHandler.GetMailMode(); mode != "" {
		s.mailConfig.Mode = mode
	}
	if from := s.dashboardHandler.GetMailFrom(); from != "" {
		s.mailConfig.From = from
	}
	host, port, user, pass := s.dashboardHandler.GetSMTPConfig()
	if host != "" {
		s.mailConfig.SMTPHost = host
	}
	if port != 587 || s.mailConfig.SMTPPort == 0 {
		s.mailConfig.SMTPPort = port
	}
	if user != "" {
		s.mailConfig.SMTPUser = user
	}
	if pass != "" {
		s.mailConfig.SMTPPass = pass
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/server/... -v
```

Expected: All tests pass

**Step 5: Commit server changes**

```bash
git add internal/server/server.go
git commit -m "feat(server): add mail config hot-reload support"
```

---

## Task 3: Frontend - State and Data Loading

**Files:**
- Modify: `internal/dashboard/static/app.js` (state, loading, toggle)

**Step 1: Add mailSettings to state initialization (around line 62)**

Find the `storageSettings` block and add `mailSettings` after it:

```javascript
            mailSettings: {
                mode: 'log',
                from: 'noreply@localhost',
                smtp: {
                    host: '',
                    port: 587,
                    user: '',
                    pass: ''
                },
                loading: false,
                saving: false,
                dirty: false,
                originalMode: 'log'
            },
```

**Step 2: Add 'email' to expandedSections (line 55)**

Change:
```javascript
expandedSections: { server: true, apiKeys: false, auth: false, oauth: false, storage: false, templates: false, export: false },
```
To:
```javascript
expandedSections: { server: true, apiKeys: false, auth: false, oauth: false, email: false, storage: false, templates: false, export: false },
```

**Step 3: Add loadMailSettings method (after loadStorageSettings around line 2930)**

```javascript
    async loadMailSettings() {
        this.state.settings.mailSettings.loading = true;
        this.render();

        try {
            const resp = await fetch('/_/api/settings/mail');
            if (resp.ok) {
                const data = await resp.json();
                this.state.settings.mailSettings.mode = data.mode || 'log';
                this.state.settings.mailSettings.from = data.from || 'noreply@localhost';
                this.state.settings.mailSettings.smtp = {
                    host: data.smtp_host || '',
                    port: data.smtp_port || 587,
                    user: data.smtp_user || '',
                    pass: data.smtp_pass || ''
                };
                this.state.settings.mailSettings.originalMode = data.mode || 'log';
                this.state.settings.mailSettings.dirty = false;
            }
        } catch (err) {
            console.error('Failed to load mail settings:', err);
        }

        this.state.settings.mailSettings.loading = false;
        this.render();
    },
```

**Step 4: Update toggleSettingsSection to handle 'email'**

Find `toggleSettingsSection` and add email case (around line 2893):

```javascript
    toggleSettingsSection(section) {
        this.state.settings.expandedSections[section] = !this.state.settings.expandedSections[section];
        if (section === 'storage' && this.state.settings.expandedSections.storage) {
            this.loadStorageSettings();
        }
        if (section === 'email' && this.state.settings.expandedSections.email) {
            this.loadMailSettings();
        }
        this.render();
    },
```

**Step 5: Commit frontend state changes**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add mail settings state and loading"
```

---

## Task 4: Frontend - UI Rendering

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add renderMailSettingsSection method (after renderStorageSettingsSection)**

```javascript
    renderMailSettingsSection(expanded) {
        const ms = this.state.settings.mailSettings;
        const modeChanged = ms.mode !== ms.originalMode;

        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('email')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>Email</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        ${ms.loading ? '<div class="loading">Loading mail settings...</div>' : `
                            <div class="mail-mode-selector">
                                <label class="form-label">Email Mode</label>
                                <div class="radio-group">
                                    <label class="radio-label">
                                        <input type="radio" name="mail-mode" value="log"
                                               ${ms.mode === 'log' ? 'checked' : ''}
                                               onchange="App.updateMailField('mode', 'log')">
                                        <span>Log (stdout)</span>
                                    </label>
                                    <label class="radio-label">
                                        <input type="radio" name="mail-mode" value="catch"
                                               ${ms.mode === 'catch' ? 'checked' : ''}
                                               onchange="App.updateMailField('mode', 'catch')">
                                        <span>Catch (database + UI)</span>
                                    </label>
                                    <label class="radio-label">
                                        <input type="radio" name="mail-mode" value="smtp"
                                               ${ms.mode === 'smtp' ? 'checked' : ''}
                                               onchange="App.updateMailField('mode', 'smtp')">
                                        <span>SMTP (real email)</span>
                                    </label>
                                </div>
                                <small class="text-muted">
                                    ${ms.mode === 'log' ? 'Emails are printed to server console. Good for quick debugging.' :
                                      ms.mode === 'catch' ? 'Emails are stored in database. View at /mail. Good for development.' :
                                      'Emails are sent via SMTP server. Use for staging/production.'}
                                </small>
                            </div>

                            ${modeChanged ? `
                                <div class="warning-banner">
                                    <strong>Note:</strong> Changing email mode takes effect immediately after saving.
                                    ${ms.mode === 'catch' ? 'The mail viewer will be available at <code>/mail</code>.' : ''}
                                </div>
                            ` : ''}

                            <div class="settings-subsection">
                                <h4>General Settings</h4>
                                <div class="form-group">
                                    <label class="form-label">From Address</label>
                                    <input type="email" class="form-input"
                                           value="${this.escapeHtml(ms.from)}"
                                           placeholder="noreply@localhost"
                                           onchange="App.updateMailField('from', this.value)">
                                    <small class="text-muted">Default sender address for all emails.</small>
                                </div>
                            </div>

                            ${ms.mode === 'smtp' ? `
                            <div class="settings-subsection">
                                <h4>SMTP Settings</h4>
                                <div class="form-group">
                                    <label class="form-label">SMTP Host</label>
                                    <input type="text" class="form-input"
                                           value="${this.escapeHtml(ms.smtp.host)}"
                                           placeholder="smtp.gmail.com"
                                           onchange="App.updateMailField('smtp.host', this.value)">
                                </div>
                                <div class="form-group">
                                    <label class="form-label">SMTP Port</label>
                                    <input type="number" class="form-input"
                                           value="${ms.smtp.port}"
                                           placeholder="587"
                                           onchange="App.updateMailField('smtp.port', parseInt(this.value) || 587)">
                                    <small class="text-muted">Common ports: 587 (TLS), 465 (SSL), 25 (unencrypted)</small>
                                </div>
                                <div class="form-group">
                                    <label class="form-label">Username</label>
                                    <input type="text" class="form-input"
                                           value="${this.escapeHtml(ms.smtp.user)}"
                                           placeholder="user@example.com"
                                           onchange="App.updateMailField('smtp.user', this.value)">
                                </div>
                                <div class="form-group">
                                    <label class="form-label">Password</label>
                                    <input type="password" class="form-input"
                                           value="${ms.smtp.pass}"
                                           placeholder="${ms.smtp.user ? '••••••••' : 'Enter password'}"
                                           onchange="App.updateMailField('smtp.pass', this.value)">
                                    <small class="text-muted">Enter a new value to change the password.</small>
                                </div>
                            </div>
                            ` : ''}

                            ${ms.dirty ? `
                                <div class="form-actions">
                                    <button class="btn btn-secondary" onclick="App.cancelMailSettings()">Cancel</button>
                                    <button class="btn btn-primary" onclick="App.saveMailSettings()" ${ms.saving ? 'disabled' : ''}>
                                        ${ms.saving ? 'Saving...' : 'Save Changes'}
                                    </button>
                                </div>
                            ` : ''}
                        `}
                    </div>
                ` : ''}
            </div>
        `;
    },
```

**Step 2: Add email section to renderSettingsView**

Find `renderSettingsView` and add the email section call after the auth section (around line 3324). Look for the `oauth` section header and add email before storage:

```javascript
                ${this.renderMailSettingsSection(expandedSections.email)}
                ${this.renderStorageSettingsSection(expandedSections.storage)}
```

**Step 3: Commit UI rendering**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add mail settings UI section"
```

---

## Task 5: Frontend - Update and Save Methods

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add updateMailField method (after updateStorageField)**

```javascript
    updateMailField(field, value) {
        const ms = this.state.settings.mailSettings;
        if (field === 'mode') {
            ms.mode = value;
        } else if (field === 'from') {
            ms.from = value;
        } else if (field.startsWith('smtp.')) {
            const smtpField = field.substring(5);
            ms.smtp[smtpField] = value;
        }
        ms.dirty = true;
        this.render();
    },
```

**Step 2: Add saveMailSettings method**

```javascript
    async saveMailSettings() {
        const ms = this.state.settings.mailSettings;
        ms.saving = true;
        this.render();

        try {
            const body = {
                mode: ms.mode,
                from: ms.from
            };

            // Only include SMTP settings if mode is smtp
            if (ms.mode === 'smtp') {
                body.smtp_host = ms.smtp.host;
                body.smtp_port = ms.smtp.port;
                body.smtp_user = ms.smtp.user;
                if (ms.smtp.pass && ms.smtp.pass !== '********') {
                    body.smtp_pass = ms.smtp.pass;
                }
            }

            const resp = await fetch('/_/api/settings/mail', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            if (resp.ok) {
                ms.originalMode = ms.mode;
                ms.dirty = false;
                this.showToast('Mail settings saved successfully');
            } else {
                const err = await resp.text();
                this.showToast('Failed to save: ' + err, 'error');
            }
        } catch (err) {
            this.showToast('Failed to save mail settings: ' + err.message, 'error');
        }

        ms.saving = false;
        this.render();
    },
```

**Step 3: Add cancelMailSettings method**

```javascript
    cancelMailSettings() {
        this.state.settings.mailSettings.dirty = false;
        this.loadMailSettings();
    },
```

**Step 4: Commit save/update methods**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add mail settings save/update handlers"
```

---

## Task 6: Testing

**Files:**
- Create: `internal/dashboard/mail_settings_test.go`

**Step 1: Create test file**

```go
// internal/dashboard/mail_settings_test.go
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
	db := setupTestDB(t)
	defer db.Close()

	handler := NewHandler(db, "")
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
	assert.Equal(t, 587, resp.SMTPPort)
}

func TestUpdateMailSettings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	handler := NewHandler(db, "")

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
	db := setupTestDB(t)
	defer db.Close()

	handler := NewHandler(db, "")
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
	db := setupTestDB(t)
	defer db.Close()

	handler := NewHandler(db, "")
	handler.SetMailReloadFunc(func(cfg *MailConfig) error { return nil })

	r := chi.NewRouter()
	r.Get("/settings/mail", handler.handleGetMailSettings)
	r.Patch("/settings/mail", handler.handleUpdateMailSettings)

	// Update SMTP settings
	body := `{
		"mode": "smtp",
		"smtp_host": "smtp.example.com",
		"smtp_port": 465,
		"smtp_user": "user@example.com",
		"smtp_pass": "secret123"
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
	assert.Equal(t, "smtp.example.com", resp.SMTPHost)
	assert.Equal(t, 465, resp.SMTPPort)
	assert.Equal(t, "user@example.com", resp.SMTPUser)
	assert.Equal(t, "********", resp.SMTPPass) // Should be masked
}
```

**Step 2: Run tests**

```bash
go test ./internal/dashboard/... -v -run TestMailSettings
```

Expected: All tests pass

**Step 3: Commit tests**

```bash
git add internal/dashboard/mail_settings_test.go
git commit -m "test(dashboard): add mail settings API tests"
```

---

## Task 7: Documentation Update

**Files:**
- Modify: `CLAUDE.md` (add endpoint documentation)
- Modify: `docs/EMAIL.md` (add dashboard configuration section)

**Step 1: Add endpoints to CLAUDE.md**

Find the Dashboard API section and add after storage settings:

```markdown
| `/_/api/settings/mail` | GET | Get mail configuration |
| `/_/api/settings/mail` | PATCH | Update mail configuration (hot-reload) |
```

**Step 2: Add dashboard section to docs/EMAIL.md**

Add after the Configuration section:

```markdown
## Dashboard Configuration

Email settings can also be configured through the web dashboard at `/_` under Settings → Email.

**Available settings:**
- Email Mode (Log, Catch, SMTP)
- From Address
- SMTP Host, Port, Username, Password (when SMTP mode selected)

Changes made through the dashboard take effect immediately without server restart (hot-reload). Dashboard settings take priority over CLI flags and environment variables.
```

**Step 3: Commit documentation**

```bash
git add CLAUDE.md docs/EMAIL.md
git commit -m "docs: add mail settings dashboard documentation"
```

---

## Task 8: Final Integration Test

**Step 1: Build and run server**

```bash
go build -o sblite . && ./sblite serve --db test.db
```

**Step 2: Manual testing checklist**

1. Open `http://localhost:8080/_` and login
2. Navigate to Settings
3. Expand Email section
4. Verify defaults shown (Log mode, noreply@localhost)
5. Change mode to Catch, save
6. Verify `/mail` endpoint becomes accessible
7. Change mode to SMTP, fill in test values
8. Verify SMTP fields appear when SMTP selected
9. Save and verify no errors

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat(dashboard): email settings with hot-reload

- Add mail settings API endpoints (GET/PATCH)
- Add mail reload callback for hot-reload support
- Add frontend Email section in Settings
- Dashboard settings override env vars/CLI flags
- Add tests and documentation"
```

---

Plan complete and saved to `docs/plans/2026-01-21-email-settings-dashboard.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
