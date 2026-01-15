# Phase 1: Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a working Supabase Lite MVP with auth and basic CRUD via supabase-js compatibility.

**Architecture:** Single Go binary with embedded SQLite (modernc.org/sqlite). HTTP server using chi router, JWT auth via golang-jwt, bcrypt password hashing. Three main packages: `server`, `auth`, `rest`.

**Tech Stack:** Go 1.21+, modernc.org/sqlite, github.com/go-chi/chi/v5, github.com/golang-jwt/jwt/v5, golang.org/x/crypto/bcrypt

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `cmd/serve.go`
- Create: `cmd/init.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/markb/sblite`

Expected: `go.mod` created

**Step 2: Create directory structure**

Run: `mkdir -p cmd internal/server internal/auth internal/rest internal/db`

**Step 3: Create main.go**

```go
package main

import "github.com/markb/sblite/cmd"

func main() {
	cmd.Execute()
}
```

**Step 4: Create cmd/root.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sblite",
	Short: "Supabase Lite - lightweight Supabase-compatible backend",
	Long:  `A single-binary backend with SQLite, providing Supabase-compatible auth and REST APIs.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 5: Add cobra dependency**

Run: `go get github.com/spf13/cobra`

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: project scaffolding with cobra CLI

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: SQLite Database Layer

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

**Step 1: Write failing test for DB connection**

```go
// internal/db/db_test.go
package db

import (
	"os"
	"testing"
)

func TestNewDB(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	// Verify WAL mode is enabled
	var journalMode string
	err = database.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode=wal, got %s", journalMode)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v`
Expected: FAIL - package doesn't exist

**Step 3: Add SQLite dependency**

Run: `go get modernc.org/sqlite`

**Step 4: Implement db.go**

```go
// internal/db/db.go
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &DB{db}, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: SQLite database layer with WAL mode

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Auth Schema Migration

**Files:**
- Create: `internal/db/migrations.go`
- Create: `internal/db/migrations_test.go`

**Step 1: Write failing test for migrations**

```go
// internal/db/migrations_test.go
package db

import (
	"testing"
)

func TestRunMigrations(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify auth_users table exists
	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='auth_users'").Scan(&tableName)
	if err != nil {
		t.Fatalf("auth_users table not found: %v", err)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	// Run twice - should not error
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/... -v`
Expected: FAIL - RunMigrations undefined

**Step 3: Implement migrations.go**

```go
// internal/db/migrations.go
package db

import "fmt"

const authSchema = `
CREATE TABLE IF NOT EXISTS auth_users (
    id                    TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    email                 TEXT UNIQUE,
    encrypted_password    TEXT,
    email_confirmed_at    TEXT,
    invited_at            TEXT,
    confirmation_token    TEXT,
    confirmation_sent_at  TEXT,
    recovery_token        TEXT,
    recovery_sent_at      TEXT,
    email_change_token    TEXT,
    email_change          TEXT,
    last_sign_in_at       TEXT,
    raw_app_meta_data     TEXT DEFAULT '{}' CHECK (json_valid(raw_app_meta_data)),
    raw_user_meta_data    TEXT DEFAULT '{}' CHECK (json_valid(raw_user_meta_data)),
    is_super_admin        INTEGER DEFAULT 0,
    role                  TEXT DEFAULT 'authenticated',
    created_at            TEXT DEFAULT (datetime('now')),
    updated_at            TEXT DEFAULT (datetime('now')),
    banned_until          TEXT,
    deleted_at            TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email);
CREATE INDEX IF NOT EXISTS idx_auth_users_confirmation_token ON auth_users(confirmation_token);
CREATE INDEX IF NOT EXISTS idx_auth_users_recovery_token ON auth_users(recovery_token);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT,
    factor_id     TEXT,
    aal           TEXT DEFAULT 'aal1',
    not_after     TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);

CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    token       TEXT UNIQUE NOT NULL,
    user_id     TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    session_id  TEXT REFERENCES auth_sessions(id) ON DELETE CASCADE,
    revoked     INTEGER DEFAULT 0,
    created_at  TEXT DEFAULT (datetime('now')),
    updated_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_token ON auth_refresh_tokens(token);
CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_session_id ON auth_refresh_tokens(session_id);
`

func (db *DB) RunMigrations() error {
	_, err := db.Exec(authSchema)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: auth schema migrations

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Auth Service - User Model

**Files:**
- Create: `internal/auth/user.go`
- Create: `internal/auth/user_test.go`

**Step 1: Write failing test for user creation**

```go
// internal/auth/user_test.go
package auth

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
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

func TestCreateUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, err := service.CreateUser("test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	_, err := service.CreateUser("test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to create first user: %v", err)
	}

	_, err = service.CreateUser("test@example.com", "password456")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -v`
Expected: FAIL - package doesn't exist

**Step 3: Add bcrypt dependency**

Run: `go get golang.org/x/crypto/bcrypt`

**Step 4: Implement user.go**

```go
// internal/auth/user.go
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/markb/sblite/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                string            `json:"id"`
	Email             string            `json:"email"`
	EncryptedPassword string            `json:"-"`
	EmailConfirmedAt  *time.Time        `json:"email_confirmed_at,omitempty"`
	LastSignInAt      *time.Time        `json:"last_sign_in_at,omitempty"`
	AppMetadata       map[string]any    `json:"app_metadata"`
	UserMetadata      map[string]any    `json:"user_metadata"`
	Role              string            `json:"role"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type Service struct {
	db        *db.DB
	jwtSecret string
}

func NewService(database *db.DB, jwtSecret string) *Service {
	return &Service{db: database, jwtSecret: jwtSecret}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Service) CreateUser(email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, ?, '{"provider":"email","providers":["email"]}', '{}', ?, ?)
	`, id, email, string(hash), now, now)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.GetUserByID(id)
}

func (s *Service) GetUserByID(id string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var emailConfirmedAt, lastSignInAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, email, encrypted_password, email_confirmed_at, last_sign_in_at,
		       role, created_at, updated_at
		FROM auth_users WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &emailConfirmedAt,
		&lastSignInAt, &user.Role, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if emailConfirmedAt.Valid {
		t, _ := time.Parse(time.RFC3339, emailConfirmedAt.String)
		user.EmailConfirmedAt = &t
	}
	if lastSignInAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSignInAt.String)
		user.LastSignInAt = &t
	}
	user.AppMetadata = map[string]any{"provider": "email", "providers": []string{"email"}}
	user.UserMetadata = map[string]any{}

	return &user, nil
}

func (s *Service) GetUserByEmail(email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var id string
	err := s.db.QueryRow("SELECT id FROM auth_users WHERE email = ? AND deleted_at IS NULL", email).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return s.GetUserByID(id)
}

func (s *Service) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.EncryptedPassword), []byte(password))
	return err == nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: auth user model with bcrypt password hashing

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: JWT Token Generation

**Files:**
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/jwt_test.go`

**Step 1: Write failing test for JWT generation**

```go
// internal/auth/jwt_test.go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAccessToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user := &User{
		ID:    "test-user-id",
		Email: "test@example.com",
		Role:  "authenticated",
	}

	tokenString, err := service.GenerateAccessToken(user, "session-123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Parse and verify token
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-min-32-characters"), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to get claims")
	}

	if claims["sub"] != "test-user-id" {
		t.Errorf("expected sub=test-user-id, got %v", claims["sub"])
	}
	if claims["email"] != "test@example.com" {
		t.Errorf("expected email=test@example.com, got %v", claims["email"])
	}
	if claims["role"] != "authenticated" {
		t.Errorf("expected role=authenticated, got %v", claims["role"])
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	user, _ := service.CreateUser("test@example.com", "password123")

	session, refreshToken, err := service.CreateSession(user)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Error("expected session ID to be set")
	}
	if refreshToken == "" {
		t.Error("expected refresh token to be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -v`
Expected: FAIL - GenerateAccessToken undefined

**Step 3: Add JWT dependency**

Run: `go get github.com/golang-jwt/jwt/v5`

**Step 4: Implement jwt.go**

```go
// internal/auth/jwt.go
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	AAL       string    `json:"aal"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

const (
	AccessTokenExpiry  = 3600      // 1 hour
	RefreshTokenExpiry = 604800    // 1 week
)

func (s *Service) GenerateAccessToken(user *User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"aud":        "authenticated",
		"exp":        now.Add(time.Duration(AccessTokenExpiry) * time.Second).Unix(),
		"iat":        now.Unix(),
		"iss":        "http://localhost:8080/auth/v1",
		"sub":        user.ID,
		"email":      user.Email,
		"phone":      "",
		"role":       user.Role,
		"aal":        "aal1",
		"session_id": sessionID,
		"app_metadata": map[string]any{
			"provider":  "email",
			"providers": []string{"email"},
		},
		"user_metadata": user.UserMetadata,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Service) ValidateAccessToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &claims, nil
}

func generateRefreshToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "v1." + base64.RawURLEncoding.EncodeToString(b)
}

func (s *Service) CreateSession(user *User) (*Session, string, error) {
	sessionID := generateID()
	refreshToken := generateRefreshToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, created_at, aal)
		VALUES (?, ?, ?, 'aal1')
	`, sessionID, user.ID, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO auth_refresh_tokens (token, user_id, session_id, created_at)
		VALUES (?, ?, ?, ?)
	`, refreshToken, user.ID, sessionID, now)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: time.Now().UTC(),
		AAL:       "aal1",
	}

	return session, refreshToken, nil
}

func (s *Service) RefreshSession(refreshToken string) (*User, *Session, string, error) {
	var userID, sessionID string
	var revoked int

	err := s.db.QueryRow(`
		SELECT user_id, session_id, revoked FROM auth_refresh_tokens WHERE token = ?
	`, refreshToken).Scan(&userID, &sessionID, &revoked)

	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid refresh token")
	}

	if revoked == 1 {
		return nil, nil, "", fmt.Errorf("refresh token has been revoked")
	}

	// Revoke old token
	s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE token = ?", refreshToken)

	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, nil, "", err
	}

	// Create new session
	return s.createSessionWithExistingID(user, sessionID)
}

func (s *Service) createSessionWithExistingID(user *User, sessionID string) (*User, *Session, string, error) {
	refreshToken := generateRefreshToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO auth_refresh_tokens (token, user_id, session_id, created_at)
		VALUES (?, ?, ?, ?)
	`, refreshToken, user.ID, sessionID, now)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: time.Now().UTC(),
		AAL:       "aal1",
	}

	return user, session, refreshToken, nil
}

func (s *Service) RevokeSession(sessionID string) error {
	_, err := s.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1 WHERE session_id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	_, err = s.db.Exec("DELETE FROM auth_sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: JWT token generation and session management

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: HTTP Server Foundation

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Step 1: Write failing test for server creation**

```go
// internal/server/server_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	return New(database, "test-secret-key-min-32-characters")
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - package doesn't exist

**Step 3: Add chi dependency**

Run: `go get github.com/go-chi/chi/v5`

**Step 4: Implement server.go**

```go
// internal/server/server.go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
)

type Server struct {
	db          *db.DB
	router      *chi.Mux
	authService *auth.Service
}

func New(database *db.DB, jwtSecret string) *Server {
	s := &Server{
		db:          database,
		router:      chi.NewRouter(),
		authService: auth.NewService(database, jwtSecret),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/health", s.handleHealth)
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.router)
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: HTTP server foundation with chi router

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Auth Handlers - Signup

**Files:**
- Create: `internal/server/auth_handlers.go`
- Modify: `internal/server/server.go`
- Create: `internal/server/auth_handlers_test.go`

**Step 1: Write failing test for signup endpoint**

```go
// internal/server/auth_handlers_test.go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignupEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", response["email"])
	}
	if response["id"] == nil {
		t.Error("expected id to be set")
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"email": "test@example.com", "password": "password123"}`

	// First signup
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Second signup with same email
	req = httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for duplicate email, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - route not found

**Step 3: Implement auth_handlers.go**

```go
// internal/server/auth_handlers.go
package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email and password are required")
		return
	}

	if len(req.Password) < 6 {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
		return
	}

	user, err := s.authService.CreateUser(req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.writeError(w, http.StatusBadRequest, "user_already_exists", "User already registered")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create user")
		return
	}

	response := map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"created_at":   user.CreatedAt,
		"updated_at":   user.UpdatedAt,
		"app_metadata": user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
```

**Step 4: Add route to server.go**

Update setupRoutes in server.go:

```go
func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/health", s.handleHealth)

	// Auth routes
	s.router.Route("/auth/v1", func(r chi.Router) {
		r.Post("/signup", s.handleSignup)
	})
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: signup auth endpoint

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Auth Handlers - Login (Token)

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/auth_handlers_test.go`
- Modify: `internal/server/server.go`

**Step 1: Write failing test for login endpoint**

Add to auth_handlers_test.go:

```go
func TestLoginEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// First create a user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Now login
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["access_token"] == nil {
		t.Error("expected access_token to be set")
	}
	if response["refresh_token"] == nil {
		t.Error("expected refresh_token to be set")
	}
	if response["token_type"] != "bearer" {
		t.Errorf("expected token_type=bearer, got %v", response["token_type"])
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	srv := setupTestServer(t)

	// First create a user
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Now try to login with wrong password
	loginBody := `{"email": "test@example.com", "password": "wrongpassword"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - route not found

**Step 3: Implement token handler**

Add to auth_handlers.go:

```go
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken  string         `json:"access_token"`
	TokenType    string         `json:"token_type"`
	ExpiresIn    int            `json:"expires_in"`
	RefreshToken string         `json:"refresh_token"`
	User         map[string]any `json:"user"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	grantType := r.URL.Query().Get("grant_type")

	switch grantType {
	case "password":
		s.handlePasswordGrant(w, r)
	case "refresh_token":
		s.handleRefreshGrant(w, r)
	default:
		s.writeError(w, http.StatusBadRequest, "invalid_grant", "Unsupported grant type")
	}
}

func (s *Server) handlePasswordGrant(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := s.authService.GetUserByEmail(req.Email)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if !s.authService.ValidatePassword(user, req.Password) {
		s.writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	session, refreshToken, err := s.authService.CreateSession(user)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create session")
		return
	}

	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	// Update last sign in
	s.authService.UpdateLastSignIn(user.ID)

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User: map[string]any{
			"id":            user.ID,
			"email":         user.Email,
			"role":          user.Role,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleRefreshGrant(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, session, refreshToken, err := s.authService.RefreshSession(req.RefreshToken)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid_grant", "Invalid refresh token")
		return
	}

	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User: map[string]any{
			"id":            user.ID,
			"email":         user.Email,
			"role":          user.Role,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		},
	}

	json.NewEncoder(w).Encode(response)
}
```

**Step 4: Add UpdateLastSignIn to auth service**

Add to internal/auth/user.go:

```go
func (s *Service) UpdateLastSignIn(userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec("UPDATE auth_users SET last_sign_in_at = ? WHERE id = ?", now, userID)
	return err
}
```

**Step 5: Add route to server.go**

Update the auth route:

```go
s.router.Route("/auth/v1", func(r chi.Router) {
	r.Post("/signup", s.handleSignup)
	r.Post("/token", s.handleToken)
})
```

**Step 6: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add -A && git commit -m "feat: login (token) auth endpoint with password and refresh grants

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Auth Handlers - Get/Update User

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/auth_handlers_test.go`
- Modify: `internal/server/server.go`
- Create: `internal/server/middleware.go`

**Step 1: Write failing test for user endpoint**

Add to auth_handlers_test.go:

```go
func TestGetUserEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login to get token
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Get user
	req = httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var userResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &userResp)
	if userResp["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", userResp["email"])
	}
}

func TestGetUserUnauthorized(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth/v1/user", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - route not found

**Step 3: Create middleware.go**

```go
// internal/server/middleware.go
package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserContextKey   contextKey = "user"
	ClaimsContextKey contextKey = "claims"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, http.StatusUnauthorized, "no_authorization", "Authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			s.writeError(w, http.StatusUnauthorized, "invalid_authorization", "Invalid authorization header format")
			return
		}

		claims, err := s.authService.ValidateAccessToken(parts[1])
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}

		userID := (*claims)["sub"].(string)
		user, err := s.authService.GetUserByID(userID)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "user_not_found", "User not found")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(r *http.Request) *auth.User {
	user, _ := r.Context().Value(UserContextKey).(*auth.User)
	return user
}

func GetClaimsFromContext(r *http.Request) *jwt.MapClaims {
	claims, _ := r.Context().Value(ClaimsContextKey).(*jwt.MapClaims)
	return claims
}
```

**Step 4: Add import for auth package in middleware.go**

Add to imports in middleware.go:

```go
import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/auth"
)
```

**Step 5: Implement user handler**

Add to auth_handlers.go:

```go
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "User not found in context")
		return
	}

	response := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	if user.EmailConfirmedAt != nil {
		response["email_confirmed_at"] = user.EmailConfirmedAt
	}
	if user.LastSignInAt != nil {
		response["last_sign_in_at"] = user.LastSignInAt
	}

	json.NewEncoder(w).Encode(response)
}

type UpdateUserRequest struct {
	Email        string         `json:"email,omitempty"`
	Password     string         `json:"password,omitempty"`
	UserMetadata map[string]any `json:"user_metadata,omitempty"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "User not found in context")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.UserMetadata != nil {
		if err := s.authService.UpdateUserMetadata(user.ID, req.UserMetadata); err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to update user")
			return
		}
	}

	if req.Password != "" {
		if len(req.Password) < 6 {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
			return
		}
		if err := s.authService.UpdatePassword(user.ID, req.Password); err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to update password")
			return
		}
	}

	// Refetch user to get updated data
	user, _ = s.authService.GetUserByID(user.ID)

	response := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	json.NewEncoder(w).Encode(response)
}
```

**Step 6: Add update methods to auth service**

Add to internal/auth/user.go:

```go
func (s *Service) UpdateUserMetadata(userID string, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET raw_user_meta_data = ?, updated_at = ? WHERE id = ?",
		string(metadataJSON), now, userID)
	return err
}

func (s *Service) UpdatePassword(userID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec("UPDATE auth_users SET encrypted_password = ?, updated_at = ? WHERE id = ?",
		string(hash), now, userID)
	return err
}
```

Add json import to user.go:

```go
import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/markb/sblite/internal/db"
	"golang.org/x/crypto/bcrypt"
)
```

**Step 7: Add routes to server.go**

Update the auth route:

```go
s.router.Route("/auth/v1", func(r chi.Router) {
	r.Post("/signup", s.handleSignup)
	r.Post("/token", s.handleToken)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/user", s.handleGetUser)
		r.Put("/user", s.handleUpdateUser)
	})
})
```

**Step 8: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 9: Commit**

```bash
git add -A && git commit -m "feat: get/update user endpoints with auth middleware

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Auth Handlers - Logout

**Files:**
- Modify: `internal/server/auth_handlers.go`
- Modify: `internal/server/auth_handlers_test.go`
- Modify: `internal/server/server.go`

**Step 1: Write failing test for logout endpoint**

Add to auth_handlers_test.go:

```go
func TestLogoutEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create user and login
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["access_token"].(string)

	// Logout
	req = httptest.NewRequest("POST", "/auth/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - route not found

**Step 3: Implement logout handler**

Add to auth_handlers.go:

```go
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r)
	if claims == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "Claims not found in context")
		return
	}

	sessionID, ok := (*claims)["session_id"].(string)
	if !ok || sessionID == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_session", "Session ID not found in token")
		return
	}

	if err := s.authService.RevokeSession(sessionID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to revoke session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Add route to server.go**

Update the protected routes:

```go
r.Group(func(r chi.Router) {
	r.Use(s.authMiddleware)
	r.Get("/user", s.handleGetUser)
	r.Put("/user", s.handleUpdateUser)
	r.Post("/logout", s.handleLogout)
})
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: logout endpoint with session revocation

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: REST API - Query Parser

**Files:**
- Create: `internal/rest/query.go`
- Create: `internal/rest/query_test.go`

**Step 1: Write failing test for query parser**

```go
// internal/rest/query_test.go
package rest

import (
	"reflect"
	"testing"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Filter
		wantErr  bool
	}{
		{
			name:     "eq operator",
			input:    "status=eq.active",
			expected: Filter{Column: "status", Operator: "eq", Value: "active"},
		},
		{
			name:     "neq operator",
			input:    "status=neq.deleted",
			expected: Filter{Column: "status", Operator: "neq", Value: "deleted"},
		},
		{
			name:     "gt operator",
			input:    "age=gt.21",
			expected: Filter{Column: "age", Operator: "gt", Value: "21"},
		},
		{
			name:     "gte operator",
			input:    "age=gte.21",
			expected: Filter{Column: "age", Operator: "gte", Value: "21"},
		},
		{
			name:     "lt operator",
			input:    "age=lt.65",
			expected: Filter{Column: "age", Operator: "lt", Value: "65"},
		},
		{
			name:     "lte operator",
			input:    "age=lte.65",
			expected: Filter{Column: "age", Operator: "lte", Value: "65"},
		},
		{
			name:     "is null",
			input:    "deleted=is.null",
			expected: Filter{Column: "deleted", Operator: "is", Value: "null"},
		},
		{
			name:    "invalid format",
			input:   "status",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(filter, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, filter)
			}
		})
	}
}

func TestFilterToSQL(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		expected string
		args     []any
	}{
		{
			name:     "eq operator",
			filter:   Filter{Column: "status", Operator: "eq", Value: "active"},
			expected: "\"status\" = ?",
			args:     []any{"active"},
		},
		{
			name:     "neq operator",
			filter:   Filter{Column: "status", Operator: "neq", Value: "deleted"},
			expected: "\"status\" != ?",
			args:     []any{"deleted"},
		},
		{
			name:     "is null",
			filter:   Filter{Column: "deleted", Operator: "is", Value: "null"},
			expected: "\"deleted\" IS NULL",
			args:     nil,
		},
		{
			name:     "is not null",
			filter:   Filter{Column: "deleted", Operator: "is", Value: "not.null"},
			expected: "\"deleted\" IS NOT NULL",
			args:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.filter.ToSQL()
			if sql != tt.expected {
				t.Errorf("expected SQL %q, got %q", tt.expected, sql)
			}
			if !reflect.DeepEqual(args, tt.args) {
				t.Errorf("expected args %v, got %v", tt.args, args)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/rest/... -v`
Expected: FAIL - package doesn't exist

**Step 3: Implement query.go**

```go
// internal/rest/query.go
package rest

import (
	"fmt"
	"strings"
)

type Filter struct {
	Column   string
	Operator string
	Value    string
}

type Query struct {
	Table   string
	Select  []string
	Filters []Filter
	Order   []OrderBy
	Limit   int
	Offset  int
}

type OrderBy struct {
	Column string
	Desc   bool
}

var validOperators = map[string]string{
	"eq":  "=",
	"neq": "!=",
	"gt":  ">",
	"gte": ">=",
	"lt":  "<",
	"lte": "<=",
	"is":  "IS",
}

func ParseFilter(input string) (Filter, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return Filter{}, fmt.Errorf("invalid filter format: %s", input)
	}

	column := parts[0]
	opValue := parts[1]

	opParts := strings.SplitN(opValue, ".", 2)
	if len(opParts) != 2 {
		return Filter{}, fmt.Errorf("invalid operator format: %s", opValue)
	}

	operator := opParts[0]
	value := opParts[1]

	if _, ok := validOperators[operator]; !ok {
		return Filter{}, fmt.Errorf("unknown operator: %s", operator)
	}

	return Filter{
		Column:   column,
		Operator: operator,
		Value:    value,
	}, nil
}

func (f Filter) ToSQL() (string, []any) {
	quotedColumn := fmt.Sprintf("\"%s\"", f.Column)

	switch f.Operator {
	case "is":
		if f.Value == "null" {
			return fmt.Sprintf("%s IS NULL", quotedColumn), nil
		}
		if f.Value == "not.null" {
			return fmt.Sprintf("%s IS NOT NULL", quotedColumn), nil
		}
		return fmt.Sprintf("%s IS ?", quotedColumn), []any{f.Value}
	default:
		sqlOp := validOperators[f.Operator]
		return fmt.Sprintf("%s %s ?", quotedColumn, sqlOp), []any{f.Value}
	}
}

func ParseSelect(selectParam string) []string {
	if selectParam == "" {
		return []string{"*"}
	}
	return strings.Split(selectParam, ",")
}

func ParseOrder(orderParam string) []OrderBy {
	if orderParam == "" {
		return nil
	}

	var orders []OrderBy
	parts := strings.Split(orderParam, ",")
	for _, part := range parts {
		order := OrderBy{Column: part, Desc: false}
		if strings.HasSuffix(part, ".desc") {
			order.Column = strings.TrimSuffix(part, ".desc")
			order.Desc = true
		} else if strings.HasSuffix(part, ".asc") {
			order.Column = strings.TrimSuffix(part, ".asc")
		}
		orders = append(orders, order)
	}
	return orders
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/rest/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: REST query parser with filter operators

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 12: REST API - SQL Builder

**Files:**
- Create: `internal/rest/builder.go`
- Create: `internal/rest/builder_test.go`

**Step 1: Write failing test for SQL builder**

```go
// internal/rest/builder_test.go
package rest

import (
	"testing"
)

func TestBuildSelectQuery(t *testing.T) {
	query := Query{
		Table:   "todos",
		Select:  []string{"id", "title", "completed"},
		Filters: []Filter{{Column: "completed", Operator: "eq", Value: "false"}},
		Order:   []OrderBy{{Column: "created_at", Desc: true}},
		Limit:   10,
		Offset:  0,
	}

	sql, args := BuildSelectQuery(query)

	expectedSQL := `SELECT "id", "title", "completed" FROM "todos" WHERE "completed" = ? ORDER BY "created_at" DESC LIMIT 10 OFFSET 0`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "false" {
		t.Errorf("expected args [false], got %v", args)
	}
}

func TestBuildInsertQuery(t *testing.T) {
	data := map[string]any{
		"title":     "Test Todo",
		"completed": false,
	}

	sql, args := BuildInsertQuery("todos", data)

	// Note: map iteration order is not guaranteed, so we check both possibilities
	if sql != `INSERT INTO "todos" ("completed", "title") VALUES (?, ?)` &&
		sql != `INSERT INTO "todos" ("title", "completed") VALUES (?, ?)` {
		t.Errorf("unexpected SQL: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpdateQuery(t *testing.T) {
	data := map[string]any{
		"completed": true,
	}
	filters := []Filter{{Column: "id", Operator: "eq", Value: "1"}}

	sql, args := BuildUpdateQuery("todos", data, filters)

	expectedSQL := `UPDATE "todos" SET "completed" = ? WHERE "id" = ?`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildDeleteQuery(t *testing.T) {
	filters := []Filter{{Column: "id", Operator: "eq", Value: "1"}}

	sql, args := BuildDeleteQuery("todos", filters)

	expectedSQL := `DELETE FROM "todos" WHERE "id" = ?`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "1" {
		t.Errorf("expected args [1], got %v", args)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/rest/... -v`
Expected: FAIL - BuildSelectQuery undefined

**Step 3: Implement builder.go**

```go
// internal/rest/builder.go
package rest

import (
	"fmt"
	"sort"
	"strings"
)

func BuildSelectQuery(q Query) (string, []any) {
	var args []any
	var sb strings.Builder

	// SELECT clause
	sb.WriteString("SELECT ")
	if len(q.Select) == 0 || (len(q.Select) == 1 && q.Select[0] == "*") {
		sb.WriteString("*")
	} else {
		quotedCols := make([]string, len(q.Select))
		for i, col := range q.Select {
			quotedCols[i] = fmt.Sprintf("\"%s\"", strings.TrimSpace(col))
		}
		sb.WriteString(strings.Join(quotedCols, ", "))
	}

	// FROM clause
	sb.WriteString(fmt.Sprintf(" FROM \"%s\"", q.Table))

	// WHERE clause
	if len(q.Filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range q.Filters {
			sql, filterArgs := f.ToSQL()
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// ORDER BY clause
	if len(q.Order) > 0 {
		sb.WriteString(" ORDER BY ")
		var orders []string
		for _, o := range q.Order {
			dir := "ASC"
			if o.Desc {
				dir = "DESC"
			}
			orders = append(orders, fmt.Sprintf("\"%s\" %s", o.Column, dir))
		}
		sb.WriteString(strings.Join(orders, ", "))
	}

	// LIMIT and OFFSET
	if q.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
	}
	sb.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))

	return sb.String(), args
}

func BuildInsertQuery(table string, data map[string]any) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	quotedCols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))

	for i, k := range keys {
		quotedCols[i] = fmt.Sprintf("\"%s\"", k)
		placeholders[i] = "?"
		args[i] = data[k]
	}

	sql := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s)`,
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	return sql, args
}

func BuildUpdateQuery(table string, data map[string]any, filters []Filter) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var args []any
	setClauses := make([]string, len(keys))
	for i, k := range keys {
		setClauses[i] = fmt.Sprintf("\"%s\" = ?", k)
		args = append(args, data[k])
	}

	sql := fmt.Sprintf(`UPDATE "%s" SET %s`, table, strings.Join(setClauses, ", "))

	if len(filters) > 0 {
		var conditions []string
		for _, f := range filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	return sql, args
}

func BuildDeleteQuery(table string, filters []Filter) (string, []any) {
	var args []any
	sql := fmt.Sprintf(`DELETE FROM "%s"`, table)

	if len(filters) > 0 {
		var conditions []string
		for _, f := range filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	return sql, args
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/rest/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: SQL query builder for REST operations

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 13: REST API - Handlers

**Files:**
- Create: `internal/rest/handler.go`
- Create: `internal/rest/handler_test.go`
- Modify: `internal/server/server.go`

**Step 1: Write failing test for REST handlers**

```go
// internal/rest/handler_test.go
package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
)

func setupTestHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create test table
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0,
			user_id TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("failed to create todos table: %v", err)
	}

	handler := NewHandler(database)
	return handler, database
}

func TestSelectHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	// Insert test data
	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 rows, got %d", len(response))
	}
}

func TestSelectWithFilter(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (title, completed) VALUES ('Test 1', 0), ('Test 2', 1)`)

	r := chi.NewRouter()
	r.Get("/rest/v1/{table}", handler.HandleSelect)

	req := httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response []map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)

	if len(response) != 1 {
		t.Errorf("expected 1 row, got %d", len(response))
	}
}

func TestInsertHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	r := chi.NewRouter()
	r.Post("/rest/v1/{table}", handler.HandleInsert)

	body := `{"title": "New Todo", "completed": 0}`
	req := httptest.NewRequest("POST", "/rest/v1/todos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (id, title, completed) VALUES (1, 'Test', 0)`)

	r := chi.NewRouter()
	r.Patch("/rest/v1/{table}", handler.HandleUpdate)

	body := `{"completed": 1}`
	req := httptest.NewRequest("PATCH", "/rest/v1/todos?id=eq.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteHandler(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	database.Exec(`INSERT INTO todos (id, title, completed) VALUES (1, 'Test', 0)`)

	r := chi.NewRouter()
	r.Delete("/rest/v1/{table}", handler.HandleDelete)

	req := httptest.NewRequest("DELETE", "/rest/v1/todos?id=eq.1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/rest/... -v`
Expected: FAIL - NewHandler undefined

**Step 3: Implement handler.go**

```go
// internal/rest/handler.go
package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
)

type Handler struct {
	db *db.DB
}

func NewHandler(database *db.DB) *Handler {
	return &Handler{db: database}
}

// Reserved query params that are not filters
var reservedParams = map[string]bool{
	"select": true,
	"order":  true,
	"limit":  true,
	"offset": true,
}

func (h *Handler) parseQueryParams(r *http.Request) Query {
	q := Query{
		Table:  chi.URLParam(r, "table"),
		Select: ParseSelect(r.URL.Query().Get("select")),
		Order:  ParseOrder(r.URL.Query().Get("order")),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		q.Limit, _ = strconv.Atoi(limit)
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		q.Offset, _ = strconv.Atoi(offset)
	}

	// Parse filters from query params
	for key, values := range r.URL.Query() {
		if reservedParams[key] {
			continue
		}
		for _, value := range values {
			filterStr := fmt.Sprintf("%s=%s", key, value)
			if filter, err := ParseFilter(filterStr); err == nil {
				q.Filters = append(q.Filters, filter)
			}
		}
	}

	return q
}

func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	sql, args := BuildSelectQuery(q)
	rows, err := h.db.Query(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "query_error", err.Error())
		return
	}
	defer rows.Close()

	results, err := h.scanRows(rows)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "scan_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) HandleInsert(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	sql, args := BuildInsertQuery(table, data)
	result, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "insert_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Return representation if requested
	prefer := r.Header.Get("Prefer")
	if strings.Contains(prefer, "return=representation") {
		lastID, _ := result.LastInsertId()
		q := Query{Table: table, Select: []string{"*"}, Filters: []Filter{{Column: "id", Operator: "eq", Value: fmt.Sprintf("%d", lastID)}}}
		selectSQL, selectArgs := BuildSelectQuery(q)
		rows, _ := h.db.Query(selectSQL, selectArgs...)
		defer rows.Close()
		results, _ := h.scanRows(rows)
		if len(results) > 0 {
			json.NewEncoder(w).Encode(results[0])
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]any{"inserted": true})
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	sql, args := BuildUpdateQuery(q.Table, data, q.Filters)
	_, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"updated": true})
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	if len(q.Filters) == 0 {
		h.writeError(w, http.StatusBadRequest, "missing_filter", "DELETE requires at least one filter")
		return
	}

	sql, args := BuildDeleteQuery(q.Table, q.Filters)
	_, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "delete_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, nil
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
```

Add import for database/sql:

```go
import (
	"database/sql"
	"encoding/json"
	...
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/rest/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: REST API handlers for CRUD operations

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 14: Wire Up REST Routes to Server

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Step 1: Write failing test for REST routes on server**

Add to server_test.go:

```go
func TestRESTSelect(t *testing.T) {
	srv := setupTestServer(t)

	// Create test table
	srv.db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0
		)
	`)
	srv.db.Exec(`INSERT INTO todos (title, completed) VALUES ('Test', 0)`)

	req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL - 404 not found

**Step 3: Add REST handler to server**

Update server.go:

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/rest"
)

type Server struct {
	db          *db.DB
	router      *chi.Mux
	authService *auth.Service
	restHandler *rest.Handler
}

func New(database *db.DB, jwtSecret string) *Server {
	s := &Server{
		db:          database,
		router:      chi.NewRouter(),
		authService: auth.NewService(database, jwtSecret),
		restHandler: rest.NewHandler(database),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/health", s.handleHealth)

	// Auth routes
	s.router.Route("/auth/v1", func(r chi.Router) {
		r.Post("/signup", s.handleSignup)
		r.Post("/token", s.handleToken)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/user", s.handleGetUser)
			r.Put("/user", s.handleUpdateUser)
			r.Post("/logout", s.handleLogout)
		})
	})

	// REST routes
	s.router.Route("/rest/v1", func(r chi.Router) {
		r.Get("/{table}", s.restHandler.HandleSelect)
		r.Post("/{table}", s.restHandler.HandleInsert)
		r.Patch("/{table}", s.restHandler.HandleUpdate)
		r.Delete("/{table}", s.restHandler.HandleDelete)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: wire REST handlers to server routes

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 15: CLI Commands - serve and init

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/serve.go`
- Modify: `cmd/init.go`

**Step 1: Implement init command**

```go
// cmd/init.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Supabase Lite database",
	Long:  `Creates a new SQLite database with the auth schema tables.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		// Check if file already exists
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("database already exists at %s", dbPath)
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		fmt.Printf("Initialized database at %s\n", dbPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("db", "data.db", "Path to database file")
}
```

**Step 2: Implement serve command**

```go
// cmd/serve.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supabase Lite server",
	Long:  `Starts the HTTP server with auth and REST API endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")

		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
			fmt.Println("Warning: Using default JWT secret. Set SBLITE_JWT_SECRET in production.")
		}

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s. Run 'sblite init' first", dbPath)
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations in case schema is outdated
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		srv := server.New(database, jwtSecret)
		addr := fmt.Sprintf("%s:%d", host, port)
		fmt.Printf("Starting Supabase Lite on %s\n", addr)
		fmt.Printf("  Auth API: http://%s/auth/v1\n", addr)
		fmt.Printf("  REST API: http://%s/rest/v1\n", addr)

		return srv.ListenAndServe(addr)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
}
```

**Step 3: Build and test CLI**

Run: `go build -o sblite .`
Expected: Binary builds successfully

Run: `./sblite --help`
Expected: Shows help with init and serve commands

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: CLI commands for init and serve

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 16: Integration Test

**Files:**
- Create: `integration_test.go`

**Step 1: Write integration test**

```go
// integration_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/server"
)

func TestFullAuthFlow(t *testing.T) {
	// Setup
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	srv := server.New(database, "test-secret-key-min-32-characters")

	// 1. Signup
	signupBody := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("signup failed: %d %s", w.Code, w.Body.String())
	}

	// 2. Login
	loginBody := `{"email": "test@example.com", "password": "password123"}`
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=password", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}

	var loginResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	accessToken := loginResp["access_token"].(string)
	refreshToken := loginResp["refresh_token"].(string)

	// 3. Get user
	req = httptest.NewRequest("GET", "/auth/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get user failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Refresh token
	refreshBody := map[string]string{"refresh_token": refreshToken}
	refreshJSON, _ := json.Marshal(refreshBody)
	req = httptest.NewRequest("POST", "/auth/v1/token?grant_type=refresh_token", bytes.NewBuffer(refreshJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d %s", w.Code, w.Body.String())
	}

	var refreshResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &refreshResp)
	newAccessToken := refreshResp["access_token"].(string)

	// 5. Logout
	req = httptest.NewRequest("POST", "/auth/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer "+newAccessToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("logout failed: %d %s", w.Code, w.Body.String())
	}

	t.Log("Full auth flow completed successfully")
}

func TestFullRESTFlow(t *testing.T) {
	// Setup
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create test table
	database.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
		)
	`)

	srv := server.New(database, "test-secret-key-min-32-characters")

	// 1. Create todo
	createBody := `{"title": "Test Todo", "completed": 0}`
	req := httptest.NewRequest("POST", "/rest/v1/todos", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
	}

	// 2. Read todos
	req = httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("read failed: %d %s", w.Code, w.Body.String())
	}

	var todos []map[string]any
	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// 3. Update todo
	updateBody := `{"completed": 1}`
	req = httptest.NewRequest("PATCH", "/rest/v1/todos?id=eq.1", bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Read with filter
	req = httptest.NewRequest("GET", "/rest/v1/todos?completed=eq.1", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("filtered read failed: %d %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 completed todo, got %d", len(todos))
	}

	// 5. Delete todo
	req = httptest.NewRequest("DELETE", "/rest/v1/todos?id=eq.1", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}

	// 6. Verify deletion
	req = httptest.NewRequest("GET", "/rest/v1/todos", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &todos)
	if len(todos) != 0 {
		t.Fatalf("expected 0 todos after deletion, got %d", len(todos))
	}

	t.Log("Full REST flow completed successfully")
}
```

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 3: Commit**

```bash
git add -A && git commit -m "test: integration tests for auth and REST flows

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Final Directory Structure

```
sblite/
├── cmd/
│   ├── init.go
│   ├── root.go
│   └── serve.go
├── internal/
│   ├── auth/
│   │   ├── jwt.go
│   │   ├── jwt_test.go
│   │   ├── user.go
│   │   └── user_test.go
│   ├── db/
│   │   ├── db.go
│   │   ├── db_test.go
│   │   ├── migrations.go
│   │   └── migrations_test.go
│   ├── rest/
│   │   ├── builder.go
│   │   ├── builder_test.go
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   ├── query.go
│   │   └── query_test.go
│   └── server/
│       ├── auth_handlers.go
│       ├── auth_handlers_test.go
│       ├── middleware.go
│       ├── server.go
│       └── server_test.go
├── docs/
│   └── plans/
│       ├── 2026-01-16-phase1-foundation.md
│       └── 2026-01-16-supabase-lite-design.md
├── go.mod
├── go.sum
├── integration_test.go
└── main.go
```

---

Plan complete and saved to `docs/plans/2026-01-16-phase1-foundation.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session in worktree, batch execution with checkpoints

Which approach?