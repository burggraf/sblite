#!/bin/bash
# Test all logging modes for sblite
# Usage: ./scripts/test-logging.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test directory
TEST_DIR=$(mktemp -d)
TEST_DB="$TEST_DIR/test.db"
LOG_FILE="$TEST_DIR/test.log"
LOG_DB="$TEST_DIR/log.db"

echo "Test directory: $TEST_DIR"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
    sleep 0.5
    rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Build if needed
if [ ! -f "./sblite" ]; then
    echo -e "${YELLOW}Building sblite...${NC}"
    go build -o sblite .
fi

# Initialize test database
echo -e "\n${YELLOW}Initializing test database...${NC}"
./sblite init --db "$TEST_DB"

# Helper function to start server and make request
test_mode() {
    local mode_name="$1"
    shift
    local extra_args="$@"

    echo -e "\n${YELLOW}Testing: $mode_name${NC}"

    # Start server in background
    ./sblite serve --db "$TEST_DB" $extra_args &
    local pid=$!
    sleep 1

    # Make test requests
    curl -s http://localhost:8080/health > /dev/null
    curl -s http://localhost:8080/auth/v1/settings > /dev/null
    curl -s -X POST http://localhost:8080/auth/v1/token -d '{}' > /dev/null 2>&1 || true

    # Stop server
    kill $pid 2>/dev/null || true
    wait $pid 2>/dev/null || true
    sleep 0.5
}

# ============================================
# Test 1: Console Mode - Text Format
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 1: Console Mode - Text Format${NC}"
echo -e "${GREEN}========================================${NC}"

OUTPUT=$(./sblite serve --db "$TEST_DB" --log-format=text 2>&1 &
    sleep 1
    curl -s http://localhost:8080/health > /dev/null
    sleep 0.2
    pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
    sleep 0.5
)

echo "$OUTPUT"

if echo "$OUTPUT" | grep -q "level=INFO"; then
    echo -e "${GREEN}✓ Text format working${NC}"
else
    echo -e "${RED}✗ Text format not detected${NC}"
fi

# ============================================
# Test 2: Console Mode - JSON Format
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 2: Console Mode - JSON Format${NC}"
echo -e "${GREEN}========================================${NC}"

OUTPUT=$(./sblite serve --db "$TEST_DB" --log-format=json 2>&1 &
    sleep 1
    curl -s http://localhost:8080/health > /dev/null
    sleep 0.2
    pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
    sleep 0.5
)

echo "$OUTPUT"

if echo "$OUTPUT" | grep -q '"level":"INFO"'; then
    echo -e "${GREEN}✓ JSON format working${NC}"
else
    echo -e "${RED}✗ JSON format not detected${NC}"
fi

# ============================================
# Test 3: Log Level Filtering
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 3: Log Level Filtering (warn)${NC}"
echo -e "${GREEN}========================================${NC}"

OUTPUT=$(./sblite serve --db "$TEST_DB" --log-level=warn 2>&1 &
    sleep 1
    curl -s http://localhost:8080/health > /dev/null
    sleep 0.2
    pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
    sleep 0.5
)

echo "$OUTPUT"

# Should see WARN (JWT secret) but not INFO (http request)
if echo "$OUTPUT" | grep -q "level=WARN" && ! echo "$OUTPUT" | grep -q "http request"; then
    echo -e "${GREEN}✓ Level filtering working${NC}"
else
    echo -e "${RED}✗ Level filtering may not be working correctly${NC}"
fi

# ============================================
# Test 4: File Mode
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 4: File Mode${NC}"
echo -e "${GREEN}========================================${NC}"

rm -f "$LOG_FILE"*

./sblite serve --db "$TEST_DB" --log-mode=file --log-file="$LOG_FILE" &
sleep 1
curl -s http://localhost:8080/health > /dev/null
curl -s http://localhost:8080/auth/v1/settings > /dev/null
sleep 0.2
pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
sleep 0.5

echo "Log file contents:"
cat "$LOG_FILE"

if [ -f "$LOG_FILE" ] && grep -q "http request" "$LOG_FILE"; then
    echo -e "${GREEN}✓ File logging working${NC}"
else
    echo -e "${RED}✗ File logging not working${NC}"
fi

# ============================================
# Test 5: File Mode - JSON Format
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 5: File Mode - JSON Format${NC}"
echo -e "${GREEN}========================================${NC}"

rm -f "$LOG_FILE"*

./sblite serve --db "$TEST_DB" --log-mode=file --log-file="$LOG_FILE" --log-format=json &
sleep 1
curl -s http://localhost:8080/health > /dev/null
sleep 0.2
pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
sleep 0.5

echo "Log file contents:"
cat "$LOG_FILE"

if grep -q '"msg":"http request"' "$LOG_FILE"; then
    echo -e "${GREEN}✓ File JSON format working${NC}"
else
    echo -e "${RED}✗ File JSON format not working${NC}"
fi

# ============================================
# Test 6: Database Mode
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 6: Database Mode${NC}"
echo -e "${GREEN}========================================${NC}"

rm -f "$LOG_DB"*

./sblite serve --db "$TEST_DB" --log-mode=database --log-db="$LOG_DB" --log-fields=request_id,extra &
sleep 1
curl -s http://localhost:8080/health > /dev/null
curl -s http://localhost:8080/auth/v1/settings > /dev/null
curl -s -X POST http://localhost:8080/auth/v1/signup -H "Content-Type: application/json" -d '{"email":"test@example.com","password":"password123"}' > /dev/null 2>&1 || true
sleep 0.2
pkill -f "sblite serve --db $TEST_DB" 2>/dev/null || true
sleep 0.5

echo "Database log entries:"
sqlite3 "$LOG_DB" "SELECT timestamp, level, message, request_id FROM logs ORDER BY id"

LOG_COUNT=$(sqlite3 "$LOG_DB" "SELECT COUNT(*) FROM logs")
if [ "$LOG_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Database logging working ($LOG_COUNT entries)${NC}"
else
    echo -e "${RED}✗ Database logging not working${NC}"
fi

# Check request_id field
REQ_ID_COUNT=$(sqlite3 "$LOG_DB" "SELECT COUNT(*) FROM logs WHERE request_id IS NOT NULL AND request_id != ''")
if [ "$REQ_ID_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Request ID capture working${NC}"
else
    echo -e "${RED}✗ Request ID not being captured${NC}"
fi

# ============================================
# Test 7: Database Query Examples
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test 7: Database Query Examples${NC}"
echo -e "${GREEN}========================================${NC}"

echo -e "\nCount by level:"
sqlite3 "$LOG_DB" "SELECT level, COUNT(*) as count FROM logs GROUP BY level"

echo -e "\nHTTP requests only:"
sqlite3 "$LOG_DB" "SELECT timestamp, message, request_id FROM logs WHERE message='http request'"

echo -e "\nDistinct levels:"
sqlite3 "$LOG_DB" "SELECT DISTINCT level FROM logs"

# ============================================
# Summary
# ============================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test Summary${NC}"
echo -e "${GREEN}========================================${NC}"
echo "All logging mode tests completed."
echo "Test artifacts in: $TEST_DIR"
echo ""
echo "Modes tested:"
echo "  1. Console - Text format"
echo "  2. Console - JSON format"
echo "  3. Log level filtering"
echo "  4. File - Text format"
echo "  5. File - JSON format"
echo "  6. Database with field capture"
echo "  7. Database queries"
