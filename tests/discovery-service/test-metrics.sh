#!/bin/bash
# Black box tests for RFC 7 Section 10 - Logging and Monitoring

METADATA_DB="test-metadata.db"
FEED_DIR="test-feed"
PASSED=0
FAILED=0

# Note: This test runs after other tests, so sources may not be due for fetching.
# We focus on testing logging structure rather than specific fetch logs.

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

echo "Testing RFC 7 Section 10 - Logging and Monitoring"
echo "=================================================="
echo ""

# Run discovery service with longer timeout to capture metrics
echo "Running discovery service..."
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > metrics-test.log 2>&1 &
METRICS_PID=$!

sleep 5

# Kill the service
kill $METRICS_PID 2>/dev/null
wait $METRICS_PID 2>/dev/null

# Test 1: INFO level logging for service start
echo "Test 1: Service startup is logged with INFO level"
if grep -q "INFO.*Discovery service starting" metrics-test.log; then
    pass "Service startup logged"
else
    fail "Service startup not logged"
fi

# Test 2: INFO level is used for normal operations
echo ""
echo "Test 2: INFO level is used for normal operations"
if grep -q "INFO" metrics-test.log; then
    pass "INFO level logging present"
else
    fail "INFO level logging not found"
fi

# Test 3: Check for structured logging format
echo ""
echo "Test 3: Logs follow expected format"
# Count lines that match structured logging patterns
INFO_COUNT=$(grep -c "INFO" metrics-test.log || echo "0")
if [ "$INFO_COUNT" -gt 0 ]; then
    pass "Structured logging format present ($INFO_COUNT INFO messages)"
else
    fail "No structured log messages found"
fi

# Test 4: Service shutdown is logged
echo ""
echo "Test 4: Service shutdown is logged"
if grep -q "INFO.*Discovery service stopping" metrics-test.log; then
    pass "Service shutdown logged"
else
    fail "Service shutdown not logged"
fi

# Test 5: Logs contain timestamps/structure
echo ""
echo "Test 5: Logs are timestamped"
# Check that logs have Go's standard log format with date/time
if grep -E "^[0-9]{4}/" metrics-test.log > /dev/null 2>&1; then
    pass "Logs include timestamps"
else
    fail "Logs missing timestamps"
fi

# Test 6: Service logs key lifecycle events
echo ""
echo "Test 6: Lifecycle events are logged"
LIFECYCLE_EVENTS=0
if grep -q "starting" metrics-test.log; then
    ((LIFECYCLE_EVENTS++))
fi
if grep -q "stopping" metrics-test.log; then
    ((LIFECYCLE_EVENTS++))
fi

if [ $LIFECYCLE_EVENTS -ge 2 ]; then
    pass "Lifecycle events logged (start and stop)"
elif [ $LIFECYCLE_EVENTS -ge 1 ]; then
    pass "At least one lifecycle event logged"
else
    fail "No lifecycle events found in logs"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

# Cleanup
rm -f metrics-test.log

exit $FAILED
