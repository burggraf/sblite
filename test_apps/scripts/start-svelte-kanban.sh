#!/bin/bash
# Start script for svelte-kanban
# This script starts sblite and the app together

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

APP_DIR="$TEST_APPS_DIR/svelte-kanban"
DB_PATH="$APP_DIR/data.db"

# Ensure sblite is built
ensure_sblite

# Initialize database if it doesn't exist
if [[ ! -f "$DB_PATH" ]]; then
    init_database "$DB_PATH"
fi

# Function to cleanup on exit
cleanup() {
    log_info "Shutting down..."
    if [[ -n "$SBLITE_PID" ]]; then
        kill "$SBLITE_PID" 2>/dev/null || true
    fi
    if [[ -n "$APP_PID" ]]; then
        kill "$APP_PID" 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Start sblite in background
log_info "Starting sblite on http://localhost:8080..."
"$SBLITE_ROOT/sblite" serve --db "$DB_PATH" --port 8080 --host localhost &
SBLITE_PID=$!

# Wait for sblite to be ready
sleep 2

# Check if sblite is running
if ! kill -0 "$SBLITE_PID" 2>/dev/null; then
    log_error "sblite failed to start"
    exit 1
fi

log_success "sblite started (PID: $SBLITE_PID)"

# Start the app
log_info "Starting svelte-kanban..."
cd "$APP_DIR"
pnpm run dev &
APP_PID=$!

log_success "svelte-kanban started (PID: $APP_PID)"
log_info "Press Ctrl+C to stop both services"

# Wait for either process to exit
wait
