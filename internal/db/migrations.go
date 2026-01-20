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
    is_anonymous          INTEGER DEFAULT 0,
    role                  TEXT DEFAULT 'authenticated',
    created_at            TEXT DEFAULT (datetime('now')),
    updated_at            TEXT DEFAULT (datetime('now')),
    banned_until          TEXT,
    deleted_at            TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email);
CREATE INDEX IF NOT EXISTS idx_auth_users_confirmation_token ON auth_users(confirmation_token);
CREATE INDEX IF NOT EXISTS idx_auth_users_recovery_token ON auth_users(recovery_token);
CREATE INDEX IF NOT EXISTS idx_auth_users_is_anonymous ON auth_users(is_anonymous);

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

CREATE TABLE IF NOT EXISTS _rls_tables (
    table_name    TEXT PRIMARY KEY,
    enabled       INTEGER DEFAULT 0
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

const columnsSchema = `
CREATE TABLE IF NOT EXISTS _columns (
    table_name    TEXT NOT NULL,
    column_name   TEXT NOT NULL,
    pg_type       TEXT NOT NULL CHECK (pg_type IN (
                    'uuid', 'text', 'integer', 'numeric',
                    'boolean', 'timestamptz', 'jsonb', 'bytea'
                  )),
    is_nullable   INTEGER DEFAULT 1,
    default_value TEXT,
    is_primary    INTEGER DEFAULT 0,
    description   TEXT DEFAULT '',
    created_at    TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (table_name, column_name)
);

CREATE INDEX IF NOT EXISTS idx_columns_table ON _columns(table_name);
`

// API Docs schema for storing table and function descriptions
const apiDocsSchema = `
CREATE TABLE IF NOT EXISTS _table_descriptions (
    table_name    TEXT PRIMARY KEY,
    description   TEXT DEFAULT '',
    updated_at    TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS _function_descriptions (
    function_name TEXT PRIMARY KEY,
    description   TEXT DEFAULT '',
    updated_at    TEXT DEFAULT (datetime('now'))
);
`

const schemaMigrationsSchema = `
CREATE TABLE IF NOT EXISTS _schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

const dashboardSchema = `
CREATE TABLE IF NOT EXISTS _dashboard (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);
`

// OAuth: auth_identities table stores OAuth provider accounts linked to users
const oauthIdentitiesSchema = `
CREATE TABLE IF NOT EXISTS auth_identities (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	provider TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	identity_data TEXT,
	last_sign_in_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_identities_user ON auth_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_identities_provider ON auth_identities(provider, provider_id);
`

// OAuth: auth_flow_state table stores PKCE state during OAuth flow (temporary, 10-minute expiry)
const oauthFlowStateSchema = `
CREATE TABLE IF NOT EXISTS auth_flow_state (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	code_verifier TEXT NOT NULL,
	redirect_to TEXT,
	linking_user_id TEXT,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
`

// FTS: _fts_indexes table tracks full-text search indexes
const ftsSchema = `
CREATE TABLE IF NOT EXISTS _fts_indexes (
    table_name    TEXT NOT NULL,
    index_name    TEXT NOT NULL,
    columns       TEXT NOT NULL,
    tokenizer     TEXT DEFAULT 'unicode61',
    created_at    TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (table_name, index_name)
);

CREATE INDEX IF NOT EXISTS idx_fts_indexes_table ON _fts_indexes(table_name);
`

// Edge Functions schema for function configuration, secrets, and metadata
const functionsSchema = `
-- Functions configuration (key-value store)
CREATE TABLE IF NOT EXISTS _functions_config (
    key           TEXT PRIMARY KEY,
    value         TEXT NOT NULL,
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Functions secrets (encrypted with JWT secret)
CREATE TABLE IF NOT EXISTS _functions_secrets (
    name          TEXT PRIMARY KEY,
    value         TEXT NOT NULL,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);

-- Per-function metadata and configuration
CREATE TABLE IF NOT EXISTS _functions_metadata (
    name          TEXT PRIMARY KEY,
    verify_jwt    INTEGER DEFAULT 1,
    memory_mb     INTEGER,
    timeout_ms    INTEGER,
    import_map    TEXT,
    env_vars      TEXT DEFAULT '{}' CHECK (json_valid(env_vars)),
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);
`

// RPC functions schema for PostgreSQL-compatible stored functions
const rpcFunctionsSchema = `
-- Function definitions
CREATE TABLE IF NOT EXISTS _rpc_functions (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    language TEXT NOT NULL DEFAULT 'sql' CHECK (language IN ('sql')),
    return_type TEXT NOT NULL,
    returns_set INTEGER NOT NULL DEFAULT 0,
    volatility TEXT DEFAULT 'VOLATILE' CHECK (volatility IN ('VOLATILE', 'STABLE', 'IMMUTABLE')),
    security TEXT DEFAULT 'INVOKER' CHECK (security IN ('INVOKER', 'DEFINER')),
    source_pg TEXT NOT NULL,
    source_sqlite TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Function arguments
CREATE TABLE IF NOT EXISTS _rpc_function_args (
    id TEXT PRIMARY KEY,
    function_id TEXT NOT NULL REFERENCES _rpc_functions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    position INTEGER NOT NULL,
    default_value TEXT,
    UNIQUE(function_id, position)
);

CREATE INDEX IF NOT EXISTS idx_rpc_function_args_function ON _rpc_function_args(function_id);
`

// Storage schema for Supabase-compatible file storage
// Based on https://supabase.com/docs/guides/storage/schema/design
const storageSchema = `
-- Buckets table: containers for organizing files
CREATE TABLE IF NOT EXISTS storage_buckets (
    id            TEXT PRIMARY KEY,
    name          TEXT UNIQUE NOT NULL,
    owner         TEXT,
    owner_id      TEXT,
    public        INTEGER DEFAULT 0,
    file_size_limit   INTEGER,
    allowed_mime_types TEXT,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_storage_buckets_name ON storage_buckets(name);

-- Objects table: file metadata (actual files stored on filesystem or S3)
CREATE TABLE IF NOT EXISTS storage_objects (
    id            TEXT PRIMARY KEY,
    bucket_id     TEXT NOT NULL REFERENCES storage_buckets(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    owner         TEXT,
    owner_id      TEXT,
    metadata      TEXT DEFAULT '{}' CHECK (json_valid(metadata)),
    path_tokens   TEXT DEFAULT '[]' CHECK (json_valid(path_tokens)),
    user_metadata TEXT DEFAULT '{}' CHECK (json_valid(user_metadata)),
    version       TEXT,
    size          INTEGER,
    mime_type     TEXT,
    etag          TEXT,
    last_accessed_at TEXT,
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now')),
    UNIQUE(bucket_id, name)
);

CREATE INDEX IF NOT EXISTS idx_storage_objects_bucket ON storage_objects(bucket_id);
CREATE INDEX IF NOT EXISTS idx_storage_objects_name ON storage_objects(name);
CREATE INDEX IF NOT EXISTS idx_storage_objects_owner ON storage_objects(owner_id);
CREATE INDEX IF NOT EXISTS idx_storage_objects_bucket_name ON storage_objects(bucket_id, name);
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
	// First, check if auth_users exists and add is_anonymous column if missing
	// This handles migration from older databases that don't have this column
	var hasIsAnonymous int
	row := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('auth_users')
		WHERE name = 'is_anonymous'
	`)
	if err := row.Scan(&hasIsAnonymous); err == nil && hasIsAnonymous == 0 {
		// Table exists but column doesn't - add it
		var tableExists int
		row := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_users'`)
		if err := row.Scan(&tableExists); err == nil && tableExists > 0 {
			_, _ = db.Exec(`ALTER TABLE auth_users ADD COLUMN is_anonymous INTEGER DEFAULT 0`)
		}
	}

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

	_, err = db.Exec(columnsSchema)
	if err != nil {
		return fmt.Errorf("failed to run columns schema migration: %w", err)
	}

	_, err = db.Exec(schemaMigrationsSchema)
	if err != nil {
		return fmt.Errorf("failed to run schema migrations table creation: %w", err)
	}

	_, err = db.Exec(dashboardSchema)
	if err != nil {
		return fmt.Errorf("failed to run dashboard schema migration: %w", err)
	}

	_, err = db.Exec(oauthIdentitiesSchema)
	if err != nil {
		return fmt.Errorf("failed to run OAuth identities schema migration: %w", err)
	}

	_, err = db.Exec(oauthFlowStateSchema)
	if err != nil {
		return fmt.Errorf("failed to run OAuth flow state schema migration: %w", err)
	}

	_, err = db.Exec(ftsSchema)
	if err != nil {
		return fmt.Errorf("failed to run FTS schema migration: %w", err)
	}

	_, err = db.Exec(storageSchema)
	if err != nil {
		return fmt.Errorf("failed to run storage schema migration: %w", err)
	}

	// Enable RLS by default on storage_objects table (matches Supabase behavior)
	_, err = db.Exec(`INSERT OR IGNORE INTO _rls_tables (table_name, enabled) VALUES ('storage_objects', 1)`)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on storage_objects: %w", err)
	}

	_, err = db.Exec(functionsSchema)
	if err != nil {
		return fmt.Errorf("failed to run functions schema migration: %w", err)
	}

	_, err = db.Exec(rpcFunctionsSchema)
	if err != nil {
		return fmt.Errorf("failed to run RPC functions schema migration: %w", err)
	}

	// Add description column to _columns if it doesn't exist (for existing databases)
	var hasDescription int
	row = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('_columns')
		WHERE name = 'description'
	`)
	if err := row.Scan(&hasDescription); err == nil && hasDescription == 0 {
		// Table exists but column doesn't - add it
		var tableExists int
		row := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_columns'`)
		if err := row.Scan(&tableExists); err == nil && tableExists > 0 {
			_, _ = db.Exec(`ALTER TABLE _columns ADD COLUMN description TEXT DEFAULT ''`)
		}
	}

	_, err = db.Exec(apiDocsSchema)
	if err != nil {
		return fmt.Errorf("failed to run API docs schema migration: %w", err)
	}

	return nil
}
