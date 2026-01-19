#!/bin/bash
# common.sh - Shared functions for test app scripts
# Source this file in individual setup scripts

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get the root directory of the sblite project
SBLITE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_APPS_DIR="$SBLITE_ROOT/test_apps"
SCRIPTS_DIR="$TEST_APPS_DIR/scripts"

# Default configuration
SBLITE_HOST="${SBLITE_HOST:-localhost}"
SBLITE_PORT="${SBLITE_PORT:-8080}"
SBLITE_URL="http://${SBLITE_HOST}:${SBLITE_PORT}"
SBLITE_JWT_SECRET="${SBLITE_JWT_SECRET:-super-secret-jwt-key-please-change-in-production}"

# Export for sblite to use
export SBLITE_JWT_SECRET

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if sblite binary exists, build if not
ensure_sblite() {
    if [[ ! -f "$SBLITE_ROOT/sblite" ]]; then
        log_info "Building sblite..."
        cd "$SBLITE_ROOT"
        go build -o sblite .
        log_success "sblite built successfully"
    else
        log_info "sblite binary found"
    fi
}

# Initialize a database for a test app
init_database() {
    local db_path="$1"
    if [[ -z "$db_path" ]]; then
        log_error "Database path required"
        return 1
    fi

    log_info "Initializing database at $db_path..."
    "$SBLITE_ROOT/sblite" init --db "$db_path"
    log_success "Database initialized"
}

# Generate API keys and return them
generate_keys() {
    log_info "Generating API keys..."
    local output
    output=$("$SBLITE_ROOT/sblite" keys generate 2>/dev/null)

    ANON_KEY=$(echo "$output" | grep "SBLITE_ANON_KEY=" | cut -d'=' -f2)
    SERVICE_KEY=$(echo "$output" | grep "SBLITE_SERVICE_KEY=" | cut -d'=' -f2)

    if [[ -z "$ANON_KEY" || -z "$SERVICE_KEY" ]]; then
        log_error "Failed to generate API keys"
        return 1
    fi

    log_success "API keys generated"
    export ANON_KEY
    export SERVICE_KEY
}

# Download a directory from GitHub using the API (downloads only the specific folder)
download_github_dir() {
    local owner="$1"
    local repo="$2"
    local path="$3"
    local target="$4"
    local branch="${5:-master}"

    mkdir -p "$target"

    # Get directory contents from GitHub API
    local api_url="https://api.github.com/repos/$owner/$repo/contents/$path?ref=$branch"
    local response=$(curl -sL "$api_url")

    # Check for API errors
    if echo "$response" | grep -q '"message"'; then
        local msg=$(echo "$response" | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
        log_error "GitHub API error: $msg"
        return 1
    fi

    # Parse JSON and download each item
    echo "$response" | python3 -c "
import json
import sys
import os
import urllib.request

data = json.load(sys.stdin)
target = '$target'
owner = '$owner'
repo = '$repo'
branch = '$branch'

for item in data:
    name = item['name']
    item_type = item['type']

    if item_type == 'file':
        filepath = os.path.join(target, name)
        print(f'  Downloading {name}', file=sys.stderr)
        urllib.request.urlretrieve(item['download_url'], filepath)
    elif item_type == 'dir':
        # Print directory for recursive handling
        print(item['path'])
"
}

# Clone a Supabase example - downloads only the specific folder
clone_supabase_example() {
    local example_path="$1"  # e.g., "examples/todo-list/nextjs-todo-list"
    local target_dir="$2"    # e.g., "nextjs-todo-list"

    local full_target="$TEST_APPS_DIR/$target_dir"

    if [[ -d "$full_target" ]]; then
        log_warn "Directory $full_target already exists"
        return 0
    fi

    # Check if we have a local template
    local local_template="$TEST_APPS_DIR/templates/$target_dir"
    if [[ -d "$local_template" ]]; then
        log_info "Using local template for $target_dir..."
        cp -r "$local_template" "$full_target"
        log_success "Copied local template to $full_target"
        return 0
    fi

    log_info "Downloading $example_path (only this folder, not entire repo)..."
    mkdir -p "$full_target"

    # Recursive function to download directory
    download_dir_recursive() {
        local path="$1"
        local target="$2"

        local subdirs=$(download_github_dir "supabase" "supabase" "$path" "$target" "master")

        # Recursively download subdirectories
        while IFS= read -r subdir; do
            if [[ -n "$subdir" ]]; then
                local subdir_name=$(basename "$subdir")
                local subdir_target="$target/$subdir_name"
                mkdir -p "$subdir_target"
                log_info "  Entering $subdir_name/"
                download_dir_recursive "$subdir" "$subdir_target"
            fi
        done <<< "$subdirs"
    }

    download_dir_recursive "$example_path" "$full_target"

    if [[ -z "$(ls -A "$full_target" 2>/dev/null)" ]]; then
        log_error "Failed to download $example_path"
        rm -rf "$full_target"
        return 1
    fi

    log_success "Downloaded to $full_target"
}

# Create .env.local file for Next.js apps
create_nextjs_env() {
    local app_dir="$1"
    local env_file="$app_dir/.env.local"

    cat > "$env_file" << EOF
# sblite configuration
NEXT_PUBLIC_SUPABASE_URL=$SBLITE_URL
NEXT_PUBLIC_SUPABASE_ANON_KEY=$ANON_KEY
# Some examples use PUBLISHABLE_KEY instead of ANON_KEY
NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=$ANON_KEY

# Service role key for server-side operations
SUPABASE_SERVICE_ROLE_KEY=$SERVICE_KEY
EOF

    log_success "Created $env_file"
}

# Create .env file for React (Vite) apps
create_vite_env() {
    local app_dir="$1"
    local env_file="$app_dir/.env"

    cat > "$env_file" << EOF
# sblite configuration
VITE_SUPABASE_URL=$SBLITE_URL
VITE_SUPABASE_ANON_KEY=$ANON_KEY
EOF

    log_success "Created $env_file"
}

# Create .env file for Vue apps
create_vue_env() {
    local app_dir="$1"
    local env_file="$app_dir/.env"

    cat > "$env_file" << EOF
# sblite configuration
VITE_SUPABASE_URL=$SBLITE_URL
VITE_SUPABASE_ANON_KEY=$ANON_KEY
EOF

    log_success "Created $env_file"
}

# Create .env file for Svelte apps
create_svelte_env() {
    local app_dir="$1"
    local env_file="$app_dir/.env"

    cat > "$env_file" << EOF
# sblite configuration
VITE_SUPABASE_URL=$SBLITE_URL
VITE_SUPABASE_ANON_KEY=$ANON_KEY
EOF

    log_success "Created $env_file"
}

# Create .env file for SvelteKit apps
create_sveltekit_env() {
    local app_dir="$1"
    local env_file="$app_dir/.env"

    cat > "$env_file" << EOF
# sblite configuration
PUBLIC_SUPABASE_URL=$SBLITE_URL
PUBLIC_SUPABASE_ANON_KEY=$ANON_KEY
SUPABASE_SERVICE_ROLE_KEY=$SERVICE_KEY
EOF

    log_success "Created $env_file"
}

# Install dependencies with pnpm
install_deps() {
    local app_dir="$1"
    log_info "Installing dependencies in $app_dir..."
    cd "$app_dir"
    pnpm install
    log_success "Dependencies installed"
}

# Create a startup script for an app
create_startup_script() {
    local app_name="$1"
    local app_dir_name="$2"  # Just the directory name, not full path
    local db_name="$3"
    local dev_command="${4:-pnpm run dev}"
    local needs_functions="${5:-false}"

    local script_path="$SCRIPTS_DIR/start-${app_name}.sh"

    local functions_flag=""
    if [[ "$needs_functions" == "true" ]]; then
        functions_flag=" --functions"
    fi

    cat > "$script_path" << 'SCRIPT_START'
#!/bin/bash
# Start script for APP_NAME_PLACEHOLDER
# This script starts sblite and the app together

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_DIR="$TEST_APPS_DIR/APP_DIR_PLACEHOLDER"
DB_PATH="$APP_DIR/DB_NAME_PLACEHOLDER.db"
SCRIPT_START

    # Replace placeholders
    sed -i.bak "s/APP_NAME_PLACEHOLDER/$app_name/g" "$script_path"
    sed -i.bak "s/APP_DIR_PLACEHOLDER/$app_name/g" "$script_path"
    sed -i.bak "s/DB_NAME_PLACEHOLDER/$db_name/g" "$script_path"
    rm -f "$script_path.bak"

    cat >> "$script_path" << EOF

# Ensure sblite is built
ensure_sblite

# Initialize database if it doesn't exist
if [[ ! -f "\$DB_PATH" ]]; then
    init_database "\$DB_PATH"
fi

# Function to cleanup on exit
cleanup() {
    log_info "Shutting down..."
    if [[ -n "\$SBLITE_PID" ]]; then
        kill "\$SBLITE_PID" 2>/dev/null || true
    fi
    if [[ -n "\$APP_PID" ]]; then
        kill "\$APP_PID" 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Start sblite in background
log_info "Starting sblite on $SBLITE_URL..."
"\$SBLITE_ROOT/sblite" serve --db "\$DB_PATH" --port $SBLITE_PORT --host $SBLITE_HOST$functions_flag &
SBLITE_PID=\$!

# Wait for sblite to be ready
sleep 2

# Check if sblite is running
if ! kill -0 "\$SBLITE_PID" 2>/dev/null; then
    log_error "sblite failed to start"
    exit 1
fi

log_success "sblite started (PID: \$SBLITE_PID)"

# Start the app
log_info "Starting $app_name..."
cd "\$APP_DIR"
$dev_command &
APP_PID=\$!

log_success "$app_name started (PID: \$APP_PID)"
log_info "Press Ctrl+C to stop both services"

# Wait for either process to exit
wait
EOF

    chmod +x "$script_path"
    log_success "Created startup script: $script_path"
}

# Run SQL file against sblite database using the SQL endpoint
run_sql_file() {
    local db_path="$1"
    local sql_file="$2"

    if [[ ! -f "$sql_file" ]]; then
        log_warn "SQL file not found: $sql_file"
        return 1
    fi

    log_info "Running SQL from $sql_file..."

    # Use sqlite3 directly for setup
    sqlite3 "$db_path" < "$sql_file"

    log_success "SQL executed"
}

# Run inline SQL against sblite database
run_sql() {
    local db_path="$1"
    local sql="$2"

    log_info "Running SQL..."
    sqlite3 "$db_path" "$sql"
    log_success "SQL executed"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Ensure required tools are installed
check_requirements() {
    local missing=()

    for cmd in git node pnpm go sqlite3; do
        if ! command_exists "$cmd"; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing[*]}"
        return 1
    fi

    log_success "All requirements satisfied"
}

log_info "common.sh loaded"
