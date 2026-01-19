#!/bin/bash
# setup-finopenpos.sh - Setup the FinOpenPOS example
# https://github.com/JoaoHenriqueBarbosa/FinOpenPOS

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_NAME="finopenpos"
APP_DIR="$TEST_APPS_DIR/$APP_NAME"

log_info "Setting up $APP_NAME..."

# Check requirements
check_requirements

# Ensure sblite is built
ensure_sblite

# Clone from GitHub if not exists
if [[ ! -d "$APP_DIR" ]]; then
    log_info "Cloning FinOpenPOS from GitHub..."
    git clone --depth 1 https://github.com/JoaoHenriqueBarbosa/FinOpenPOS.git "$APP_DIR"
    log_success "Cloned to $APP_DIR"
else
    log_info "App directory already exists, skipping clone"
fi

# Generate API keys
generate_keys

# Create .env.local
create_nextjs_env "$APP_DIR"

# Install dependencies
install_deps "$APP_DIR"

# Initialize database if it doesn't exist
DB_PATH="$APP_DIR/data.db"
if [[ ! -f "$DB_PATH" ]]; then
    init_database "$DB_PATH"
else
    log_info "Database already exists, skipping init"
fi

# Create the schema (adapted for SQLite)
log_info "Creating FinOpenPOS schema..."
sqlite3 "$DB_PATH" << 'EOF'
-- Create Products table
CREATE TABLE IF NOT EXISTS products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    price TEXT NOT NULL,
    in_stock INTEGER NOT NULL,
    user_uid TEXT NOT NULL,
    category TEXT
);

-- Create Customers table
CREATE TABLE IF NOT EXISTS customers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT,
    user_uid TEXT NOT NULL,
    status TEXT CHECK (status IN ('active', 'inactive')),
    created_at TEXT DEFAULT (datetime('now'))
);

-- Create Orders table
CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_id INTEGER REFERENCES customers(id),
    total_amount TEXT NOT NULL,
    user_uid TEXT NOT NULL,
    status TEXT CHECK (status IN ('pending', 'completed', 'cancelled')),
    created_at TEXT DEFAULT (datetime('now'))
);

-- Create OrderItems table
CREATE TABLE IF NOT EXISTS order_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER REFERENCES orders(id),
    product_id INTEGER REFERENCES products(id),
    quantity INTEGER NOT NULL,
    price TEXT NOT NULL
);

-- Create PaymentMethods table
CREATE TABLE IF NOT EXISTS payment_methods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- Create Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    description TEXT,
    order_id INTEGER REFERENCES orders(id),
    payment_method_id INTEGER REFERENCES payment_methods(id),
    amount TEXT NOT NULL,
    user_uid TEXT NOT NULL,
    type TEXT CHECK (type IN ('income', 'expense')),
    category TEXT,
    status TEXT CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TEXT DEFAULT (datetime('now'))
);

-- Insert initial payment methods
INSERT OR IGNORE INTO payment_methods (name) VALUES ('Credit Card'), ('Debit Card'), ('Cash');

-- Register column types in _columns for type tracking
INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type) VALUES
    ('products', 'id', 'integer'),
    ('products', 'name', 'text'),
    ('products', 'description', 'text'),
    ('products', 'price', 'numeric'),
    ('products', 'in_stock', 'integer'),
    ('products', 'user_uid', 'text'),
    ('products', 'category', 'text'),
    ('customers', 'id', 'integer'),
    ('customers', 'name', 'text'),
    ('customers', 'email', 'text'),
    ('customers', 'phone', 'text'),
    ('customers', 'user_uid', 'text'),
    ('customers', 'status', 'text'),
    ('customers', 'created_at', 'timestamptz'),
    ('orders', 'id', 'integer'),
    ('orders', 'customer_id', 'integer'),
    ('orders', 'total_amount', 'numeric'),
    ('orders', 'user_uid', 'text'),
    ('orders', 'status', 'text'),
    ('orders', 'created_at', 'timestamptz'),
    ('order_items', 'id', 'integer'),
    ('order_items', 'order_id', 'integer'),
    ('order_items', 'product_id', 'integer'),
    ('order_items', 'quantity', 'integer'),
    ('order_items', 'price', 'numeric'),
    ('payment_methods', 'id', 'integer'),
    ('payment_methods', 'name', 'text'),
    ('transactions', 'id', 'integer'),
    ('transactions', 'description', 'text'),
    ('transactions', 'order_id', 'integer'),
    ('transactions', 'payment_method_id', 'integer'),
    ('transactions', 'amount', 'numeric'),
    ('transactions', 'user_uid', 'text'),
    ('transactions', 'type', 'text'),
    ('transactions', 'category', 'text'),
    ('transactions', 'status', 'text'),
    ('transactions', 'created_at', 'timestamptz');

-- Enable RLS on tables
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES
    ('products', 1),
    ('customers', 1),
    ('orders', 1),
    ('order_items', 1),
    ('transactions', 1);

-- Create RLS policies (user can only see their own data)
INSERT OR REPLACE INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('products', 'Users can view own products', 'SELECT', 'auth.uid() = user_uid', NULL),
    ('products', 'Users can insert own products', 'INSERT', NULL, 'auth.uid() = user_uid'),
    ('products', 'Users can update own products', 'UPDATE', 'auth.uid() = user_uid', NULL),
    ('products', 'Users can delete own products', 'DELETE', 'auth.uid() = user_uid', NULL),
    ('customers', 'Users can view own customers', 'SELECT', 'auth.uid() = user_uid', NULL),
    ('customers', 'Users can insert own customers', 'INSERT', NULL, 'auth.uid() = user_uid'),
    ('customers', 'Users can update own customers', 'UPDATE', 'auth.uid() = user_uid', NULL),
    ('customers', 'Users can delete own customers', 'DELETE', 'auth.uid() = user_uid', NULL),
    ('orders', 'Users can view own orders', 'SELECT', 'auth.uid() = user_uid', NULL),
    ('orders', 'Users can insert own orders', 'INSERT', NULL, 'auth.uid() = user_uid'),
    ('orders', 'Users can update own orders', 'UPDATE', 'auth.uid() = user_uid', NULL),
    ('orders', 'Users can delete own orders', 'DELETE', 'auth.uid() = user_uid', NULL),
    ('transactions', 'Users can view own transactions', 'SELECT', 'auth.uid() = user_uid', NULL),
    ('transactions', 'Users can insert own transactions', 'INSERT', NULL, 'auth.uid() = user_uid'),
    ('transactions', 'Users can update own transactions', 'UPDATE', 'auth.uid() = user_uid', NULL),
    ('transactions', 'Users can delete own transactions', 'DELETE', 'auth.uid() = user_uid', NULL);
EOF

log_success "Database schema created"

# Create startup script
create_startup_script "$APP_NAME" "$APP_DIR" "data" "pnpm run dev"

log_success "Setup complete!"
echo ""
echo "To start the app, run:"
echo "  $SCRIPTS_DIR/start-${APP_NAME}.sh"
echo ""
echo "Then open http://localhost:3000 in your browser"
echo "sblite API will be running at $SBLITE_URL"
