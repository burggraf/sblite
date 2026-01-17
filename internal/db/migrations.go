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

// RLS policies table for storing row-level security policies
const rlsSchema = `
CREATE TABLE IF NOT EXISTS _rls_policies (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name    TEXT NOT NULL,
    policy_name   TEXT NOT NULL,
    command       TEXT CHECK (command IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE', 'ALL')),
    using_expr    TEXT,
    check_expr    TEXT,
    enabled       INTEGER DEFAULT 1,
    created_at    TEXT DEFAULT (datetime('now')),
    UNIQUE(table_name, policy_name)
);
`

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
'Confirm your email

Click the link below to confirm your email address:
{{.ConfirmationURL}}

This link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-recovery', 'recovery', 'Reset your password',
'<h2>Reset your password</h2><p>Click the link below to reset your password:</p><p><a href="{{.ConfirmationURL}}">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Reset your password

Click the link below to reset your password:
{{.ConfirmationURL}}

This link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-magic_link', 'magic_link', 'Your login link',
'<h2>Your login link</h2><p>Click the link below to sign in:</p><p><a href="{{.ConfirmationURL}}">Sign In</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Your login link

Click the link below to sign in:
{{.ConfirmationURL}}

This link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-email_change', 'email_change', 'Confirm email change',
'<h2>Confirm your new email</h2><p>Click the link below to confirm your new email address:</p><p><a href="{{.ConfirmationURL}}">Confirm New Email</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'Confirm your new email

Click the link below to confirm your new email address:
{{.ConfirmationURL}}

This link expires in {{.ExpiresIn}}.',
datetime('now')),

('tpl-invite', 'invite', 'You have been invited',
'<h2>You have been invited</h2><p>Click the link below to accept your invitation and set your password:</p><p><a href="{{.ConfirmationURL}}">Accept Invitation</a></p><p>This link expires in {{.ExpiresIn}}.</p>',
'You have been invited

Click the link below to accept your invitation and set your password:
{{.ConfirmationURL}}

This link expires in {{.ExpiresIn}}.',
datetime('now'));
`

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
