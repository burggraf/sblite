# Email System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a complete email system for sblite with full Supabase parity, supporting email confirmation, password reset, magic links, email change verification, and user invitations.

**Architecture:** Three-layer design with Mailer interface (LogMailer, CatchMailer, SMTPMailer implementations), EmailService for high-level operations, and TemplateService for database-stored templates. Includes embedded web UI for mail catching mode.

**Tech Stack:** Go standard library `net/smtp`, `html/template`, `text/template`, `embed`; Chi router for web UI; SQLite for storage.

---

## Task 1: Add Database Migrations for Email Tables

**Files:**
- Modify: `internal/db/migrations.go`
- Test: `internal/db/migrations_test.go`

**Step 1: Write the failing test**

Add to `internal/db/migrations_test.go`:

```go
func TestEmailTablesMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test auth_emails table exists
	_, err := db.Exec(`INSERT INTO auth_emails (id, to_email, from_email, subject, email_type, created_at)
		VALUES ('test-id', 'to@test.com', 'from@test.com', 'Test', 'confirmation', datetime('now'))`)
	if err != nil {
		t.Fatalf("auth_emails table should exist: %v", err)
	}

	// Test auth_email_templates table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM auth_email_templates").Scan(&count)
	if err != nil {
		t.Fatalf("auth_email_templates table should exist: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 default templates, got %d", count)
	}

	// Test auth_verification_tokens table exists
	_, err = db.Exec(`INSERT INTO auth_verification_tokens (id, user_id, type, email, expires_at, created_at)
		VALUES ('token-id', 'user-id', 'confirmation', 'test@test.com', datetime('now', '+1 day'), datetime('now'))`)
	if err != nil {
		t.Fatalf("auth_verification_tokens table should exist: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/db/... -run TestEmailTablesMigration -v`

Expected: FAIL with "no such table: auth_emails"

**Step 3: Add email schema to migrations.go**

Add after `rlsSchema` constant in `internal/db/migrations.go`:

```go
const emailSchema = `
CREATE TABLE IF NOT EXISTS auth_emails (
    id TEXT PRIMARY KEY,
    to_email TEXT NOT NULL,
    from_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT,
    body_text TEXT,
    email_type TEXT NOT NULL,
    user_id TEXT,
    created_at TEXT NOT NULL,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_emails_created_at ON auth_emails(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_emails_type ON auth_emails(email_type);

CREATE TABLE IF NOT EXISTS auth_email_templates (
    id TEXT PRIMARY KEY,
    type TEXT UNIQUE NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_verification_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    email TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_verification_tokens_user ON auth_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_verification_tokens_type ON auth_verification_tokens(type);
`

const defaultTemplates = `
INSERT OR IGNORE INTO auth_email_templates (id, type, subject, body_html, body_text, updated_at) VALUES
('tpl-confirmation', 'confirmation', 'Confirm your email',
'<h2>Confirm your email</h2><p>Click the link below to confirm your email address:</p><p><a href="{{.ConfirmationURL}}">Confirm Email</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Confirm your email\n\nClick the link below to confirm your email address:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-recovery', 'recovery', 'Reset your password',
'<h2>Reset your password</h2><p>Click the link below to reset your password:</p><p><a href="{{.ConfirmationURL}}">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Reset your password\n\nClick the link below to reset your password:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-magic_link', 'magic_link', 'Your login link',
'<h2>Your login link</h2><p>Click the link below to sign in:</p><p><a href="{{.ConfirmationURL}}">Sign In</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Your login link\n\nClick the link below to sign in:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-email_change', 'email_change', 'Confirm email change',
'<h2>Confirm your new email</h2><p>Click the link below to confirm your new email address:</p><p><a href="{{.ConfirmationURL}}">Confirm New Email</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Confirm your new email\n\nClick the link below to confirm your new email address:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-invite', 'invite', 'You have been invited',
'<h2>You have been invited</h2><p>Click the link below to accept your invitation and set your password:</p><p><a href="{{.ConfirmationURL}}">Accept Invitation</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'You have been invited\n\nClick the link below to accept your invitation and set your password:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.',
datetime('now'));
`
```

**Step 4: Update RunMigrations function**

Modify `RunMigrations` in `internal/db/migrations.go`:

```go
func (db *DB) RunMigrations() error {
	_, err := db.Exec(authSchema)
	if err != nil {
		return fmt.Errorf("failed to run auth migrations: %w", err)
	}

	_, err = db.Exec(rlsSchema)
	if err != nil {
		return fmt.Errorf("failed to run RLS migrations: %w", err)
	}

	_, err = db.Exec(emailSchema)
	if err != nil {
		return fmt.Errorf("failed to run email migrations: %w", err)
	}

	_, err = db.Exec(defaultTemplates)
	if err != nil {
		return fmt.Errorf("failed to seed email templates: %w", err)
	}

	return nil
}
```

**Step 5: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/db/... -run TestEmailTablesMigration -v`

Expected: PASS

**Step 6: Run all db tests to ensure no regressions**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/db/... -v`

Expected: All tests PASS

**Step 7: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/db/migrations.go internal/db/migrations_test.go
git commit -m "feat(db): add email tables and default templates migration"
```

---

## Task 2: Create Mail Package - Interface and Types

**Files:**
- Create: `internal/mail/mailer.go`
- Create: `internal/mail/mailer_test.go`

**Step 1: Write the failing test**

Create `internal/mail/mailer_test.go`:

```go
// internal/mail/mailer_test.go
package mail

import (
	"testing"
)

func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
	}{
		{
			name: "valid message",
			msg: &Message{
				To:       "user@example.com",
				From:     "noreply@example.com",
				Subject:  "Test",
				BodyHTML: "<p>Test</p>",
				Type:     TypeConfirmation,
			},
			wantErr: false,
		},
		{
			name: "missing to",
			msg: &Message{
				From:    "noreply@example.com",
				Subject: "Test",
				Type:    TypeConfirmation,
			},
			wantErr: true,
		},
		{
			name: "missing subject",
			msg: &Message{
				To:   "user@example.com",
				From: "noreply@example.com",
				Type: TypeConfirmation,
			},
			wantErr: true,
		},
		{
			name: "missing body",
			msg: &Message{
				To:      "user@example.com",
				From:    "noreply@example.com",
				Subject: "Test",
				Type:    TypeConfirmation,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -v`

Expected: FAIL (package doesn't exist)

**Step 3: Create the mailer.go file**

Create `internal/mail/mailer.go`:

```go
// Package mail provides email sending functionality for sblite.
package mail

import (
	"context"
	"fmt"
)

// Email types
const (
	TypeConfirmation = "confirmation"
	TypeRecovery     = "recovery"
	TypeMagicLink    = "magic_link"
	TypeEmailChange  = "email_change"
	TypeInvite       = "invite"
)

// Mail modes
const (
	ModeLog   = "log"
	ModeCatch = "catch"
	ModeSMTP  = "smtp"
)

// Message represents an email message.
type Message struct {
	To       string
	From     string
	Subject  string
	BodyHTML string
	BodyText string
	Type     string
	UserID   string
	Metadata map[string]any
}

// Validate checks if the message has all required fields.
func (m *Message) Validate() error {
	if m.To == "" {
		return fmt.Errorf("to address is required")
	}
	if m.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if m.BodyHTML == "" && m.BodyText == "" {
		return fmt.Errorf("body (html or text) is required")
	}
	return nil
}

// Mailer is the interface for sending emails.
type Mailer interface {
	Send(ctx context.Context, msg *Message) error
}

// Config holds email configuration.
type Config struct {
	Mode     string // log, catch, smtp
	From     string // default sender address
	SiteURL  string // base URL for email links
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Mode:     ModeLog,
		From:     "noreply@localhost",
		SiteURL:  "http://localhost:8080",
		SMTPPort: 587,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -v`

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/
git commit -m "feat(mail): add Mailer interface and Message type"
```

---

## Task 3: Implement LogMailer

**Files:**
- Create: `internal/mail/log_mailer.go`
- Create: `internal/mail/log_mailer_test.go`

**Step 1: Write the failing test**

Create `internal/mail/log_mailer_test.go`:

```go
// internal/mail/log_mailer_test.go
package mail

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLogMailer_Send(t *testing.T) {
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyText: "Test body content",
		Type:     TypeConfirmation,
	}

	err := mailer.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user@example.com") {
		t.Errorf("output should contain recipient email")
	}
	if !strings.Contains(output, "Test Subject") {
		t.Errorf("output should contain subject")
	}
	if !strings.Contains(output, "Test body content") {
		t.Errorf("output should contain body")
	}
}

func TestLogMailer_SendValidationError(t *testing.T) {
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)

	msg := &Message{} // invalid - missing required fields

	err := mailer.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("Send() should return error for invalid message")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestLogMailer -v`

Expected: FAIL (NewLogMailer not found)

**Step 3: Implement LogMailer**

Create `internal/mail/log_mailer.go`:

```go
// internal/mail/log_mailer.go
package mail

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogMailer logs emails to an io.Writer (default: stdout).
type LogMailer struct {
	w io.Writer
}

// NewLogMailer creates a new LogMailer. If w is nil, os.Stdout is used.
func NewLogMailer(w io.Writer) *LogMailer {
	if w == nil {
		w = os.Stdout
	}
	return &LogMailer{w: w}
}

// Send logs the email to the writer.
func (m *LogMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  EMAIL [%s] %s\n", strings.ToUpper(msg.Type), time.Now().Format(time.RFC3339)))
	sb.WriteString("════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  From:    %s\n", msg.From))
	sb.WriteString(fmt.Sprintf("  To:      %s\n", msg.To))
	sb.WriteString(fmt.Sprintf("  Subject: %s\n", msg.Subject))
	sb.WriteString("────────────────────────────────────────────────────────────\n")

	// Prefer text body for log output, fall back to HTML
	body := msg.BodyText
	if body == "" {
		body = msg.BodyHTML
	}
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString("════════════════════════════════════════════════════════════\n\n")

	_, err := fmt.Fprint(m.w, sb.String())
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestLogMailer -v`

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/log_mailer.go internal/mail/log_mailer_test.go
git commit -m "feat(mail): implement LogMailer for stdout output"
```

---

## Task 4: Implement CatchMailer

**Files:**
- Create: `internal/mail/catch_mailer.go`
- Create: `internal/mail/catch_mailer_test.go`

**Step 1: Write the failing test**

Create `internal/mail/catch_mailer_test.go`:

```go
// internal/mail/catch_mailer_test.go
package mail

import (
	"context"
	"os"
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return database
}

func TestCatchMailer_Send(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>Test HTML</p>",
		BodyText: "Test text",
		Type:     TypeConfirmation,
		UserID:   "user-123",
	}

	err := mailer.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify email was stored
	emails, err := mailer.ListEmails(10, 0)
	if err != nil {
		t.Fatalf("ListEmails() error = %v", err)
	}
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	if emails[0].To != "user@example.com" {
		t.Errorf("expected to = user@example.com, got %s", emails[0].To)
	}
}

func TestCatchMailer_GetEmail(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>Test</p>",
		Type:     TypeRecovery,
	}

	_ = mailer.Send(context.Background(), msg)

	emails, _ := mailer.ListEmails(10, 0)
	email, err := mailer.GetEmail(emails[0].ID)
	if err != nil {
		t.Fatalf("GetEmail() error = %v", err)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("expected subject = Test Subject, got %s", email.Subject)
	}
}

func TestCatchMailer_DeleteEmail(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test",
		BodyHTML: "<p>Test</p>",
		Type:     TypeConfirmation,
	}

	_ = mailer.Send(context.Background(), msg)
	emails, _ := mailer.ListEmails(10, 0)

	err := mailer.DeleteEmail(emails[0].ID)
	if err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}

	emails, _ = mailer.ListEmails(10, 0)
	if len(emails) != 0 {
		t.Errorf("expected 0 emails after delete, got %d", len(emails))
	}
}

func TestCatchMailer_ClearAll(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	for i := 0; i < 3; i++ {
		msg := &Message{
			To:       "user@example.com",
			From:     "noreply@example.com",
			Subject:  "Test",
			BodyHTML: "<p>Test</p>",
			Type:     TypeConfirmation,
		}
		_ = mailer.Send(context.Background(), msg)
	}

	err := mailer.ClearAll()
	if err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}

	emails, _ := mailer.ListEmails(10, 0)
	if len(emails) != 0 {
		t.Errorf("expected 0 emails after clear, got %d", len(emails))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestCatchMailer -v`

Expected: FAIL (NewCatchMailer not found)

**Step 3: Implement CatchMailer**

Create `internal/mail/catch_mailer.go`:

```go
// internal/mail/catch_mailer.go
package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/markb/sblite/internal/db"
)

// CaughtEmail represents a stored email in catch mode.
type CaughtEmail struct {
	ID        string         `json:"id"`
	To        string         `json:"to"`
	From      string         `json:"from"`
	Subject   string         `json:"subject"`
	BodyHTML  string         `json:"body_html,omitempty"`
	BodyText  string         `json:"body_text,omitempty"`
	Type      string         `json:"type"`
	UserID    string         `json:"user_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// CatchMailer stores emails in the database for local development.
type CatchMailer struct {
	db *db.DB
}

// NewCatchMailer creates a new CatchMailer.
func NewCatchMailer(database *db.DB) *CatchMailer {
	return &CatchMailer{db: database}
}

// Send stores the email in the database.
func (m *CatchMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	var metadataJSON *string
	if msg.Metadata != nil {
		b, err := json.Marshal(msg.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		s := string(b)
		metadataJSON = &s
	}

	_, err := m.db.Exec(`
		INSERT INTO auth_emails (id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, msg.To, msg.From, msg.Subject, msg.BodyHTML, msg.BodyText, msg.Type, msg.UserID, now, metadataJSON)

	if err != nil {
		return fmt.Errorf("failed to store email: %w", err)
	}

	return nil
}

// ListEmails returns caught emails, newest first.
func (m *CatchMailer) ListEmails(limit, offset int) ([]CaughtEmail, error) {
	rows, err := m.db.Query(`
		SELECT id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata
		FROM auth_emails
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list emails: %w", err)
	}
	defer rows.Close()

	var emails []CaughtEmail
	for rows.Next() {
		var e CaughtEmail
		var bodyHTML, bodyText, userID, metadataJSON *string
		var createdAt string

		err := rows.Scan(&e.ID, &e.To, &e.From, &e.Subject, &bodyHTML, &bodyText, &e.Type, &userID, &createdAt, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}

		if bodyHTML != nil {
			e.BodyHTML = *bodyHTML
		}
		if bodyText != nil {
			e.BodyText = *bodyText
		}
		if userID != nil {
			e.UserID = *userID
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if metadataJSON != nil {
			json.Unmarshal([]byte(*metadataJSON), &e.Metadata)
		}

		emails = append(emails, e)
	}

	return emails, nil
}

// GetEmail returns a single email by ID.
func (m *CatchMailer) GetEmail(id string) (*CaughtEmail, error) {
	var e CaughtEmail
	var bodyHTML, bodyText, userID, metadataJSON *string
	var createdAt string

	err := m.db.QueryRow(`
		SELECT id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata
		FROM auth_emails WHERE id = ?
	`, id).Scan(&e.ID, &e.To, &e.From, &e.Subject, &bodyHTML, &bodyText, &e.Type, &userID, &createdAt, &metadataJSON)

	if err != nil {
		return nil, fmt.Errorf("email not found: %w", err)
	}

	if bodyHTML != nil {
		e.BodyHTML = *bodyHTML
	}
	if bodyText != nil {
		e.BodyText = *bodyText
	}
	if userID != nil {
		e.UserID = *userID
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if metadataJSON != nil {
		json.Unmarshal([]byte(*metadataJSON), &e.Metadata)
	}

	return &e, nil
}

// DeleteEmail removes a single email.
func (m *CatchMailer) DeleteEmail(id string) error {
	_, err := m.db.Exec("DELETE FROM auth_emails WHERE id = ?", id)
	return err
}

// ClearAll removes all caught emails.
func (m *CatchMailer) ClearAll() error {
	_, err := m.db.Exec("DELETE FROM auth_emails")
	return err
}

// Count returns the total number of caught emails.
func (m *CatchMailer) Count() (int, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM auth_emails").Scan(&count)
	return count, err
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestCatchMailer -v`

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/catch_mailer.go internal/mail/catch_mailer_test.go
git commit -m "feat(mail): implement CatchMailer for local development"
```

---

## Task 5: Implement SMTPMailer

**Files:**
- Create: `internal/mail/smtp_mailer.go`
- Create: `internal/mail/smtp_mailer_test.go`

**Step 1: Write the failing test**

Create `internal/mail/smtp_mailer_test.go`:

```go
// internal/mail/smtp_mailer_test.go
package mail

import (
	"context"
	"testing"
)

func TestSMTPMailer_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  SMTPConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
				User: "user",
				Pass: "pass",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: SMTPConfig{
				Port: 587,
				User: "user",
				Pass: "pass",
			},
			wantErr: true,
		},
		{
			name: "missing credentials",
			config: SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSMTPMailer_BuildMessage(t *testing.T) {
	mailer := NewSMTPMailer(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		User: "user",
		Pass: "pass",
	})

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>HTML body</p>",
		BodyText: "Text body",
		Type:     TypeConfirmation,
	}

	raw := mailer.buildMessage(msg)

	if len(raw) == 0 {
		t.Error("buildMessage should return non-empty byte slice")
	}
}

func TestSMTPMailer_SendValidationError(t *testing.T) {
	mailer := NewSMTPMailer(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		User: "user",
		Pass: "pass",
	})

	msg := &Message{} // invalid

	err := mailer.Send(context.Background(), msg)
	if err == nil {
		t.Error("Send() should return error for invalid message")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestSMTPMailer -v`

Expected: FAIL (SMTPConfig not found)

**Step 3: Implement SMTPMailer**

Create `internal/mail/smtp_mailer.go`:

```go
// internal/mail/smtp_mailer.go
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"
)

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
}

// Validate checks if the SMTP configuration is complete.
func (c *SMTPConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if c.User == "" || c.Pass == "" {
		return fmt.Errorf("SMTP credentials are required")
	}
	return nil
}

// SMTPMailer sends emails via SMTP.
type SMTPMailer struct {
	config SMTPConfig
}

// NewSMTPMailer creates a new SMTPMailer.
func NewSMTPMailer(config SMTPConfig) *SMTPMailer {
	return &SMTPMailer{config: config}
}

// Send sends the email via SMTP.
func (m *SMTPMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	auth := smtp.PlainAuth("", m.config.User, m.config.Pass, m.config.Host)

	// Build the email message
	body := m.buildMessage(msg)

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Upgrade to TLS if port is 587
	client, err := smtp.NewClient(conn, m.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// STARTTLS for port 587
	if m.config.Port == 587 {
		tlsConfig := &tls.Config{ServerName: m.config.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// buildMessage creates a MIME multipart email message.
func (m *SMTPMailer) buildMessage(msg *Message) []byte {
	var buf bytes.Buffer

	// Create multipart writer
	writer := multipart.NewWriter(&buf)

	// Write headers
	headers := make(textproto.MIMEHeader)
	headers.Set("From", msg.From)
	headers.Set("To", msg.To)
	headers.Set("Subject", msg.Subject)
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", writer.Boundary()))
	headers.Set("Date", time.Now().Format(time.RFC1123Z))

	// Write headers to buffer
	for k, v := range headers {
		buf.Reset() // Clear for fresh start
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v[0])
	}

	// Rebuild with proper structure
	buf.Reset()
	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%s\r\n", writer.Boundary())
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "\r\n")

	// Text part
	if msg.BodyText != "" {
		textPart, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"text/plain; charset=utf-8"},
		})
		textPart.Write([]byte(msg.BodyText))
	}

	// HTML part
	if msg.BodyHTML != "" {
		htmlPart, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"text/html; charset=utf-8"},
		})
		htmlPart.Write([]byte(msg.BodyHTML))
	}

	writer.Close()

	return buf.Bytes()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestSMTPMailer -v`

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/smtp_mailer.go internal/mail/smtp_mailer_test.go
git commit -m "feat(mail): implement SMTPMailer for production email delivery"
```

---

## Task 6: Implement Template Service

**Files:**
- Create: `internal/mail/templates.go`
- Create: `internal/mail/templates_test.go`

**Step 1: Write the failing test**

Create `internal/mail/templates_test.go`:

```go
// internal/mail/templates_test.go
package mail

import (
	"testing"
)

func TestTemplateService_GetTemplate(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	tpl, err := svc.GetTemplate(TypeConfirmation)
	if err != nil {
		t.Fatalf("GetTemplate() error = %v", err)
	}

	if tpl.Type != TypeConfirmation {
		t.Errorf("expected type = confirmation, got %s", tpl.Type)
	}
	if tpl.Subject == "" {
		t.Error("expected non-empty subject")
	}
}

func TestTemplateService_ListTemplates(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	templates, err := svc.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}

	if len(templates) != 5 {
		t.Errorf("expected 5 templates, got %d", len(templates))
	}
}

func TestTemplateService_UpdateTemplate(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	err := svc.UpdateTemplate(TypeConfirmation, "New Subject", "<p>New HTML</p>", "New text")
	if err != nil {
		t.Fatalf("UpdateTemplate() error = %v", err)
	}

	tpl, _ := svc.GetTemplate(TypeConfirmation)
	if tpl.Subject != "New Subject" {
		t.Errorf("expected subject = New Subject, got %s", tpl.Subject)
	}
}

func TestTemplateService_Render(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	data := TemplateData{
		SiteURL:         "https://example.com",
		ConfirmationURL: "https://example.com/auth/v1/verify?token=abc123",
		Email:           "user@example.com",
		Token:           "abc123",
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := svc.Render(TypeConfirmation, data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if subject == "" {
		t.Error("expected non-empty subject")
	}
	if html == "" {
		t.Error("expected non-empty HTML")
	}
	if text == "" {
		t.Error("expected non-empty text")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestTemplateService -v`

Expected: FAIL (NewTemplateService not found)

**Step 3: Implement TemplateService**

Create `internal/mail/templates.go`:

```go
// internal/mail/templates.go
package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"sync"
	texttemplate "text/template"
	"time"

	"github.com/markb/sblite/internal/db"
)

// EmailTemplate represents a stored email template.
type EmailTemplate struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Subject   string    `json:"subject"`
	BodyHTML  string    `json:"body_html"`
	BodyText  string    `json:"body_text,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateData contains variables for template rendering.
type TemplateData struct {
	SiteURL         string
	ConfirmationURL string
	Email           string
	Token           string
	ExpiresIn       string
}

// TemplateService manages email templates.
type TemplateService struct {
	db    *db.DB
	cache map[string]*EmailTemplate
	mu    sync.RWMutex
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(database *db.DB) *TemplateService {
	return &TemplateService{
		db:    database,
		cache: make(map[string]*EmailTemplate),
	}
}

// GetTemplate retrieves a template by type.
func (s *TemplateService) GetTemplate(templateType string) (*EmailTemplate, error) {
	// Check cache first
	s.mu.RLock()
	if tpl, ok := s.cache[templateType]; ok {
		s.mu.RUnlock()
		return tpl, nil
	}
	s.mu.RUnlock()

	// Query database
	var tpl EmailTemplate
	var bodyText *string
	var updatedAt string

	err := s.db.QueryRow(`
		SELECT id, type, subject, body_html, body_text, updated_at
		FROM auth_email_templates WHERE type = ?
	`, templateType).Scan(&tpl.ID, &tpl.Type, &tpl.Subject, &tpl.BodyHTML, &bodyText, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	if bodyText != nil {
		tpl.BodyText = *bodyText
	}
	tpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Update cache
	s.mu.Lock()
	s.cache[templateType] = &tpl
	s.mu.Unlock()

	return &tpl, nil
}

// ListTemplates returns all templates.
func (s *TemplateService) ListTemplates() ([]EmailTemplate, error) {
	rows, err := s.db.Query(`
		SELECT id, type, subject, body_html, body_text, updated_at
		FROM auth_email_templates ORDER BY type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	var templates []EmailTemplate
	for rows.Next() {
		var tpl EmailTemplate
		var bodyText *string
		var updatedAt string

		err := rows.Scan(&tpl.ID, &tpl.Type, &tpl.Subject, &tpl.BodyHTML, &bodyText, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}

		if bodyText != nil {
			tpl.BodyText = *bodyText
		}
		tpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		templates = append(templates, tpl)
	}

	return templates, nil
}

// UpdateTemplate updates a template by type.
func (s *TemplateService) UpdateTemplate(templateType, subject, bodyHTML, bodyText string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		UPDATE auth_email_templates
		SET subject = ?, body_html = ?, body_text = ?, updated_at = ?
		WHERE type = ?
	`, subject, bodyHTML, bodyText, now, templateType)

	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	// Invalidate cache
	s.mu.Lock()
	delete(s.cache, templateType)
	s.mu.Unlock()

	return nil
}

// Render renders a template with the given data.
func (s *TemplateService) Render(templateType string, data TemplateData) (subject, html, text string, err error) {
	tpl, err := s.GetTemplate(templateType)
	if err != nil {
		return "", "", "", err
	}

	// Render subject (plain text template)
	subjectTpl, err := texttemplate.New("subject").Parse(tpl.Subject)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTpl.Execute(&subjectBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}
	subject = subjectBuf.String()

	// Render HTML body
	htmlTpl, err := template.New("html").Parse(tpl.BodyHTML)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse HTML template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := htmlTpl.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render HTML: %w", err)
	}
	html = htmlBuf.String()

	// Render text body
	if tpl.BodyText != "" {
		textTpl, err := texttemplate.New("text").Parse(tpl.BodyText)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse text template: %w", err)
		}
		var textBuf bytes.Buffer
		if err := textTpl.Execute(&textBuf, data); err != nil {
			return "", "", "", fmt.Errorf("failed to render text: %w", err)
		}
		text = textBuf.String()
	}

	return subject, html, text, nil
}

// InvalidateCache clears the template cache.
func (s *TemplateService) InvalidateCache() {
	s.mu.Lock()
	s.cache = make(map[string]*EmailTemplate)
	s.mu.Unlock()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestTemplateService -v`

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/templates.go internal/mail/templates_test.go
git commit -m "feat(mail): implement TemplateService with caching and rendering"
```

---

## Task 7: Implement Email Service

**Files:**
- Create: `internal/mail/service.go`
- Create: `internal/mail/service_test.go`

**Step 1: Write the failing test**

Create `internal/mail/service_test.go`:

```go
// internal/mail/service_test.go
package mail

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmailService_SendConfirmation(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendConfirmation("user-123", "user@example.com", "token123")
	if err != nil {
		t.Fatalf("SendConfirmation() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user@example.com") {
		t.Error("output should contain recipient email")
	}
	if !strings.Contains(output, "CONFIRMATION") {
		t.Error("output should indicate confirmation type")
	}
}

func TestEmailService_SendRecovery(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendRecovery("user-123", "user@example.com", "token456")
	if err != nil {
		t.Fatalf("SendRecovery() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "RECOVERY") {
		t.Error("output should indicate recovery type")
	}
}

func TestEmailService_SendMagicLink(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendMagicLink("user@example.com", "token789")
	if err != nil {
		t.Fatalf("SendMagicLink() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MAGIC_LINK") {
		t.Error("output should indicate magic_link type")
	}
}

func TestEmailService_SendEmailChange(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendEmailChange("user-123", "new@example.com", "tokenABC")
	if err != nil {
		t.Fatalf("SendEmailChange() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "new@example.com") {
		t.Error("output should contain new email")
	}
	if !strings.Contains(output, "EMAIL_CHANGE") {
		t.Error("output should indicate email_change type")
	}
}

func TestEmailService_SendInvite(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendInvite("invited@example.com", "tokenXYZ")
	if err != nil {
		t.Fatalf("SendInvite() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "invited@example.com") {
		t.Error("output should contain invited email")
	}
	if !strings.Contains(output, "INVITE") {
		t.Error("output should indicate invite type")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestEmailService -v`

Expected: FAIL (NewEmailService not found)

**Step 3: Implement EmailService**

Create `internal/mail/service.go`:

```go
// internal/mail/service.go
package mail

import (
	"context"
	"fmt"
)

// EmailService provides high-level email sending operations.
type EmailService struct {
	mailer    Mailer
	templates *TemplateService
	config    *Config
}

// NewEmailService creates a new EmailService.
func NewEmailService(mailer Mailer, templates *TemplateService, config *Config) *EmailService {
	return &EmailService{
		mailer:    mailer,
		templates: templates,
		config:    config,
	}
}

// SendConfirmation sends an email confirmation message.
func (s *EmailService) SendConfirmation(userID, email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=signup", s.config.SiteURL, token)

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := s.templates.Render(TypeConfirmation, data)
	if err != nil {
		return fmt.Errorf("failed to render confirmation template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeConfirmation,
		UserID:   userID,
	}

	return s.mailer.Send(context.Background(), msg)
}

// SendRecovery sends a password recovery email.
func (s *EmailService) SendRecovery(userID, email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=recovery", s.config.SiteURL, token)

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "1 hour",
	}

	subject, html, text, err := s.templates.Render(TypeRecovery, data)
	if err != nil {
		return fmt.Errorf("failed to render recovery template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeRecovery,
		UserID:   userID,
	}

	return s.mailer.Send(context.Background(), msg)
}

// SendMagicLink sends a magic link email.
func (s *EmailService) SendMagicLink(email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=magiclink", s.config.SiteURL, token)

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "1 hour",
	}

	subject, html, text, err := s.templates.Render(TypeMagicLink, data)
	if err != nil {
		return fmt.Errorf("failed to render magic link template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeMagicLink,
	}

	return s.mailer.Send(context.Background(), msg)
}

// SendEmailChange sends an email change verification.
func (s *EmailService) SendEmailChange(userID, newEmail, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=email_change", s.config.SiteURL, token)

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           newEmail,
		Token:           token,
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := s.templates.Render(TypeEmailChange, data)
	if err != nil {
		return fmt.Errorf("failed to render email change template: %w", err)
	}

	msg := &Message{
		To:       newEmail,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeEmailChange,
		UserID:   userID,
	}

	return s.mailer.Send(context.Background(), msg)
}

// SendInvite sends an invitation email.
func (s *EmailService) SendInvite(email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=invite", s.config.SiteURL, token)

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "7 days",
	}

	subject, html, text, err := s.templates.Render(TypeInvite, data)
	if err != nil {
		return fmt.Errorf("failed to render invite template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeInvite,
	}

	return s.mailer.Send(context.Background(), msg)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -run TestEmailService -v`

Expected: PASS

**Step 5: Run all mail package tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/... -v`

Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/service.go internal/mail/service_test.go
git commit -m "feat(mail): implement EmailService for high-level email operations"
```

---

## Task 8: Implement Mail Viewer Web UI

**Files:**
- Create: `internal/mail/viewer/handler.go`
- Create: `internal/mail/viewer/static/index.html`
- Create: `internal/mail/viewer/handler_test.go`

**Step 1: Write the failing test**

Create `internal/mail/viewer/handler_test.go`:

```go
// internal/mail/viewer/handler_test.go
package viewer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/mail"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return database
}

func TestHandler_ListEmails(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	// Add a test email
	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyHTML: "<p>Test</p>",
		Type:     mail.TypeConfirmation,
	})

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/emails", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var emails []mail.CaughtEmail
	json.NewDecoder(w.Body).Decode(&emails)
	if len(emails) != 1 {
		t.Errorf("expected 1 email, got %d", len(emails))
	}
}

func TestHandler_GetEmail(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyHTML: "<p>Test body</p>",
		Type:     mail.TypeConfirmation,
	})

	emails, _ := catchMailer.ListEmails(1, 0)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/emails/"+emails[0].ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_DeleteEmail(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test",
		BodyHTML: "<p>Test</p>",
		Type:     mail.TypeConfirmation,
	})

	emails, _ := catchMailer.ListEmails(1, 0)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/api/emails/"+emails[0].ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify deleted
	remaining, _ := catchMailer.ListEmails(10, 0)
	if len(remaining) != 0 {
		t.Errorf("expected 0 emails, got %d", len(remaining))
	}
}

func TestHandler_ClearAll(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	for i := 0; i < 3; i++ {
		catchMailer.Send(nil, &mail.Message{
			To:       "test@example.com",
			From:     "noreply@example.com",
			Subject:  "Test",
			BodyHTML: "<p>Test</p>",
			Type:     mail.TypeConfirmation,
		})
	}

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/api/emails", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	remaining, _ := catchMailer.ListEmails(10, 0)
	if len(remaining) != 0 {
		t.Errorf("expected 0 emails, got %d", len(remaining))
	}
}

func TestHandler_ServeUI(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type text/html, got %s", ct)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/viewer/... -v`

Expected: FAIL (package doesn't exist)

**Step 3: Create static HTML file**

Create directory and file `internal/mail/viewer/static/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>sblite Mail Catcher</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        h1 { color: #333; font-size: 24px; }
        .actions { display: flex; gap: 10px; }
        button { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
        .btn-primary { background: #3b82f6; color: white; }
        .btn-danger { background: #ef4444; color: white; }
        .btn-secondary { background: #e5e7eb; color: #374151; }
        button:hover { opacity: 0.9; }
        .filters { margin-bottom: 15px; }
        select { padding: 8px 12px; border: 1px solid #ddd; border-radius: 4px; font-size: 14px; }
        table { width: 100%; background: white; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); border-collapse: collapse; }
        th, td { padding: 12px 16px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #f9fafb; font-weight: 600; color: #374151; }
        tr:hover { background: #f9fafb; cursor: pointer; }
        tr:last-child td { border-bottom: none; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 12px; font-weight: 500; }
        .badge-confirmation { background: #dbeafe; color: #1e40af; }
        .badge-recovery { background: #fef3c7; color: #92400e; }
        .badge-magic_link { background: #d1fae5; color: #065f46; }
        .badge-email_change { background: #ede9fe; color: #5b21b6; }
        .badge-invite { background: #fce7f3; color: #9d174d; }
        .time { color: #6b7280; font-size: 13px; }
        .empty { text-align: center; padding: 60px 20px; color: #6b7280; }
        .modal { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 100; }
        .modal.active { display: flex; align-items: center; justify-content: center; }
        .modal-content { background: white; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; display: flex; flex-direction: column; }
        .modal-header { padding: 16px 20px; border-bottom: 1px solid #eee; display: flex; justify-content: space-between; align-items: center; }
        .modal-header h2 { font-size: 18px; }
        .modal-body { padding: 20px; overflow-y: auto; flex: 1; }
        .modal-body iframe { width: 100%; height: 400px; border: 1px solid #eee; border-radius: 4px; }
        .close-btn { background: none; border: none; font-size: 24px; cursor: pointer; color: #6b7280; }
        .email-meta { display: grid; grid-template-columns: auto 1fr; gap: 8px 16px; margin-bottom: 20px; font-size: 14px; }
        .email-meta dt { color: #6b7280; }
        .email-meta dd { color: #111827; }
        .tabs { display: flex; gap: 10px; margin-bottom: 15px; }
        .tab { padding: 6px 12px; border: none; background: #e5e7eb; border-radius: 4px; cursor: pointer; }
        .tab.active { background: #3b82f6; color: white; }
        pre { background: #f3f4f6; padding: 15px; border-radius: 4px; overflow-x: auto; font-size: 13px; white-space: pre-wrap; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>sblite Mail Catcher</h1>
            <div class="actions">
                <button class="btn-secondary" onclick="refresh()">Refresh</button>
                <button class="btn-danger" onclick="clearAll()">Clear All</button>
            </div>
        </header>

        <div class="filters">
            <select id="typeFilter" onchange="filterEmails()">
                <option value="">All Types</option>
                <option value="confirmation">Confirmation</option>
                <option value="recovery">Recovery</option>
                <option value="magic_link">Magic Link</option>
                <option value="email_change">Email Change</option>
                <option value="invite">Invite</option>
            </select>
        </div>

        <table id="emailTable">
            <thead>
                <tr>
                    <th>Type</th>
                    <th>To</th>
                    <th>Subject</th>
                    <th>Time</th>
                    <th></th>
                </tr>
            </thead>
            <tbody id="emailList"></tbody>
        </table>

        <div id="emptyState" class="empty" style="display: none;">
            <p>No emails caught yet.</p>
            <p style="margin-top: 8px; font-size: 14px;">Emails will appear here when sent in catch mode.</p>
        </div>
    </div>

    <div id="modal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 id="modalSubject">Email Subject</h2>
                <button class="close-btn" onclick="closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <dl class="email-meta">
                    <dt>From:</dt><dd id="modalFrom"></dd>
                    <dt>To:</dt><dd id="modalTo"></dd>
                    <dt>Type:</dt><dd id="modalType"></dd>
                    <dt>Time:</dt><dd id="modalTime"></dd>
                </dl>
                <div class="tabs">
                    <button class="tab active" onclick="showTab('html')">HTML</button>
                    <button class="tab" onclick="showTab('text')">Plain Text</button>
                </div>
                <div id="htmlContent"><iframe id="htmlFrame" sandbox></iframe></div>
                <div id="textContent" style="display:none;"><pre id="textBody"></pre></div>
            </div>
        </div>
    </div>

    <script>
        let emails = [];
        let currentEmail = null;

        async function loadEmails() {
            try {
                const res = await fetch('/mail/api/emails');
                emails = await res.json() || [];
                renderEmails();
            } catch (e) {
                console.error('Failed to load emails:', e);
            }
        }

        function renderEmails() {
            const filter = document.getElementById('typeFilter').value;
            const filtered = filter ? emails.filter(e => e.type === filter) : emails;
            const tbody = document.getElementById('emailList');
            const empty = document.getElementById('emptyState');
            const table = document.getElementById('emailTable');

            if (filtered.length === 0) {
                table.style.display = 'none';
                empty.style.display = 'block';
                return;
            }

            table.style.display = 'table';
            empty.style.display = 'none';

            tbody.innerHTML = filtered.map(e => `
                <tr onclick="viewEmail('${e.id}')">
                    <td><span class="badge badge-${e.type}">${e.type}</span></td>
                    <td>${escapeHtml(e.to)}</td>
                    <td>${escapeHtml(e.subject)}</td>
                    <td class="time">${timeAgo(e.created_at)}</td>
                    <td><button class="btn-secondary" onclick="event.stopPropagation(); deleteEmail('${e.id}')">Delete</button></td>
                </tr>
            `).join('');
        }

        function filterEmails() {
            renderEmails();
        }

        async function viewEmail(id) {
            try {
                const res = await fetch(`/mail/api/emails/${id}`);
                currentEmail = await res.json();

                document.getElementById('modalSubject').textContent = currentEmail.subject;
                document.getElementById('modalFrom').textContent = currentEmail.from;
                document.getElementById('modalTo').textContent = currentEmail.to;
                document.getElementById('modalType').textContent = currentEmail.type;
                document.getElementById('modalTime').textContent = new Date(currentEmail.created_at).toLocaleString();

                const frame = document.getElementById('htmlFrame');
                frame.srcdoc = currentEmail.body_html || '<p>No HTML content</p>';
                document.getElementById('textBody').textContent = currentEmail.body_text || 'No plain text content';

                document.getElementById('modal').classList.add('active');
            } catch (e) {
                console.error('Failed to load email:', e);
            }
        }

        function closeModal() {
            document.getElementById('modal').classList.remove('active');
            currentEmail = null;
        }

        function showTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            event.target.classList.add('active');
            document.getElementById('htmlContent').style.display = tab === 'html' ? 'block' : 'none';
            document.getElementById('textContent').style.display = tab === 'text' ? 'block' : 'none';
        }

        async function deleteEmail(id) {
            if (!confirm('Delete this email?')) return;
            try {
                await fetch(`/mail/api/emails/${id}`, { method: 'DELETE' });
                loadEmails();
            } catch (e) {
                console.error('Failed to delete email:', e);
            }
        }

        async function clearAll() {
            if (!confirm('Delete all emails?')) return;
            try {
                await fetch('/mail/api/emails', { method: 'DELETE' });
                loadEmails();
            } catch (e) {
                console.error('Failed to clear emails:', e);
            }
        }

        function refresh() {
            loadEmails();
        }

        function timeAgo(dateStr) {
            const date = new Date(dateStr);
            const seconds = Math.floor((new Date() - date) / 1000);
            if (seconds < 60) return 'just now';
            if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
            if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
            return Math.floor(seconds / 86400) + 'd ago';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Close modal on escape key
        document.addEventListener('keydown', e => {
            if (e.key === 'Escape') closeModal();
        });

        // Close modal on backdrop click
        document.getElementById('modal').addEventListener('click', e => {
            if (e.target.id === 'modal') closeModal();
        });

        // Initial load
        loadEmails();

        // Auto-refresh every 5 seconds
        setInterval(loadEmails, 5000);
    </script>
</body>
</html>
```

**Step 4: Create the handler**

Create `internal/mail/viewer/handler.go`:

```go
// Package viewer provides a web UI for viewing caught emails.
package viewer

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/mail"
)

//go:embed static/index.html
var staticFS embed.FS

// Handler serves the mail viewer web UI.
type Handler struct {
	catcher *mail.CatchMailer
}

// NewHandler creates a new Handler.
func NewHandler(catcher *mail.CatchMailer) *Handler {
	return &Handler{catcher: catcher}
}

// RegisterRoutes registers the mail viewer routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.serveUI)
	r.Get("/api/emails", h.listEmails)
	r.Get("/api/emails/{id}", h.getEmail)
	r.Delete("/api/emails/{id}", h.deleteEmail)
	r.Delete("/api/emails", h.clearAll)
}

func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func (h *Handler) listEmails(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	emails, err := h.catcher.ListEmails(limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emails)
}

func (h *Handler) getEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	email, err := h.catcher.GetEmail(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(email)
}

func (h *Handler) deleteEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.catcher.DeleteEmail(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clearAll(w http.ResponseWriter, r *http.Request) {
	if err := h.catcher.ClearAll(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 5: Run test to verify it passes**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./internal/mail/viewer/... -v`

Expected: PASS

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/mail/viewer/
git commit -m "feat(mail): implement mail viewer web UI with embedded HTML"
```

---

## Task 9: Add CLI Flags and Server Integration

**Files:**
- Modify: `cmd/serve.go`
- Modify: `internal/server/server.go`

**Step 1: Update cmd/serve.go**

Modify `cmd/serve.go` to add mail configuration flags:

```go
// cmd/serve.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/mail"
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

		// Build mail configuration
		mailConfig := buildMailConfig(cmd)

		srv := server.New(database, jwtSecret, mailConfig)
		addr := fmt.Sprintf("%s:%d", host, port)
		fmt.Printf("Starting Supabase Lite on %s\n", addr)
		fmt.Printf("  Auth API: http://%s/auth/v1\n", addr)
		fmt.Printf("  REST API: http://%s/rest/v1\n", addr)
		if mailConfig.Mode == mail.ModeCatch {
			fmt.Printf("  Mail UI:  http://%s/mail\n", addr)
		}
		fmt.Printf("  Mail mode: %s\n", mailConfig.Mode)

		return srv.ListenAndServe(addr)
	},
}

func buildMailConfig(cmd *cobra.Command) *mail.Config {
	config := mail.DefaultConfig()

	// Environment variables take precedence
	if mode := os.Getenv("SBLITE_MAIL_MODE"); mode != "" {
		config.Mode = mode
	}
	if from := os.Getenv("SBLITE_MAIL_FROM"); from != "" {
		config.From = from
	}
	if siteURL := os.Getenv("SBLITE_SITE_URL"); siteURL != "" {
		config.SiteURL = siteURL
	}
	if smtpHost := os.Getenv("SBLITE_SMTP_HOST"); smtpHost != "" {
		config.SMTPHost = smtpHost
	}
	if smtpPort := os.Getenv("SBLITE_SMTP_PORT"); smtpPort != "" {
		fmt.Sscanf(smtpPort, "%d", &config.SMTPPort)
	}
	if smtpUser := os.Getenv("SBLITE_SMTP_USER"); smtpUser != "" {
		config.SMTPUser = smtpUser
	}
	if smtpPass := os.Getenv("SBLITE_SMTP_PASS"); smtpPass != "" {
		config.SMTPPass = smtpPass
	}

	// CLI flags override environment variables
	if mode, _ := cmd.Flags().GetString("mail-mode"); mode != "" {
		config.Mode = mode
	}
	if from, _ := cmd.Flags().GetString("mail-from"); from != "" {
		config.From = from
	}
	if siteURL, _ := cmd.Flags().GetString("site-url"); siteURL != "" {
		config.SiteURL = siteURL
	}

	return config
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().String("mail-mode", "", "Email mode: log, catch, or smtp (default: log)")
	serveCmd.Flags().String("mail-from", "", "Default sender email address")
	serveCmd.Flags().String("site-url", "", "Base URL for email links")
}
```

**Step 2: Update internal/server/server.go**

Modify `internal/server/server.go` to integrate mail services:

```go
// internal/server/server.go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/mail/viewer"
	"github.com/markb/sblite/internal/rest"
	"github.com/markb/sblite/internal/rls"
)

type Server struct {
	db           *db.DB
	router       *chi.Mux
	authService  *auth.Service
	rlsService   *rls.Service
	rlsEnforcer  *rls.Enforcer
	restHandler  *rest.Handler
	mailConfig   *mail.Config
	mailer       mail.Mailer
	catchMailer  *mail.CatchMailer
	emailService *mail.EmailService
}

func New(database *db.DB, jwtSecret string, mailConfig *mail.Config) *Server {
	rlsService := rls.NewService(database)
	rlsEnforcer := rls.NewEnforcer(rlsService)

	s := &Server{
		db:          database,
		router:      chi.NewRouter(),
		authService: auth.NewService(database, jwtSecret),
		rlsService:  rlsService,
		rlsEnforcer: rlsEnforcer,
		restHandler: rest.NewHandler(database, rlsEnforcer),
		mailConfig:  mailConfig,
	}

	// Initialize mail services
	s.initMail()

	s.setupRoutes()
	return s
}

func (s *Server) initMail() {
	templates := mail.NewTemplateService(s.db)

	switch s.mailConfig.Mode {
	case mail.ModeCatch:
		s.catchMailer = mail.NewCatchMailer(s.db)
		s.mailer = s.catchMailer
	case mail.ModeSMTP:
		smtpConfig := mail.SMTPConfig{
			Host: s.mailConfig.SMTPHost,
			Port: s.mailConfig.SMTPPort,
			User: s.mailConfig.SMTPUser,
			Pass: s.mailConfig.SMTPPass,
		}
		if err := smtpConfig.Validate(); err != nil {
			// Fall back to log mode if SMTP config is invalid
			fmt.Printf("Warning: Invalid SMTP config (%v), falling back to log mode\n", err)
			s.mailer = mail.NewLogMailer(nil)
			s.mailConfig.Mode = mail.ModeLog
		} else {
			s.mailer = mail.NewSMTPMailer(smtpConfig)
		}
	default:
		s.mailer = mail.NewLogMailer(nil)
	}

	s.emailService = mail.NewEmailService(s.mailer, templates, s.mailConfig)
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
		r.Post("/recover", s.handleRecover)
		r.Post("/verify", s.handleVerify)
		r.Get("/verify", s.handleVerify)
		r.Post("/magiclink", s.handleMagicLink)
		r.Post("/resend", s.handleResend)
		r.Get("/settings", s.handleSettings)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/user", s.handleGetUser)
			r.Put("/user", s.handleUpdateUser)
			r.Post("/logout", s.handleLogout)
			r.Post("/invite", s.handleInvite)
		})
	})

	// REST routes (with API key validation and optional auth for RLS)
	s.router.Route("/rest/v1", func(r chi.Router) {
		r.Use(s.apiKeyMiddleware)       // Validates apikey header, extracts role
		r.Use(s.optionalAuthMiddleware) // Extracts user JWT if present
		// OpenAPI schema endpoint (must be before /{table} to avoid conflict)
		r.Get("/", s.handleOpenAPI)
		r.Get("/{table}", s.restHandler.HandleSelect)
		r.Head("/{table}", s.restHandler.HandleSelect) // HEAD for count-only queries
		r.Post("/{table}", s.restHandler.HandleInsert)
		r.Patch("/{table}", s.restHandler.HandleUpdate)
		r.Delete("/{table}", s.restHandler.HandleDelete)
	})

	// Mail viewer (only in catch mode)
	if s.mailConfig.Mode == mail.ModeCatch && s.catchMailer != nil {
		viewerHandler := viewer.NewHandler(s.catchMailer)
		s.router.Route("/mail", func(r chi.Router) {
			viewerHandler.RegisterRoutes(r)
		})
	}
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

// handleOpenAPI generates and returns the OpenAPI 3.0 specification for the REST API.
func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec, err := rest.GenerateOpenAPISpec(s.db.DB)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "openapi_error",
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

// EmailService returns the email service for use by handlers.
func (s *Server) EmailService() *mail.EmailService {
	return s.emailService
}
```

**Step 3: Build to verify compilation**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go build ./...`

Expected: Build succeeds

**Step 4: Run all tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./... -v`

Expected: All tests PASS (may have some failures in server tests due to signature changes - fix in next step)

**Step 5: Fix server tests if needed**

Update any server tests that call `server.New()` to include the mail config parameter.

**Step 6: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add cmd/serve.go internal/server/server.go
git commit -m "feat(server): integrate mail services with CLI flags and server"
```

---

## Task 10: Add New Auth Handlers for Email Flows

**Files:**
- Modify: `internal/server/auth_handlers.go`

**Step 1: Add magic link handler**

Add to `internal/server/auth_handlers.go`:

```go
type MagicLinkRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate magic link token
	token, err := s.authService.GenerateMagicLinkToken(req.Email)
	if err != nil {
		// Don't reveal if user exists - always return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "If the email exists, a magic link has been sent",
		})
		return
	}

	// Send magic link email
	if err := s.emailService.SendMagicLink(req.Email, token); err != nil {
		// Log error but don't expose to user
		fmt.Printf("Failed to send magic link email: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email exists, a magic link has been sent",
	})
}
```

**Step 2: Add invite handler**

```go
type InviteRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	// Check if user has admin privileges (service_role)
	claims := GetClaimsFromContext(r)
	if claims == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	role, _ := (*claims)["role"].(string)
	if role != "service_role" {
		s.writeError(w, http.StatusForbidden, "forbidden", "Admin access required")
		return
	}

	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate invite token
	token, err := s.authService.GenerateInviteToken(req.Email)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "user_already_exists", "User already registered")
		return
	}

	// Send invite email
	if err := s.emailService.SendInvite(req.Email, token); err != nil {
		fmt.Printf("Failed to send invite email: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Invitation sent",
	})
}
```

**Step 3: Add resend handler**

```go
type ResendRequest struct {
	Type  string `json:"type"`  // confirmation, recovery, invite
	Email string `json:"email"`
}

func (s *Server) handleResend(w http.ResponseWriter, r *http.Request) {
	var req ResendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	switch req.Type {
	case "confirmation", "signup":
		user, err := s.authService.GetUserByEmail(req.Email)
		if err == nil && user.EmailConfirmedAt == nil {
			token, _ := s.authService.GenerateConfirmationToken(user.ID)
			s.emailService.SendConfirmation(user.ID, req.Email, token)
		}
	case "recovery":
		token, _ := s.authService.GenerateRecoveryToken(req.Email)
		if token != "" {
			user, _ := s.authService.GetUserByEmail(req.Email)
			if user != nil {
				s.emailService.SendRecovery(user.ID, req.Email, token)
			}
		}
	}

	// Always return success to prevent enumeration
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If applicable, an email has been sent",
	})
}
```

**Step 4: Update handleRecover to send email**

Update the existing `handleRecover` function:

```go
func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	var req RecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate token and send email
	token, err := s.authService.GenerateRecoveryToken(req.Email)
	if err == nil && token != "" {
		user, _ := s.authService.GetUserByEmail(req.Email)
		if user != nil {
			s.emailService.SendRecovery(user.ID, req.Email, token)
		}
	}

	// Always return success to prevent email enumeration
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email exists, a recovery link has been sent",
	})
}
```

**Step 5: Add import for fmt**

Ensure `"fmt"` is in the imports of `auth_handlers.go`.

**Step 6: Build to verify compilation**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go build ./...`

Expected: Build succeeds

**Step 7: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/server/auth_handlers.go
git commit -m "feat(auth): add magic link, invite, and resend handlers"
```

---

## Task 11: Add Auth Service Methods for New Token Types

**Files:**
- Modify: `internal/auth/user.go`
- Create: `internal/auth/tokens.go`

**Step 1: Create tokens.go with new token methods**

Create `internal/auth/tokens.go`:

```go
// internal/auth/tokens.go
package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TokenTypeConfirmation = "confirmation"
	TokenTypeRecovery     = "recovery"
	TokenTypeMagicLink    = "magiclink"
	TokenTypeEmailChange  = "email_change"
	TokenTypeInvite       = "invite"
)

// VerificationToken represents a token stored in auth_verification_tokens.
type VerificationToken struct {
	ID        string
	UserID    string
	Type      string
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreateVerificationToken creates a new verification token.
func (s *Service) CreateVerificationToken(userID, tokenType, email string, expiresIn time.Duration) (string, error) {
	token := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(expiresIn)

	_, err := s.db.Exec(`
		INSERT INTO auth_verification_tokens (id, user_id, type, email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token, userID, tokenType, email, expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		return "", fmt.Errorf("failed to create verification token: %w", err)
	}

	return token, nil
}

// ValidateVerificationToken checks if a token is valid (exists, not expired, not used).
func (s *Service) ValidateVerificationToken(token, expectedType string) (*VerificationToken, error) {
	var vt VerificationToken
	var expiresAt, createdAt string
	var usedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, user_id, type, email, expires_at, used_at, created_at
		FROM auth_verification_tokens WHERE id = ?
	`, token).Scan(&vt.ID, &vt.UserID, &vt.Type, &vt.Email, &expiresAt, &usedAt, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	vt.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	vt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if usedAt.Valid {
		t, _ := time.Parse(time.RFC3339, usedAt.String)
		vt.UsedAt = &t
	}

	// Check type
	if vt.Type != expectedType {
		return nil, fmt.Errorf("invalid token type")
	}

	// Check if already used
	if vt.UsedAt != nil {
		return nil, fmt.Errorf("token already used")
	}

	// Check expiration
	if time.Now().After(vt.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return &vt, nil
}

// MarkTokenUsed marks a verification token as used.
func (s *Service) MarkTokenUsed(token string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec("UPDATE auth_verification_tokens SET used_at = ? WHERE id = ?", now, token)
	return err
}

// GenerateMagicLinkToken creates a magic link token for the given email.
func (s *Service) GenerateMagicLinkToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.GetUserByEmail(email)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	return s.CreateVerificationToken(user.ID, TokenTypeMagicLink, email, 1*time.Hour)
}

// GenerateInviteToken creates an invite token for a new user.
func (s *Service) GenerateInviteToken(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check if user already exists
	_, err := s.GetUserByEmail(email)
	if err == nil {
		return "", fmt.Errorf("user already exists")
	}

	// Create a placeholder user ID (will be created when invite is accepted)
	placeholderID := uuid.New().String()

	return s.CreateVerificationToken(placeholderID, TokenTypeInvite, email, 7*24*time.Hour)
}

// VerifyMagicLink verifies a magic link token and returns a session.
func (s *Service) VerifyMagicLink(token string) (*User, *Session, string, error) {
	vt, err := s.ValidateVerificationToken(token, TokenTypeMagicLink)
	if err != nil {
		return nil, nil, "", err
	}

	user, err := s.GetUserByID(vt.UserID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("user not found")
	}

	// Mark token as used
	s.MarkTokenUsed(token)

	// Confirm email if not already confirmed
	if user.EmailConfirmedAt == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		s.db.Exec("UPDATE auth_users SET email_confirmed_at = ? WHERE id = ?", now, user.ID)
	}

	// Create session
	session, refreshToken, err := s.CreateSession(user)
	if err != nil {
		return nil, nil, "", err
	}

	return user, session, refreshToken, nil
}

// AcceptInvite accepts an invitation and creates a new user.
func (s *Service) AcceptInvite(token, password string) (*User, error) {
	vt, err := s.ValidateVerificationToken(token, TokenTypeInvite)
	if err != nil {
		return nil, err
	}

	// Create the user
	user, err := s.CreateUser(vt.Email, password, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Confirm email immediately since they came from invite
	now := time.Now().UTC().Format(time.RFC3339)
	s.db.Exec("UPDATE auth_users SET email_confirmed_at = ?, invited_at = ? WHERE id = ?", now, now, user.ID)

	// Mark token as used
	s.MarkTokenUsed(token)

	return s.GetUserByID(user.ID)
}
```

**Step 2: Build to verify compilation**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go build ./...`

Expected: Build succeeds

**Step 3: Commit**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add internal/auth/tokens.go
git commit -m "feat(auth): add verification token management for all email flows"
```

---

## Task 12: Final Integration and Testing

**Step 1: Run all tests**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go test ./... -v`

Expected: All tests PASS

**Step 2: Build the binary**

Run: `cd /Users/markb/dev/sblite/.worktrees/email-system && go build -o sblite .`

Expected: Binary builds successfully

**Step 3: Manual smoke test**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
rm -f test.db
./sblite init --db test.db
./sblite serve --db test.db --mail-mode=catch --port 8081
```

In another terminal:
```bash
# Test signup
curl -X POST http://localhost:8081/auth/v1/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Check mail viewer
open http://localhost:8081/mail
```

**Step 4: Commit final integration**

```bash
cd /Users/markb/dev/sblite/.worktrees/email-system
git add -A
git commit -m "test: verify email system integration"
```

---

## Summary

This plan implements:

1. **Database migrations** for email tables (auth_emails, auth_email_templates, auth_verification_tokens)
2. **Mail package** with Mailer interface and Message type
3. **LogMailer** for stdout output (default mode)
4. **CatchMailer** for database storage (development mode)
5. **SMTPMailer** for real email delivery (production mode)
6. **TemplateService** for database-stored templates with caching
7. **EmailService** for high-level send operations
8. **Mail viewer web UI** embedded in binary
9. **CLI flags** for mail configuration
10. **Auth handlers** for magic link, invite, resend
11. **Token management** in auth service

Total: ~1000 lines of Go code, 12 commits
