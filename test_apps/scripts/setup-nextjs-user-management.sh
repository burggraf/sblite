#!/bin/bash
# setup-nextjs-user-management.sh - Setup the Next.js User Management example

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_NAME="nextjs-user-management"
APP_DIR="$TEST_APPS_DIR/$APP_NAME"

log_info "Setting up $APP_NAME..."

# Check requirements
check_requirements

# Ensure sblite is built
ensure_sblite

# Clone the example
clone_supabase_example "examples/user-management/nextjs-user-management" "$APP_NAME"

# Generate API keys
generate_keys

# Create .env.local
create_nextjs_env "$APP_DIR"

# Install dependencies
install_deps "$APP_DIR"

# Initialize database
DB_PATH="$APP_DIR/data.db"
init_database "$DB_PATH"

# Create the profiles table and storage schema
log_info "Creating profiles table and storage..."
sqlite3 "$DB_PATH" << 'EOF'
-- Create profiles table (sblite-compatible version)
CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    updated_at TEXT,
    username TEXT UNIQUE,
    full_name TEXT,
    avatar_url TEXT,
    website TEXT,
    CONSTRAINT username_length CHECK (length(username) >= 3 OR username IS NULL)
);

-- Register column types in _columns for type tracking
INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type) VALUES
    ('profiles', 'id', 'uuid'),
    ('profiles', 'updated_at', 'timestamptz'),
    ('profiles', 'username', 'text'),
    ('profiles', 'full_name', 'text'),
    ('profiles', 'avatar_url', 'text'),
    ('profiles', 'website', 'text');

-- Enable RLS on profiles table
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('profiles', 1);

-- Create RLS policies for profiles
INSERT OR REPLACE INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('profiles', 'Public profiles are viewable by everyone', 'SELECT', 'true', NULL),
    ('profiles', 'Users can insert their own profile', 'INSERT', NULL, 'auth.uid() = id'),
    ('profiles', 'Users can update own profile', 'UPDATE', 'auth.uid() = id', NULL);

-- Create avatars storage bucket
INSERT OR IGNORE INTO storage_buckets (id, name, public, file_size_limit, allowed_mime_types, created_at, updated_at)
VALUES ('avatars', 'avatars', 1, 5242880, '["image/jpeg","image/png","image/gif","image/webp"]', datetime('now'), datetime('now'));
EOF

log_success "Database schema created"

# Create startup script
create_startup_script "$APP_NAME" "$APP_DIR" "data" "npm run dev"

log_success "Setup complete!"
echo ""
echo "To start the app, run:"
echo "  $SCRIPTS_DIR/start-${APP_NAME}.sh"
echo ""
echo "Then open http://localhost:3000 in your browser"
echo "sblite API will be running at $SBLITE_URL"
echo ""
echo "NOTE: Profile creation on signup is not automatic in sblite."
echo "Users will need to create their profile after signing up."
