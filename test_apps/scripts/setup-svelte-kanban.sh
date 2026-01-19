#!/bin/bash
# setup-svelte-kanban.sh - Setup the Svelte Kanban (Trello clone) example
# https://github.com/supabase-community/svelte-kanban

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_NAME="svelte-kanban"
APP_DIR="$TEST_APPS_DIR/$APP_NAME"

log_info "Setting up $APP_NAME..."

# Check requirements
check_requirements

# Ensure sblite is built
ensure_sblite

# Clone from GitHub if not exists
if [[ ! -d "$APP_DIR" ]]; then
    log_info "Cloning Svelte Kanban from GitHub..."
    git clone --depth 1 https://github.com/supabase-community/svelte-kanban.git "$APP_DIR"
    log_success "Cloned to $APP_DIR"
else
    log_info "App directory already exists, skipping clone"
fi

# Generate API keys
generate_keys

# Create .env file for SvelteKit app
log_info "Creating .env file..."
cat > "$APP_DIR/.env" << EOF
# sblite configuration
PUBLIC_SUPABASE_URL=$SBLITE_URL
PUBLIC_SUPABASE_KEY=$ANON_KEY
EOF
log_success "Created $APP_DIR/.env"

# Install dependencies
install_deps "$APP_DIR"

# Initialize database if it doesn't exist
DB_PATH="$APP_DIR/data.db"
if [[ ! -f "$DB_PATH" ]]; then
    init_database "$DB_PATH"
else
    log_info "Database already exists, skipping init"
fi

# Create the schema (adapted for SQLite from setup.sql)
log_info "Creating Svelte Kanban schema..."
sqlite3 "$DB_PATH" << 'EOF'
-- Create boards table
CREATE TABLE IF NOT EXISTS boards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    title TEXT,
    position INTEGER,
    inserted_at TEXT DEFAULT (datetime('now'))
);

-- Create lists table
CREATE TABLE IF NOT EXISTS lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    board_id INTEGER REFERENCES boards(id) ON DELETE CASCADE,
    title TEXT,
    position INTEGER,
    inserted_at TEXT DEFAULT (datetime('now'))
);

-- Create cards table
CREATE TABLE IF NOT EXISTS cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    list_id INTEGER REFERENCES lists(id) ON DELETE CASCADE,
    position INTEGER,
    description TEXT CHECK (length(description) > 0),
    completed_at TEXT,
    inserted_at TEXT DEFAULT (datetime('now'))
);

-- Register column types in _columns for type tracking
INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type) VALUES
    ('boards', 'id', 'integer'),
    ('boards', 'user_id', 'uuid'),
    ('boards', 'title', 'text'),
    ('boards', 'position', 'integer'),
    ('boards', 'inserted_at', 'timestamptz'),
    ('lists', 'id', 'integer'),
    ('lists', 'user_id', 'uuid'),
    ('lists', 'board_id', 'integer'),
    ('lists', 'title', 'text'),
    ('lists', 'position', 'integer'),
    ('lists', 'inserted_at', 'timestamptz'),
    ('cards', 'id', 'integer'),
    ('cards', 'user_id', 'uuid'),
    ('cards', 'list_id', 'integer'),
    ('cards', 'position', 'integer'),
    ('cards', 'description', 'text'),
    ('cards', 'completed_at', 'timestamptz'),
    ('cards', 'inserted_at', 'timestamptz');

-- Enable RLS on tables
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES
    ('boards', 1),
    ('lists', 1),
    ('cards', 1);

-- Create RLS policies (user can only access their own data)
INSERT OR REPLACE INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('boards', 'Users can view own boards', 'SELECT', 'auth.uid() = user_id', NULL),
    ('boards', 'Users can insert own boards', 'INSERT', NULL, 'auth.uid() = user_id'),
    ('boards', 'Users can update own boards', 'UPDATE', 'auth.uid() = user_id', NULL),
    ('boards', 'Users can delete own boards', 'DELETE', 'auth.uid() = user_id', NULL),
    ('lists', 'Users can view own lists', 'SELECT', 'auth.uid() = user_id', NULL),
    ('lists', 'Users can insert own lists', 'INSERT', NULL, 'auth.uid() = user_id'),
    ('lists', 'Users can update own lists', 'UPDATE', 'auth.uid() = user_id', NULL),
    ('lists', 'Users can delete own lists', 'DELETE', 'auth.uid() = user_id', NULL),
    ('cards', 'Users can view own cards', 'SELECT', 'auth.uid() = user_id', NULL),
    ('cards', 'Users can insert own cards', 'INSERT', NULL, 'auth.uid() = user_id'),
    ('cards', 'Users can update own cards', 'UPDATE', 'auth.uid() = user_id', NULL),
    ('cards', 'Users can delete own cards', 'DELETE', 'auth.uid() = user_id', NULL);
EOF

log_success "Database schema created"

# Create startup script
create_startup_script "$APP_NAME" "$APP_DIR" "data" "pnpm run dev"

log_success "Setup complete!"
echo ""
echo "To start the app, run:"
echo "  $SCRIPTS_DIR/start-${APP_NAME}.sh"
echo ""
echo "Then open http://localhost:5173 in your browser"
echo "sblite API will be running at $SBLITE_URL"
echo ""
echo "Note: The original app uses PostgreSQL functions (sort_board, sort_list)"
echo "for drag-and-drop reordering. These are not supported in sblite, so"
echo "reordering may not persist correctly."
