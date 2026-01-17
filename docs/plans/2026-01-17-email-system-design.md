# Email System Design

## Overview

Implement a complete email system for sblite with full Supabase parity, supporting email confirmation, password reset, magic links, email change verification, and user invitations.

## Requirements

| Decision | Choice |
|----------|--------|
| Email use cases | Full Supabase parity (confirmation, reset, magic links, email change, invites) |
| Mail viewer | Embedded web UI at `/mail` |
| SMTP scope | Internal only (sblite's emails only) |
| Templates | Database-stored, API manageable |
| Link format | Supabase-compatible tokens |
| Default mode | Log mode (print to stdout) |

## Architecture

### Three-Layer Design

**1. Mailer Interface** (`internal/mail/mailer.go`)

```go
type Mailer interface {
    Send(ctx context.Context, msg *Message) error
}

type Message struct {
    To       string
    From     string
    Subject  string
    BodyHTML string
    BodyText string
    Type     string // confirmation, recovery, magic_link, email_change, invite
    UserID   string
    Metadata map[string]any
}
```

**2. Three Implementations:**

- `LogMailer` - Prints formatted email to stdout (default)
- `CatchMailer` - Stores in SQLite `auth_emails` table
- `SMTPMailer` - Relays to configured SMTP server

**3. Email Service** (`internal/mail/service.go`)

High-level functions that auth handlers call:

- `SendConfirmation(user)`
- `SendPasswordReset(user)`
- `SendMagicLink(email)`
- `SendEmailChange(user, newEmail)`
- `SendInvite(email, inviterID)`

The service loads templates from the database, renders them with user data, generates tokens, and calls the configured Mailer.

## Database Schema

### `auth_emails` - Caught emails (for catch mode)

```sql
CREATE TABLE auth_emails (
    id TEXT PRIMARY KEY,           -- UUID
    to_email TEXT NOT NULL,
    from_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT,
    body_text TEXT,
    email_type TEXT NOT NULL,      -- confirmation, recovery, magic_link, email_change, invite
    user_id TEXT,                  -- FK to auth_users (nullable for invites)
    created_at TEXT NOT NULL,
    metadata TEXT                  -- JSON for extra data
);
```

### `auth_email_templates` - Customizable templates

```sql
CREATE TABLE auth_email_templates (
    id TEXT PRIMARY KEY,
    type TEXT UNIQUE NOT NULL,     -- confirmation, recovery, magic_link, email_change, invite
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    updated_at TEXT NOT NULL
);
```

### `auth_verification_tokens` - For confirmation/reset links

```sql
CREATE TABLE auth_verification_tokens (
    id TEXT PRIMARY KEY,           -- UUID (the token itself)
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,            -- confirmation, recovery, email_change, invite
    email TEXT NOT NULL,           -- Target email (for email_change, this is the new email)
    expires_at TEXT NOT NULL,
    used_at TEXT,                  -- NULL until used
    created_at TEXT NOT NULL
);
```

Templates are seeded with defaults on migration. Tokens expire (configurable, default 24h for confirmation, 1h for reset).

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SBLITE_MAIL_MODE` | `log` | `log`, `catch`, or `smtp` |
| `SBLITE_SMTP_HOST` | - | SMTP server hostname |
| `SBLITE_SMTP_PORT` | `587` | SMTP port |
| `SBLITE_SMTP_USER` | - | SMTP username |
| `SBLITE_SMTP_PASS` | - | SMTP password |
| `SBLITE_MAIL_FROM` | `noreply@localhost` | Default sender address |
| `SBLITE_SITE_URL` | `http://localhost:8080` | Base URL for email links |

### CLI Flags

```bash
sblite serve --mail-mode=catch
sblite serve --mail-from=noreply@example.com
sblite serve --site-url=https://myapp.com
```

### Mode Behavior

| Mode | Sends email? | Stores in DB? | Web UI? |
|------|--------------|---------------|---------|
| `log` | No (prints to stdout) | No | No |
| `catch` | No | Yes | Yes at `/mail` |
| `smtp` | Yes (real delivery) | No | No |

If `SBLITE_MAIL_MODE=smtp` but SMTP credentials are missing, startup fails with a clear error.

## Auth Endpoints

### New Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/v1/verify` | GET | Verify token from email link (confirmation, recovery, etc.) |
| `/auth/v1/verify` | POST | Verify OTP code (alternative to link) |
| `/auth/v1/recover` | POST | Request password reset email |
| `/auth/v1/magiclink` | POST | Request magic link email |
| `/auth/v1/invite` | POST | Invite user by email (requires auth + admin) |
| `/auth/v1/resend` | POST | Resend confirmation/invite email |

### Modified Endpoints

| Endpoint | Change |
|----------|--------|
| `/auth/v1/signup` | Add `email_confirm` option; if enabled, sends confirmation email |
| `/auth/v1/user` (PUT) | When email changes, sends verification to new email |

### Verification Flow

1. User clicks link: `GET /auth/v1/verify?token=xxx&type=signup`
2. Server validates token exists, not expired, not used
3. Marks token as used, updates user (e.g., `email_confirmed_at`)
4. Redirects to `SBLITE_SITE_URL` with success params (or error params if failed)

Supabase redirects to a configurable URL with hash fragments containing the session. We'll match this behavior.

## Mail Viewer Web UI

### Route

`GET /mail` (only active in `catch` mode)

### Features

- List view of all caught emails, newest first
- Click to view full email (HTML rendered or plain text)
- Filter by type (confirmation, recovery, etc.)
- Delete individual emails or clear all
- Auto-refresh option for dev convenience

### Implementation

- Single HTML page with embedded CSS/JS using `go:embed`
- Vanilla JS (no framework) - keeps binary small
- API endpoints under `/mail/api/*`:
  - `GET /mail/api/emails` - List emails (with pagination)
  - `GET /mail/api/emails/:id` - Get single email
  - `DELETE /mail/api/emails/:id` - Delete email
  - `DELETE /mail/api/emails` - Clear all

### UI Mockup

```
┌─────────────────────────────────────────────────────────┐
│  sblite Mail Catcher                    [Clear All]     │
├─────────────────────────────────────────────────────────┤
│  Filter: [All ▼]                                        │
├──────────┬─────────────────────┬───────────────┬────────┤
│  Type    │  To                 │  Subject      │  Time  │
├──────────┼─────────────────────┼───────────────┼────────┤
│  confirm │  user@example.com   │  Confirm your │  2m ago│
│  reset   │  other@example.com  │  Reset your   │  5m ago│
└──────────┴─────────────────────┴───────────────┴────────┘
```

## Email Templates

### Template Variables

All templates support:

- `{{.SiteURL}}` - Base URL of the application
- `{{.ConfirmationURL}}` - Full link with token
- `{{.Email}}` - User's email address
- `{{.Token}}` - Raw token (for OTP display)
- `{{.ExpiresIn}}` - Human-readable expiry ("24 hours")

### Default Templates

| Type | Subject | Key content |
|------|---------|-------------|
| `confirmation` | Confirm your email | "Click to confirm your account" |
| `recovery` | Reset your password | "Click to reset your password" |
| `magic_link` | Your login link | "Click to sign in" |
| `email_change` | Confirm email change | "Click to confirm your new email" |
| `invite` | You've been invited | "Click to accept invitation and set password" |

### Template Engine

- Use Go's `html/template` for HTML body
- Use `text/template` for plain text body
- Templates stored as raw strings in DB, parsed at send time
- Cache parsed templates in memory (invalidate on update)

### Admin API

- `GET /auth/v1/admin/templates` - List all templates
- `GET /auth/v1/admin/templates/:type` - Get one template
- `PUT /auth/v1/admin/templates/:type` - Update template

Admin endpoints require `service_role` API key.

## File Structure

### New Files

```
internal/
├── mail/
│   ├── mailer.go          # Mailer interface + Message type
│   ├── log_mailer.go      # LogMailer implementation
│   ├── catch_mailer.go    # CatchMailer implementation
│   ├── smtp_mailer.go     # SMTPMailer implementation
│   ├── service.go         # High-level send functions
│   ├── templates.go       # Template loading/rendering/caching
│   └── viewer/
│       ├── handler.go     # /mail routes
│       └── static/        # Embedded HTML/CSS/JS
│           └── index.html
```

### Modified Files

| File | Changes |
|------|---------|
| `internal/db/migrations.go` | Add 3 new tables + seed templates |
| `internal/server/server.go` | Initialize mailer, register `/mail` routes |
| `internal/server/auth_handlers.go` | Add verify, recover, magiclink, invite, resend endpoints |
| `cmd/serve.go` | Add mail-mode, mail-from, site-url flags |

## Estimated Scope

- ~800-1000 lines of new Go code
- 3 new database tables
- 6 new auth endpoints + 2 modified
- 1 web UI (embedded HTML/JS)
- Full test coverage for mail package
