#!/bin/bash
#
# run-mail-tests.sh - Run all e2e email tests
#
# This script:
# 1. Builds sblite
# 2. Starts Mailpit via Docker
# 3. Runs catch-mode tests (mail-api, email-flows, verification)
# 4. Runs SMTP-mode tests (smtp)
# 5. Cleans up on exit
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
E2E_DIR="$SCRIPT_DIR"
TEST_DB="$E2E_DIR/test.db"
MAILPIT_CONTAINER="sblite-mailpit-test"
MAILPIT_HTTP_PORT=8025
MAILPIT_SMTP_PORT=1025
SERVER_PORT=8080
SERVER_PID=""
MAILPIT_STARTED_BY_US=false

# Test results tracking
CATCH_TESTS_PASSED=0
SMTP_TESTS_PASSED=0
CATCH_TESTS_FAILED=0
SMTP_TESTS_FAILED=0

log() {
    echo -e "${BLUE}[run-mail-tests]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[run-mail-tests]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[run-mail-tests]${NC} $1"
}

log_error() {
    echo -e "${RED}[run-mail-tests]${NC} $1"
}

# Cleanup function - called on exit/interrupt
cleanup() {
    local exit_code=$?
    log "Cleaning up..."

    # Stop sblite server if running
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        log "Stopping sblite server (PID: $SERVER_PID)..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi

    # Stop and remove Mailpit container only if we started it
    if [ "$MAILPIT_STARTED_BY_US" = true ]; then
        if docker ps -q -f name="$MAILPIT_CONTAINER" 2>/dev/null | grep -q .; then
            log "Stopping Mailpit container..."
            docker stop "$MAILPIT_CONTAINER" >/dev/null 2>&1 || true
        fi
        if docker ps -aq -f name="$MAILPIT_CONTAINER" 2>/dev/null | grep -q .; then
            log "Removing Mailpit container..."
            docker rm "$MAILPIT_CONTAINER" >/dev/null 2>&1 || true
        fi
    fi

    # Remove test database
    if [ -f "$TEST_DB" ]; then
        log "Removing test database..."
        rm -f "$TEST_DB" "$TEST_DB-wal" "$TEST_DB-shm" 2>/dev/null || true
    fi

    log "Cleanup complete"
    exit $exit_code
}

trap cleanup EXIT INT TERM

# Check if a port is in use
port_in_use() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -i ":$port" >/dev/null 2>&1
    elif command -v netstat >/dev/null 2>&1; then
        netstat -an | grep -q ":$port.*LISTEN"
    else
        # Fallback: try to connect
        (echo >/dev/tcp/localhost/$port) 2>/dev/null
    fi
}

# Wait for a service to be ready
wait_for_service() {
    local host=$1
    local port=$2
    local service_name=$3
    local max_attempts=${4:-30}
    local attempt=1

    log "Waiting for $service_name on $host:$port..."
    while [ $attempt -le $max_attempts ]; do
        if curl -s "http://$host:$port" >/dev/null 2>&1; then
            log_success "$service_name is ready"
            return 0
        fi
        sleep 1
        attempt=$((attempt + 1))
    done

    log_error "$service_name failed to start after $max_attempts seconds"
    return 1
}

# Build sblite
build_sblite() {
    log "Building sblite..."
    cd "$PROJECT_ROOT"
    go build -o sblite .
    log_success "Build complete"
}

# Check if Mailpit API is responding
mailpit_is_running() {
    curl -s "http://localhost:$MAILPIT_HTTP_PORT/api/v1/info" >/dev/null 2>&1
}

# Start Mailpit via Docker
start_mailpit() {
    log "Checking for Mailpit..."

    # Check if our named container is already running
    if docker ps -q -f name="$MAILPIT_CONTAINER" 2>/dev/null | grep -q .; then
        log_warn "Mailpit container '$MAILPIT_CONTAINER' already running, reusing it"
        MAILPIT_STARTED_BY_US=false
        return 0
    fi

    # Check if Mailpit is already running on expected ports (e.g., from another container)
    if mailpit_is_running; then
        log_warn "Mailpit already running on port $MAILPIT_HTTP_PORT, reusing existing instance"
        MAILPIT_STARTED_BY_US=false
        return 0
    fi

    # Check if ports are in use by something else
    if port_in_use $MAILPIT_HTTP_PORT; then
        log_error "Port $MAILPIT_HTTP_PORT is in use by something other than Mailpit"
        exit 1
    fi
    if port_in_use $MAILPIT_SMTP_PORT; then
        log_error "Port $MAILPIT_SMTP_PORT is already in use"
        exit 1
    fi

    log "Starting Mailpit container..."

    # Remove any stopped container with the same name
    if docker ps -aq -f name="$MAILPIT_CONTAINER" 2>/dev/null | grep -q .; then
        docker rm "$MAILPIT_CONTAINER" >/dev/null 2>&1
    fi

    # Start Mailpit
    docker run -d --name "$MAILPIT_CONTAINER" \
        -p "$MAILPIT_HTTP_PORT:8025" \
        -p "$MAILPIT_SMTP_PORT:1025" \
        axllent/mailpit \
        --smtp-auth-accept-any --smtp-auth-allow-insecure >/dev/null

    MAILPIT_STARTED_BY_US=true

    # Wait for Mailpit to be ready
    wait_for_service localhost $MAILPIT_HTTP_PORT "Mailpit"
}

# Initialize test database
init_test_db() {
    log "Initializing test database..."
    cd "$PROJECT_ROOT"

    # Remove existing test database
    rm -f "$TEST_DB" "$TEST_DB-wal" "$TEST_DB-shm" 2>/dev/null || true

    # Initialize new database
    ./sblite init --db "$TEST_DB"
    log_success "Test database initialized"
}

# Start sblite server in catch mode
start_server_catch() {
    log "Starting sblite server in CATCH mode..."
    cd "$PROJECT_ROOT"

    if port_in_use $SERVER_PORT; then
        log_error "Port $SERVER_PORT is already in use"
        exit 1
    fi

    SBLITE_JWT_SECRET=test-secret-key-for-testing \
    SBLITE_SITE_URL="http://localhost:$SERVER_PORT" \
    SBLITE_MAIL_FROM="noreply@test.local" \
    ./sblite serve --mail-mode=catch --db "$TEST_DB" --port $SERVER_PORT &
    SERVER_PID=$!

    wait_for_service localhost $SERVER_PORT "sblite (catch mode)"
}

# Start sblite server in SMTP mode
start_server_smtp() {
    log "Starting sblite server in SMTP mode..."
    cd "$PROJECT_ROOT"

    if port_in_use $SERVER_PORT; then
        log_error "Port $SERVER_PORT is already in use"
        exit 1
    fi

    SBLITE_JWT_SECRET=test-secret-key-for-testing \
    SBLITE_SITE_URL="http://localhost:$SERVER_PORT" \
    SBLITE_MAIL_FROM="noreply@test.local" \
    SBLITE_SMTP_HOST=localhost \
    SBLITE_SMTP_PORT=$MAILPIT_SMTP_PORT \
    SBLITE_SMTP_USER=test \
    SBLITE_SMTP_PASS=test \
    ./sblite serve --mail-mode=smtp --db "$TEST_DB" --port $SERVER_PORT &
    SERVER_PID=$!

    wait_for_service localhost $SERVER_PORT "sblite (SMTP mode)"
}

# Stop the sblite server
stop_server() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        log "Stopping sblite server..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
        SERVER_PID=""
        # Brief pause to ensure port is released
        sleep 1
    fi
}

# Run catch mode tests
run_catch_tests() {
    log "Running catch mode tests..."
    cd "$E2E_DIR"

    local test_files=(
        "tests/email/mail-api.test.ts"
        "tests/email/email-flows.test.ts"
        "tests/email/verification.test.ts"
    )

    if npm test -- "${test_files[@]}"; then
        CATCH_TESTS_PASSED=1
        log_success "Catch mode tests passed"
    else
        CATCH_TESTS_FAILED=1
        log_error "Catch mode tests failed"
    fi
}

# Run SMTP mode tests
run_smtp_tests() {
    log "Running SMTP mode tests..."
    cd "$E2E_DIR"

    # Clear Mailpit inbox before tests
    curl -s -X DELETE "http://localhost:$MAILPIT_HTTP_PORT/api/v1/messages" >/dev/null || true

    if SBLITE_TEST_SMTP=true npm test -- tests/email/smtp.test.ts; then
        SMTP_TESTS_PASSED=1
        log_success "SMTP mode tests passed"
    else
        SMTP_TESTS_FAILED=1
        log_error "SMTP mode tests failed"
    fi
}

# Print test summary
print_summary() {
    echo ""
    echo "========================================"
    echo "           TEST SUMMARY"
    echo "========================================"

    if [ $CATCH_TESTS_PASSED -eq 1 ]; then
        echo -e "Catch mode tests: ${GREEN}PASSED${NC}"
    elif [ $CATCH_TESTS_FAILED -eq 1 ]; then
        echo -e "Catch mode tests: ${RED}FAILED${NC}"
    else
        echo -e "Catch mode tests: ${YELLOW}NOT RUN${NC}"
    fi

    if [ $SMTP_TESTS_PASSED -eq 1 ]; then
        echo -e "SMTP mode tests:  ${GREEN}PASSED${NC}"
    elif [ $SMTP_TESTS_FAILED -eq 1 ]; then
        echo -e "SMTP mode tests:  ${RED}FAILED${NC}"
    else
        echo -e "SMTP mode tests:  ${YELLOW}NOT RUN${NC}"
    fi

    echo "========================================"

    # Return failure if any tests failed
    if [ $CATCH_TESTS_FAILED -eq 1 ] || [ $SMTP_TESTS_FAILED -eq 1 ]; then
        return 1
    fi
    return 0
}

# Install npm dependencies if needed
ensure_npm_deps() {
    cd "$E2E_DIR"
    if [ ! -d "node_modules" ]; then
        log "Installing npm dependencies..."
        npm install
    fi
}

# Main function
main() {
    log "Starting e2e email tests..."
    echo ""

    # Prerequisites
    build_sblite
    ensure_npm_deps
    start_mailpit
    init_test_db

    echo ""
    log "========== PHASE 1: CATCH MODE TESTS =========="
    echo ""

    start_server_catch
    run_catch_tests || true  # Don't exit on failure
    stop_server

    echo ""
    log "========== PHASE 2: SMTP MODE TESTS =========="
    echo ""

    # Re-initialize database for clean state
    init_test_db
    start_server_smtp
    run_smtp_tests || true  # Don't exit on failure
    stop_server

    echo ""
    print_summary
}

# Run main
main "$@"
