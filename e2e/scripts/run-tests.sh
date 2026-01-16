#!/bin/bash
#
# sblite E2E Test Runner
#
# Usage:
#   ./scripts/run-tests.sh [options]
#
# Options:
#   --setup     Setup test database before running
#   --server    Start sblite server before running (and stop after)
#   --all       Run all tests
#   --rest      Run REST API tests only
#   --auth      Run Auth tests only
#   --filters   Run Filter tests only
#   --modifiers Run Modifier tests only
#   --watch     Run in watch mode
#   --ui        Run with Vitest UI
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_DIR="$(dirname "$E2E_DIR")"

# Default options
SETUP=false
START_SERVER=false
TEST_PATTERN=""
WATCH=false
UI=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --setup)
            SETUP=true
            shift
            ;;
        --server)
            START_SERVER=true
            shift
            ;;
        --all)
            TEST_PATTERN=""
            shift
            ;;
        --rest)
            TEST_PATTERN="rest/"
            shift
            ;;
        --auth)
            TEST_PATTERN="auth/"
            shift
            ;;
        --filters)
            TEST_PATTERN="filters/"
            shift
            ;;
        --modifiers)
            TEST_PATTERN="modifiers/"
            shift
            ;;
        --watch)
            WATCH=true
            shift
            ;;
        --ui)
            UI=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

cd "$E2E_DIR"

# Setup test database if requested
if [ "$SETUP" = true ]; then
    echo "📦 Setting up test database..."
    npx tsx scripts/setup-test-db.ts
fi

# Start server if requested
SERVER_PID=""
if [ "$START_SERVER" = true ]; then
    echo "🚀 Starting sblite server..."
    cd "$PROJECT_DIR"

    # Build if not exists
    if [ ! -f "./sblite" ]; then
        echo "   Building sblite..."
        go build -o sblite .
    fi

    # Start server in background
    export SBLITE_JWT_SECRET="test-secret-key"
    ./sblite serve --db "$E2E_DIR/../test.db" &
    SERVER_PID=$!

    # Wait for server to be ready
    echo "   Waiting for server..."
    sleep 2

    cd "$E2E_DIR"
fi

# Cleanup function
cleanup() {
    if [ -n "$SERVER_PID" ]; then
        echo "🛑 Stopping sblite server..."
        kill $SERVER_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Build test command
TEST_CMD="npx vitest"

if [ "$WATCH" = true ]; then
    TEST_CMD="$TEST_CMD"
elif [ "$UI" = true ]; then
    TEST_CMD="$TEST_CMD --ui"
else
    TEST_CMD="$TEST_CMD run"
fi

if [ -n "$TEST_PATTERN" ]; then
    TEST_CMD="$TEST_CMD --testPathPattern=$TEST_PATTERN"
fi

# Run tests
echo "🧪 Running tests..."
echo "   Command: $TEST_CMD"
echo ""

$TEST_CMD
