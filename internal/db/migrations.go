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
