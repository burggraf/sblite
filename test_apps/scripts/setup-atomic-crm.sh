#!/bin/bash
# setup-atomic-crm.sh - Setup the Atomic CRM example
# https://github.com/marmelab/atomic-crm

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_NAME="atomic-crm"
APP_DIR="$TEST_APPS_DIR/$APP_NAME"

log_info "Setting up $APP_NAME..."

# Check requirements
check_requirements

# Ensure sblite is built
ensure_sblite

# Clone from GitHub if not exists
if [[ ! -d "$APP_DIR" ]]; then
    log_info "Cloning Atomic CRM from GitHub..."
    git clone --depth 1 https://github.com/marmelab/atomic-crm.git "$APP_DIR"
    log_success "Cloned to $APP_DIR"
else
    log_info "App directory already exists, skipping clone"
fi

# Generate API keys
generate_keys

# Create .env file for Vite app
log_info "Creating .env file..."
cat > "$APP_DIR/.env" << EOF
# sblite configuration
VITE_SUPABASE_URL=$SBLITE_URL
VITE_SUPABASE_ANON_KEY=$ANON_KEY
EOF
log_success "Created $APP_DIR/.env"

# Create .npmrc for pnpm compatibility (this project has many peer dependencies)
log_info "Creating .npmrc for pnpm compatibility..."
cat > "$APP_DIR/.npmrc" << EOF
shamefully-hoist=true
EOF
log_success "Created $APP_DIR/.npmrc"

# Install dependencies
install_deps "$APP_DIR"

# Initialize database if it doesn't exist
DB_PATH="$APP_DIR/data.db"
if [[ ! -f "$DB_PATH" ]]; then
    init_database "$DB_PATH"
else
    log_info "Database already exists, skipping init"
fi

# Create the schema (adapted for SQLite from Atomic CRM migrations)
log_info "Creating Atomic CRM schema..."
sqlite3 "$DB_PATH" << 'EOF'
-- Create sales table (team members)
CREATE TABLE IF NOT EXISTS sales (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    user_id TEXT UNIQUE,
    administrator INTEGER DEFAULT 0,
    avatar TEXT,
    disabled INTEGER DEFAULT 0
);

-- Create companies table
CREATE TABLE IF NOT EXISTS companies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sector TEXT,
    size INTEGER,
    phone_number TEXT,
    address TEXT,
    zipcode TEXT,
    city TEXT,
    stateAbbr TEXT,
    country TEXT,
    website TEXT,
    linkedIn TEXT,
    sales_id INTEGER REFERENCES sales(id),
    description TEXT,
    revenue TEXT,
    tax_identifier TEXT,
    logo TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Create contacts table
CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    gender TEXT,
    title TEXT,
    email_jsonb TEXT DEFAULT '[]',
    phone_jsonb TEXT DEFAULT '[]',
    background TEXT,
    avatar TEXT,
    first_seen TEXT DEFAULT (datetime('now')),
    last_seen TEXT DEFAULT (datetime('now')),
    has_newsletter INTEGER DEFAULT 0,
    status TEXT,
    tags TEXT DEFAULT '[]',
    company_id INTEGER REFERENCES companies(id),
    sales_id INTEGER REFERENCES sales(id)
);

-- Create contactNotes table
CREATE TABLE IF NOT EXISTS "contactNotes" (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id INTEGER REFERENCES contacts(id),
    text TEXT,
    date TEXT DEFAULT (datetime('now')),
    sales_id INTEGER REFERENCES sales(id),
    status TEXT,
    attachments TEXT DEFAULT '[]'
);

-- Create deals table
CREATE TABLE IF NOT EXISTS deals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    company_id INTEGER REFERENCES companies(id),
    contact_ids TEXT DEFAULT '[]',
    category TEXT,
    stage TEXT,
    description TEXT,
    amount TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    archived_at TEXT,
    expected_closing_date TEXT,
    sales_id INTEGER REFERENCES sales(id),
    "index" INTEGER DEFAULT 0
);

-- Create dealNotes table
CREATE TABLE IF NOT EXISTS "dealNotes" (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deal_id INTEGER REFERENCES deals(id),
    type TEXT,
    text TEXT,
    date TEXT DEFAULT (datetime('now')),
    sales_id INTEGER REFERENCES sales(id),
    attachments TEXT DEFAULT '[]'
);

-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id INTEGER REFERENCES contacts(id),
    sales_id INTEGER REFERENCES sales(id),
    type TEXT,
    text TEXT,
    due_date TEXT,
    done_date TEXT
);

-- Create tags table
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT
);

-- Register column types in _columns for type tracking
INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type) VALUES
    ('sales', 'id', 'integer'),
    ('sales', 'first_name', 'text'),
    ('sales', 'last_name', 'text'),
    ('sales', 'email', 'text'),
    ('sales', 'user_id', 'uuid'),
    ('sales', 'administrator', 'boolean'),
    ('sales', 'avatar', 'jsonb'),
    ('sales', 'disabled', 'boolean'),
    ('companies', 'id', 'integer'),
    ('companies', 'name', 'text'),
    ('companies', 'sector', 'text'),
    ('companies', 'size', 'integer'),
    ('companies', 'phone_number', 'text'),
    ('companies', 'address', 'text'),
    ('companies', 'zipcode', 'text'),
    ('companies', 'city', 'text'),
    ('companies', 'stateAbbr', 'text'),
    ('companies', 'country', 'text'),
    ('companies', 'website', 'text'),
    ('companies', 'linkedIn', 'text'),
    ('companies', 'sales_id', 'integer'),
    ('companies', 'description', 'text'),
    ('companies', 'revenue', 'numeric'),
    ('companies', 'tax_identifier', 'text'),
    ('companies', 'logo', 'jsonb'),
    ('companies', 'created_at', 'timestamptz'),
    ('contacts', 'id', 'integer'),
    ('contacts', 'first_name', 'text'),
    ('contacts', 'last_name', 'text'),
    ('contacts', 'gender', 'text'),
    ('contacts', 'title', 'text'),
    ('contacts', 'email_jsonb', 'jsonb'),
    ('contacts', 'phone_jsonb', 'jsonb'),
    ('contacts', 'background', 'text'),
    ('contacts', 'avatar', 'jsonb'),
    ('contacts', 'first_seen', 'timestamptz'),
    ('contacts', 'last_seen', 'timestamptz'),
    ('contacts', 'has_newsletter', 'boolean'),
    ('contacts', 'status', 'text'),
    ('contacts', 'tags', 'jsonb'),
    ('contacts', 'company_id', 'integer'),
    ('contacts', 'sales_id', 'integer'),
    ('contactNotes', 'id', 'integer'),
    ('contactNotes', 'contact_id', 'integer'),
    ('contactNotes', 'text', 'text'),
    ('contactNotes', 'date', 'timestamptz'),
    ('contactNotes', 'sales_id', 'integer'),
    ('contactNotes', 'status', 'text'),
    ('contactNotes', 'attachments', 'jsonb'),
    ('deals', 'id', 'integer'),
    ('deals', 'name', 'text'),
    ('deals', 'company_id', 'integer'),
    ('deals', 'contact_ids', 'jsonb'),
    ('deals', 'category', 'text'),
    ('deals', 'stage', 'text'),
    ('deals', 'description', 'text'),
    ('deals', 'amount', 'numeric'),
    ('deals', 'created_at', 'timestamptz'),
    ('deals', 'updated_at', 'timestamptz'),
    ('deals', 'archived_at', 'timestamptz'),
    ('deals', 'expected_closing_date', 'timestamptz'),
    ('deals', 'sales_id', 'integer'),
    ('deals', 'index', 'integer'),
    ('dealNotes', 'id', 'integer'),
    ('dealNotes', 'deal_id', 'integer'),
    ('dealNotes', 'type', 'text'),
    ('dealNotes', 'text', 'text'),
    ('dealNotes', 'date', 'timestamptz'),
    ('dealNotes', 'sales_id', 'integer'),
    ('dealNotes', 'attachments', 'jsonb'),
    ('tasks', 'id', 'integer'),
    ('tasks', 'contact_id', 'integer'),
    ('tasks', 'sales_id', 'integer'),
    ('tasks', 'type', 'text'),
    ('tasks', 'text', 'text'),
    ('tasks', 'due_date', 'timestamptz'),
    ('tasks', 'done_date', 'timestamptz'),
    ('tags', 'id', 'integer'),
    ('tags', 'name', 'text'),
    ('tags', 'color', 'text');

-- Enable RLS on tables
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES
    ('sales', 1),
    ('companies', 1),
    ('contacts', 1),
    ('contactNotes', 1),
    ('deals', 1),
    ('dealNotes', 1),
    ('tasks', 1),
    ('tags', 1);

-- Create RLS policies (authenticated users can access all data in this CRM)
-- Note: The original app has permissive policies for authenticated users
INSERT OR REPLACE INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('sales', 'Authenticated users can view sales', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('sales', 'Authenticated users can insert sales', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('sales', 'Authenticated users can update sales', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('companies', 'Authenticated users can view companies', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('companies', 'Authenticated users can insert companies', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('companies', 'Authenticated users can update companies', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('companies', 'Authenticated users can delete companies', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('contacts', 'Authenticated users can view contacts', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('contacts', 'Authenticated users can insert contacts', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('contacts', 'Authenticated users can update contacts', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('contacts', 'Authenticated users can delete contacts', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('contactNotes', 'Authenticated users can view contactNotes', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('contactNotes', 'Authenticated users can insert contactNotes', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('contactNotes', 'Authenticated users can update contactNotes', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('contactNotes', 'Authenticated users can delete contactNotes', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('deals', 'Authenticated users can view deals', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('deals', 'Authenticated users can insert deals', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('deals', 'Authenticated users can update deals', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('deals', 'Authenticated users can delete deals', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('dealNotes', 'Authenticated users can view dealNotes', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('dealNotes', 'Authenticated users can insert dealNotes', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('dealNotes', 'Authenticated users can update dealNotes', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('dealNotes', 'Authenticated users can delete dealNotes', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('tasks', 'Authenticated users can view tasks', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('tasks', 'Authenticated users can insert tasks', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('tasks', 'Authenticated users can update tasks', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('tasks', 'Authenticated users can delete tasks', 'DELETE', 'auth.role() = ''authenticated''', NULL),
    ('tags', 'Authenticated users can view tags', 'SELECT', 'auth.role() = ''authenticated''', NULL),
    ('tags', 'Authenticated users can insert tags', 'INSERT', NULL, 'auth.role() = ''authenticated'''),
    ('tags', 'Authenticated users can update tags', 'UPDATE', 'auth.role() = ''authenticated''', NULL),
    ('tags', 'Authenticated users can delete tags', 'DELETE', 'auth.role() = ''authenticated''', NULL);
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
echo "Note: Create a user account first, and the app will automatically"
echo "create a sales record for you when you sign up."
