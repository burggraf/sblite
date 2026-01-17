# Phase 2: Dashboard Shell Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create the basic dashboard web UI shell with authentication, served at `/_/`

**Architecture:** Cookie-based session auth with bcrypt password hash stored in `_dashboard` table. Vanilla JS SPA with light/dark theme. Routes registered at `/_/` with API at `/_/api/`.

**Tech Stack:** Go (Chi router, bcrypt, embed), Vanilla JS, CSS variables for theming

---

## Task 1: Add _dashboard Table to Migrations

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/migrations_test.go`

**Step 1: Write the failing test**

Add to `internal/db/migrations_test.go`:

```go
func TestDashboardTableCreated(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify _dashboard table exists
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_dashboard'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query for _dashboard table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected _dashboard table to exist")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard-ui
go test ./internal/db/... -run TestDashboardTableCreated -v
```

Expected: FAIL with "expected _dashboard table to exist"

**Step 3: Implement the _dashboard schema**

Add to `internal/db/migrations.go` after `schemaMigrationsSchema`:

```go
const dashboardSchema = `
CREATE TABLE IF NOT EXISTS _dashboard (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
`
```

Update `RunMigrations()` to include:

```go
_, err = db.Exec(dashboardSchema)
if err != nil {
    return fmt.Errorf("failed to run dashboard schema migration: %w", err)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/db/... -run TestDashboardTableCreated -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/migrations.go internal/db/migrations_test.go
git commit -m "feat(db): add _dashboard table for dashboard settings"
```

---

## Task 2: Create Dashboard Package - Auth Types and Store

**Files:**
- Create: `internal/dashboard/store.go`
- Create: `internal/dashboard/store_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/store_test.go`:

```go
package dashboard

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

func TestStoreSetAndGet(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Set a value
	err := store.Set("test_key", "test_value")
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// Get the value
	value, err := store.Get("test_key")
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if value != "test_value" {
		t.Errorf("expected 'test_value', got '%s'", value)
	}
}

func TestStoreGetMissing(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Get non-existent key
	value, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "" {
		t.Errorf("expected empty string for missing key, got '%s'", value)
	}
}

func TestStoreHasPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)

	// Initially no password
	if store.HasPassword() {
		t.Error("expected no password initially")
	}

	// Set password hash
	err := store.Set("password_hash", "$2a$10$somehash")
	if err != nil {
		t.Fatalf("failed to set password_hash: %v", err)
	}

	// Now should have password
	if !store.HasPassword() {
		t.Error("expected password to exist")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/dashboard/... -v
```

Expected: FAIL (package doesn't exist)

**Step 3: Implement the Store**

Create `internal/dashboard/store.go`:

```go
// Package dashboard provides the web dashboard for sblite administration.
package dashboard

import (
	"database/sql"
)

// Store handles dashboard key-value storage in _dashboard table.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get retrieves a value by key. Returns empty string if not found.
func (s *Store) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM _dashboard WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// Set stores a value by key (upsert).
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO _dashboard (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, key, value)
	return err
}

// HasPassword returns true if a password hash is stored.
func (s *Store) HasPassword() bool {
	hash, err := s.Get("password_hash")
	return err == nil && hash != ""
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/dashboard/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/store.go internal/dashboard/store_test.go
git commit -m "feat(dashboard): add Store for key-value settings"
```

---

## Task 3: Dashboard Auth - Password Hashing and Verification

**Files:**
- Create: `internal/dashboard/auth.go`
- Create: `internal/dashboard/auth_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/auth_test.go`:

```go
package dashboard

import (
	"testing"
)

func TestAuthSetupPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup password
	err := auth.SetupPassword("mypassword123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Should now need login, not setup
	if auth.NeedsSetup() {
		t.Error("expected NeedsSetup to return false after setup")
	}
}

func TestAuthVerifyPassword(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Setup password
	err := auth.SetupPassword("mypassword123")
	if err != nil {
		t.Fatalf("failed to setup password: %v", err)
	}

	// Verify correct password
	if !auth.VerifyPassword("mypassword123") {
		t.Error("expected correct password to verify")
	}

	// Verify incorrect password
	if auth.VerifyPassword("wrongpassword") {
		t.Error("expected wrong password to fail verification")
	}
}

func TestAuthSetupOnlyOnce(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// First setup should succeed
	err := auth.SetupPassword("password1")
	if err != nil {
		t.Fatalf("first setup failed: %v", err)
	}

	// Second setup should fail
	err = auth.SetupPassword("password2")
	if err == nil {
		t.Error("expected second setup to fail")
	}
}

func TestAuthPasswordMinLength(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	auth := NewAuth(NewStore(database.DB))

	// Too short password
	err := auth.SetupPassword("short")
	if err == nil {
		t.Error("expected short password to fail")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/dashboard/... -run TestAuth -v
```

Expected: FAIL (Auth type doesn't exist)

**Step 3: Implement Auth**

Create `internal/dashboard/auth.go`:

```go
package dashboard

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLength = 8
	bcryptCost        = 10
)

var (
	ErrPasswordTooShort  = errors.New("password must be at least 8 characters")
	ErrPasswordExists    = errors.New("password already set, use reset instead")
	ErrInvalidPassword   = errors.New("invalid password")
)

// Auth handles dashboard authentication.
type Auth struct {
	store *Store
}

// NewAuth creates a new Auth.
func NewAuth(store *Store) *Auth {
	return &Auth{store: store}
}

// NeedsSetup returns true if no password has been set.
func (a *Auth) NeedsSetup() bool {
	return !a.store.HasPassword()
}

// SetupPassword sets the initial password. Fails if password already exists.
func (a *Auth) SetupPassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	if a.store.HasPassword() {
		return ErrPasswordExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	return a.store.Set("password_hash", string(hash))
}

// ResetPassword changes the password (can be used anytime).
func (a *Auth) ResetPassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	return a.store.Set("password_hash", string(hash))
}

// VerifyPassword checks if the provided password matches the stored hash.
func (a *Auth) VerifyPassword(password string) bool {
	hash, err := a.store.Get("password_hash")
	if err != nil || hash == "" {
		return false
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/dashboard/... -run TestAuth -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/auth.go internal/dashboard/auth_test.go
git commit -m "feat(dashboard): add Auth with password hashing and verification"
```

---

## Task 4: Dashboard Session Management

**Files:**
- Create: `internal/dashboard/session.go`
- Create: `internal/dashboard/session_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/session_test.go`:

```go
package dashboard

import (
	"testing"
	"time"
)

func TestSessionCreateAndValidate(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Create session
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	// Validate session
	if !sessions.Validate(token) {
		t.Error("expected session to be valid")
	}
}

func TestSessionInvalidToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Invalid token should fail
	if sessions.Validate("invalid-token") {
		t.Error("expected invalid token to fail validation")
	}
}

func TestSessionDestroy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)

	// Create session
	token, _ := sessions.Create()

	// Validate should work
	if !sessions.Validate(token) {
		t.Error("expected session to be valid")
	}

	// Destroy session
	sessions.Destroy()

	// Validate should fail now
	if sessions.Validate(token) {
		t.Error("expected session to be invalid after destroy")
	}
}

func TestSessionExpiry(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	store := NewStore(database.DB)
	sessions := NewSessionManager(store)
	sessions.maxAge = 1 * time.Second // Short expiry for testing

	// Create session
	token, _ := sessions.Create()

	// Should be valid immediately
	if !sessions.Validate(token) {
		t.Error("expected session to be valid immediately")
	}

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Should be invalid now
	if sessions.Validate(token) {
		t.Error("expected session to expire")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/dashboard/... -run TestSession -v
```

Expected: FAIL (SessionManager doesn't exist)

**Step 3: Implement SessionManager**

Create `internal/dashboard/session.go`:

```go
package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	defaultSessionMaxAge = 24 * time.Hour
)

// SessionManager handles dashboard sessions.
type SessionManager struct {
	store  *Store
	maxAge time.Duration
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(store *Store) *SessionManager {
	return &SessionManager{
		store:  store,
		maxAge: defaultSessionMaxAge,
	}
}

// Create creates a new session and returns the token.
func (s *SessionManager) Create() (string, error) {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	// Store session with expiry
	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	if err := s.store.Set("session_token", token); err != nil {
		return "", err
	}
	if err := s.store.Set("session_expiry", expiry); err != nil {
		return "", err
	}

	return token, nil
}

// Validate checks if the token is valid and not expired.
func (s *SessionManager) Validate(token string) bool {
	storedToken, err := s.store.Get("session_token")
	if err != nil || storedToken == "" || storedToken != token {
		return false
	}

	expiryStr, err := s.store.Get("session_expiry")
	if err != nil || expiryStr == "" {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return false
	}

	return time.Now().Before(expiry)
}

// Destroy removes the current session.
func (s *SessionManager) Destroy() {
	s.store.Set("session_token", "")
	s.store.Set("session_expiry", "")
}

// Refresh extends the session expiry if valid.
func (s *SessionManager) Refresh(token string) bool {
	if !s.Validate(token) {
		return false
	}

	expiry := time.Now().Add(s.maxAge).Format(time.RFC3339)
	s.store.Set("session_expiry", expiry)
	return true
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/dashboard/... -run TestSession -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/session.go internal/dashboard/session_test.go
git commit -m "feat(dashboard): add SessionManager for cookie-based sessions"
```

---

## Task 5: Dashboard HTTP Handler - Static Files

**Files:**
- Create: `internal/dashboard/handler.go`
- Create: `internal/dashboard/static/index.html`
- Create: `internal/dashboard/static/app.js`
- Create: `internal/dashboard/static/style.css`
- Create: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/handler_test.go`:

```go
package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandlerServesUI(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "sblite") {
		t.Error("expected body to contain 'sblite'")
	}
}

func TestHandlerServesCSS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/style.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("expected text/css content type, got %s", contentType)
	}
}

func TestHandlerServesJS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/app.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "javascript") {
		t.Errorf("expected javascript content type, got %s", contentType)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/dashboard/... -run TestHandler -v
```

Expected: FAIL (Handler doesn't exist)

**Step 3: Create static files**

Create `internal/dashboard/static/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>sblite Dashboard</title>
    <link rel="stylesheet" href="/_/static/style.css">
</head>
<body>
    <div id="app">
        <!-- App content rendered by JS -->
    </div>
    <script src="/_/static/app.js"></script>
</body>
</html>
```

Create `internal/dashboard/static/style.css`:

```css
/* Theme variables */
:root {
    --bg-primary: #1a1a2e;
    --bg-secondary: #16213e;
    --bg-tertiary: #0f3460;
    --text-primary: #eaeaea;
    --text-secondary: #a0a0a0;
    --accent: #e94560;
    --accent-hover: #ff6b6b;
    --border: #2a2a4a;
    --success: #4ade80;
    --warning: #fbbf24;
    --error: #ef4444;
}

[data-theme="light"] {
    --bg-primary: #ffffff;
    --bg-secondary: #f8fafc;
    --bg-tertiary: #e2e8f0;
    --text-primary: #1e293b;
    --text-secondary: #64748b;
    --accent: #e94560;
    --accent-hover: #d63850;
    --border: #e2e8f0;
}

/* Reset */
*, *::before, *::after {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    line-height: 1.5;
    min-height: 100vh;
}

/* Layout */
.layout {
    display: flex;
    min-height: 100vh;
}

.sidebar {
    width: 220px;
    background-color: var(--bg-secondary);
    border-right: 1px solid var(--border);
    padding: 1rem;
    display: flex;
    flex-direction: column;
}

.sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--border);
    margin-bottom: 1rem;
}

.logo {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--accent);
}

.main-content {
    flex: 1;
    padding: 1.5rem;
    overflow-y: auto;
}

/* Navigation */
.nav-section {
    margin-bottom: 1.5rem;
}

.nav-section-title {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    margin-bottom: 0.5rem;
}

.nav-item {
    display: block;
    padding: 0.5rem 0.75rem;
    color: var(--text-primary);
    text-decoration: none;
    border-radius: 0.375rem;
    cursor: pointer;
    transition: background-color 0.15s;
}

.nav-item:hover {
    background-color: var(--bg-tertiary);
}

.nav-item.active {
    background-color: var(--accent);
    color: white;
}

/* Buttons */
.btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.5rem 1rem;
    border-radius: 0.375rem;
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    border: none;
}

.btn-primary {
    background-color: var(--accent);
    color: white;
}

.btn-primary:hover {
    background-color: var(--accent-hover);
}

.btn-secondary {
    background-color: var(--bg-tertiary);
    color: var(--text-primary);
}

.btn-secondary:hover {
    background-color: var(--border);
}

/* Forms */
.form-group {
    margin-bottom: 1rem;
}

.form-label {
    display: block;
    font-size: 0.875rem;
    font-weight: 500;
    margin-bottom: 0.375rem;
    color: var(--text-secondary);
}

.form-input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    background-color: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 0.875rem;
}

.form-input:focus {
    outline: none;
    border-color: var(--accent);
}

/* Cards */
.card {
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    padding: 1.5rem;
}

.card-title {
    font-size: 1.125rem;
    font-weight: 600;
    margin-bottom: 1rem;
}

/* Auth screens */
.auth-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 1rem;
}

.auth-card {
    width: 100%;
    max-width: 400px;
}

.auth-title {
    font-size: 1.5rem;
    font-weight: 600;
    text-align: center;
    margin-bottom: 0.5rem;
}

.auth-subtitle {
    color: var(--text-secondary);
    text-align: center;
    margin-bottom: 1.5rem;
}

/* Theme toggle */
.theme-toggle {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 1.25rem;
    padding: 0.25rem;
    color: var(--text-secondary);
}

.theme-toggle:hover {
    color: var(--text-primary);
}

/* Error/Success messages */
.message {
    padding: 0.75rem 1rem;
    border-radius: 0.375rem;
    margin-bottom: 1rem;
    font-size: 0.875rem;
}

.message-error {
    background-color: rgba(239, 68, 68, 0.1);
    color: var(--error);
    border: 1px solid var(--error);
}

.message-success {
    background-color: rgba(74, 222, 128, 0.1);
    color: var(--success);
    border: 1px solid var(--success);
}

/* Loading state */
.loading {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2rem;
    color: var(--text-secondary);
}

/* Header */
.header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.5rem;
    background-color: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}
```

Create `internal/dashboard/static/app.js`:

```javascript
// sblite Dashboard Application

const App = {
    state: {
        authenticated: false,
        needsSetup: true,
        theme: 'dark',
        currentView: 'tables',
        loading: true,
        error: null
    },

    async init() {
        this.loadTheme();
        await this.checkAuth();
        this.render();
    },

    loadTheme() {
        const saved = localStorage.getItem('sblite_theme');
        if (saved) {
            this.state.theme = saved;
        }
        this.applyTheme();
    },

    applyTheme() {
        document.documentElement.setAttribute('data-theme', this.state.theme);
    },

    toggleTheme() {
        this.state.theme = this.state.theme === 'dark' ? 'light' : 'dark';
        localStorage.setItem('sblite_theme', this.state.theme);
        this.applyTheme();
        this.render();
    },

    async checkAuth() {
        try {
            const res = await fetch('/_/api/auth/status');
            const data = await res.json();
            this.state.needsSetup = data.needs_setup;
            this.state.authenticated = data.authenticated;
        } catch (e) {
            this.state.error = 'Failed to connect to server';
        }
        this.state.loading = false;
    },

    async setup(password) {
        try {
            const res = await fetch('/_/api/auth/setup', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ password })
            });
            if (res.ok) {
                this.state.needsSetup = false;
                this.state.authenticated = true;
                this.state.error = null;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Setup failed';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Connection error';
            this.render();
        }
    },

    async login(password) {
        try {
            const res = await fetch('/_/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ password })
            });
            if (res.ok) {
                this.state.authenticated = true;
                this.state.error = null;
                this.render();
            } else {
                this.state.error = 'Invalid password';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Connection error';
            this.render();
        }
    },

    async logout() {
        try {
            await fetch('/_/api/auth/logout', { method: 'POST' });
            this.state.authenticated = false;
            this.render();
        } catch (e) {
            this.state.error = 'Logout failed';
            this.render();
        }
    },

    navigate(view) {
        this.state.currentView = view;
        this.render();
    },

    render() {
        const app = document.getElementById('app');

        if (this.state.loading) {
            app.innerHTML = '<div class="loading">Loading...</div>';
            return;
        }

        if (this.state.needsSetup) {
            app.innerHTML = this.renderSetup();
            return;
        }

        if (!this.state.authenticated) {
            app.innerHTML = this.renderLogin();
            return;
        }

        app.innerHTML = this.renderDashboard();
    },

    renderSetup() {
        return `
            <div class="auth-container">
                <div class="card auth-card">
                    <h1 class="auth-title">Welcome to sblite</h1>
                    <p class="auth-subtitle">Set up your dashboard password to get started</p>
                    ${this.state.error ? `<div class="message message-error">${this.state.error}</div>` : ''}
                    <form onsubmit="event.preventDefault(); App.setup(this.password.value)">
                        <div class="form-group">
                            <label class="form-label" for="password">Password</label>
                            <input type="password" id="password" name="password" class="form-input"
                                   placeholder="Minimum 8 characters" minlength="8" required>
                        </div>
                        <div class="form-group">
                            <label class="form-label" for="confirm">Confirm Password</label>
                            <input type="password" id="confirm" name="confirm" class="form-input"
                                   placeholder="Confirm password" required>
                        </div>
                        <button type="submit" class="btn btn-primary" style="width: 100%">
                            Set Password
                        </button>
                    </form>
                </div>
            </div>
        `;
    },

    renderLogin() {
        return `
            <div class="auth-container">
                <div class="card auth-card">
                    <h1 class="auth-title">sblite Dashboard</h1>
                    <p class="auth-subtitle">Enter your password to continue</p>
                    ${this.state.error ? `<div class="message message-error">${this.state.error}</div>` : ''}
                    <form onsubmit="event.preventDefault(); App.login(this.password.value)">
                        <div class="form-group">
                            <label class="form-label" for="password">Password</label>
                            <input type="password" id="password" name="password" class="form-input" required>
                        </div>
                        <button type="submit" class="btn btn-primary" style="width: 100%">
                            Sign In
                        </button>
                    </form>
                </div>
            </div>
        `;
    },

    renderDashboard() {
        const themeIcon = this.state.theme === 'dark' ? '☀️' : '🌙';

        return `
            <div class="layout">
                <aside class="sidebar">
                    <div class="sidebar-header">
                        <span class="logo">sblite</span>
                        <button class="theme-toggle" onclick="App.toggleTheme()">${themeIcon}</button>
                    </div>

                    <nav>
                        <div class="nav-section">
                            <div class="nav-section-title">Database</div>
                            <a class="nav-item ${this.state.currentView === 'tables' ? 'active' : ''}"
                               onclick="App.navigate('tables')">Tables</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Auth</div>
                            <a class="nav-item ${this.state.currentView === 'users' ? 'active' : ''}"
                               onclick="App.navigate('users')">Users</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Security</div>
                            <a class="nav-item ${this.state.currentView === 'policies' ? 'active' : ''}"
                               onclick="App.navigate('policies')">Policies</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">System</div>
                            <a class="nav-item ${this.state.currentView === 'settings' ? 'active' : ''}"
                               onclick="App.navigate('settings')">Settings</a>
                            <a class="nav-item ${this.state.currentView === 'logs' ? 'active' : ''}"
                               onclick="App.navigate('logs')">Logs</a>
                        </div>
                    </nav>

                    <div style="margin-top: auto; padding-top: 1rem; border-top: 1px solid var(--border)">
                        <a class="nav-item" onclick="App.logout()">Logout</a>
                    </div>
                </aside>

                <main class="main-content">
                    ${this.renderContent()}
                </main>
            </div>
        `;
    },

    renderContent() {
        switch (this.state.currentView) {
            case 'tables':
                return '<div class="card"><h2 class="card-title">Tables</h2><p>Table management coming in Phase 3</p></div>';
            case 'users':
                return '<div class="card"><h2 class="card-title">Users</h2><p>User management coming soon</p></div>';
            case 'policies':
                return '<div class="card"><h2 class="card-title">RLS Policies</h2><p>Policy editor coming in Phase 5</p></div>';
            case 'settings':
                return '<div class="card"><h2 class="card-title">Settings</h2><p>Settings panel coming in Phase 6</p></div>';
            case 'logs':
                return '<div class="card"><h2 class="card-title">Logs</h2><p>Log viewer coming in Phase 6</p></div>';
            default:
                return '<div class="card">Select a section from the sidebar</div>';
        }
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => App.init());
```

**Step 4: Implement the Handler**

Create `internal/dashboard/handler.go`:

```go
package dashboard

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed static/*
var staticFS embed.FS

const sessionCookieName = "_sblite_session"

// Handler serves the dashboard UI and API.
type Handler struct {
	db       *sql.DB
	store    *Store
	auth     *Auth
	sessions *SessionManager
}

// NewHandler creates a new Handler.
func NewHandler(db *sql.DB) *Handler {
	store := NewStore(db)
	return &Handler{
		db:       db,
		store:    store,
		auth:     NewAuth(store),
		sessions: NewSessionManager(store),
	}
}

// RegisterRoutes registers the dashboard routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/auth/status", h.handleAuthStatus)
		r.Post("/auth/setup", h.handleSetup)
		r.Post("/auth/login", h.handleLogin)
		r.Post("/auth/logout", h.handleLogout)
	})

	// Static files
	r.Get("/static/*", h.handleStatic)

	// SPA - serve index.html for all other routes
	r.Get("/*", h.handleIndex)
	r.Get("/", h.handleIndex)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func (h *Handler) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Get the file path from URL
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	content, err := staticFS.ReadFile("static/" + path)
	if err != nil {
		if _, ok := err.(*fs.PathError); ok {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set content type based on extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false

	// Check session cookie
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		authenticated = h.sessions.Validate(cookie.Value)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"needs_setup":   h.auth.NeedsSetup(),
		"authenticated": authenticated,
	})
}

func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if err := h.auth.SetupPassword(req.Password); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Create session
	token, err := h.sessions.Create()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if !h.auth.VerifyPassword(req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
		return
	}

	// Create session
	token, err := h.sessions.Create()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Destroy()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // Delete cookie
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/dashboard/... -run TestHandler -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go internal/dashboard/static/
git commit -m "feat(dashboard): add HTTP handler with embedded static files"
```

---

## Task 6: Register Dashboard Routes in Server

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Add dashboard import and field**

In `internal/server/server.go`, add import:

```go
"github.com/markb/sblite/internal/dashboard"
```

Add field to Server struct:

```go
dashboardHandler *dashboard.Handler
```

**Step 2: Initialize dashboard handler in New()**

After `s.initMail()`:

```go
// Initialize dashboard handler
s.dashboardHandler = dashboard.NewHandler(database.DB)
```

**Step 3: Register routes in setupRoutes()**

Add after mail viewer routes:

```go
// Dashboard routes
s.router.Route("/_", func(r chi.Router) {
    s.dashboardHandler.RegisterRoutes(r)
})
```

**Step 4: Verify build succeeds**

```bash
go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/server/server.go
git commit -m "feat(server): register dashboard routes at /_/"
```

---

## Task 7: CLI Command - dashboard setup

**Files:**
- Create: `cmd/dashboard.go`

**Step 1: Create the dashboard CLI commands**

Create `cmd/dashboard.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Manage the dashboard",
	Long:  `Commands for managing the sblite dashboard.`,
}

var dashboardSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up the dashboard password",
	Long:  `Set the initial dashboard password. Only works if no password has been set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := dashboard.NewStore(database.DB)
		auth := dashboard.NewAuth(store)

		if !auth.NeedsSetup() {
			return fmt.Errorf("dashboard password already set, use 'dashboard reset-password' to change it")
		}

		password, err := promptPassword("Enter dashboard password: ")
		if err != nil {
			return err
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if err := auth.SetupPassword(password); err != nil {
			return fmt.Errorf("failed to set password: %w", err)
		}

		fmt.Println("Dashboard password set successfully")
		return nil
	},
}

var dashboardResetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset the dashboard password",
	Long:  `Change the dashboard password. This will invalidate any existing sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := dashboard.NewStore(database.DB)
		auth := dashboard.NewAuth(store)

		password, err := promptPassword("Enter new dashboard password: ")
		if err != nil {
			return err
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if err := auth.ResetPassword(password); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}

		// Destroy any existing sessions
		sessions := dashboard.NewSessionManager(store)
		sessions.Destroy()

		fmt.Println("Dashboard password reset successfully")
		return nil
	},
}

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Try to read password securely (hides input)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // Add newline after hidden input
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	// Fallback for non-terminal (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	dashboardCmd.AddCommand(dashboardSetupCmd)
	dashboardCmd.AddCommand(dashboardResetPasswordCmd)

	dashboardSetupCmd.Flags().String("db", "./data.db", "Path to the database file")
	dashboardResetPasswordCmd.Flags().String("db", "./data.db", "Path to the database file")
}
```

**Step 2: Verify build succeeds**

```bash
go build ./...
```

**Step 3: Test the command**

```bash
./sblite dashboard --help
./sblite dashboard setup --help
```

**Step 4: Commit**

```bash
git add cmd/dashboard.go
git commit -m "feat(cli): add 'sblite dashboard setup' and 'reset-password' commands"
```

---

## Task 8: Integration Test - Full Dashboard Flow

**Step 1: Build and test the complete flow**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard-ui

# Build
go build -o sblite .

# Initialize fresh database
rm -f test_dashboard.db
./sblite init --db test_dashboard.db

# Set up dashboard password via CLI
echo -e "testpass123\ntestpass123" | ./sblite dashboard setup --db test_dashboard.db

# Start server in background
./sblite serve --db test_dashboard.db &
SERVER_PID=$!
sleep 2

# Test auth status endpoint
curl -s http://localhost:8080/_/api/auth/status | jq

# Test login
curl -s -X POST http://localhost:8080/_/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"password":"testpass123"}' \
  -c cookies.txt | jq

# Test that we get the dashboard HTML
curl -s http://localhost:8080/_/ | head -20

# Cleanup
kill $SERVER_PID
rm -f test_dashboard.db test_dashboard.db-* cookies.txt sblite
```

**Step 2: Verify all tests pass**

```bash
go test ./...
```

Expected: All dashboard tests pass (3 pre-existing failures in rest package unchanged)

**Step 3: Commit if any fixes were needed**

---

## Task 9: Run All Tests and Final Verification

**Step 1: Run the full test suite**

```bash
cd /Users/markb/dev/sblite/.worktrees/dashboard-ui
go test ./...
```

Expected: All tests pass (except the 3 pre-existing failures in rest package)

**Step 2: Verify dashboard is accessible**

Start server and open http://localhost:8080/_/ in browser. Should see:
- Setup screen (first time)
- Login screen (after setup)
- Dashboard with sidebar navigation (after login)
- Theme toggle working
- Logout working

**Step 3: Final commit if needed**

```bash
git status
# Commit any remaining changes
```

---

## Summary

Phase 2 creates the dashboard shell with:

1. **`_dashboard` table** - Key-value storage for settings and password hash
2. **Store** - Database access layer for dashboard settings
3. **Auth** - Password hashing with bcrypt, setup/reset functionality
4. **SessionManager** - Cookie-based sessions with expiry
5. **Handler** - HTTP handlers for UI and API endpoints
6. **Static files** - Embedded HTML, CSS, JS for vanilla SPA
7. **Server integration** - Routes registered at `/_/`
8. **CLI commands** - `sblite dashboard setup` and `reset-password`

The dashboard will be accessible at `http://localhost:8080/_/` after running `sblite serve`.
