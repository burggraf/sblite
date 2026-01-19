#!/bin/bash
# setup-svelte-todo.sh - Setup the SvelteJS Todo List example

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_NAME="sveltejs-todo-list"
APP_DIR="$TEST_APPS_DIR/$APP_NAME"

log_info "Setting up $APP_NAME..."

# Check requirements
check_requirements

# Ensure sblite is built
ensure_sblite

# Clone the example
clone_supabase_example "examples/todo-list/sveltejs-todo-list" "$APP_NAME"

# Generate API keys
generate_keys

# Create .env file for Svelte (uses Vite)
create_svelte_env "$APP_DIR"

# Install dependencies
install_deps "$APP_DIR"

# Initialize database
DB_PATH="$APP_DIR/data.db"
if [[ ! -f "$DB_PATH" ]]; then
    init_database "$DB_PATH"
else
    log_info "Database already exists, skipping init"
fi

# Create the todos table schema (same as Next.js version)
log_info "Creating todos table..."
sqlite3 "$DB_PATH" << 'EOF'
-- Create todos table (sblite-compatible version)
CREATE TABLE IF NOT EXISTS todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    task TEXT CHECK (length(task) > 3),
    is_complete INTEGER DEFAULT 0,
    inserted_at TEXT DEFAULT (datetime('now'))
);

-- Register column types in _columns for type tracking
INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type) VALUES
    ('todos', 'id', 'integer'),
    ('todos', 'user_id', 'uuid'),
    ('todos', 'task', 'text'),
    ('todos', 'is_complete', 'boolean'),
    ('todos', 'inserted_at', 'timestamptz');

-- Enable RLS on todos table
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('todos', 1);

-- Create RLS policies for todos
INSERT OR REPLACE INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('todos', 'Individuals can create todos', 'INSERT', NULL, 'auth.uid() = user_id'),
    ('todos', 'Individuals can view their own todos', 'SELECT', 'auth.uid() = user_id', NULL),
    ('todos', 'Individuals can update their own todos', 'UPDATE', 'auth.uid() = user_id', NULL),
    ('todos', 'Individuals can delete their own todos', 'DELETE', 'auth.uid() = user_id', NULL);
EOF

log_success "Database schema created"

# Create startup script
create_startup_script "$APP_NAME" "$APP_DIR" "data" "npm run dev"

log_success "Setup complete!"
echo ""
echo "To start the app, run:"
echo "  $SCRIPTS_DIR/start-${APP_NAME}.sh"
echo ""
echo "Then open http://localhost:5173 in your browser"
echo "sblite API will be running at $SBLITE_URL"
