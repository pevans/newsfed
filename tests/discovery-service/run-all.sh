#!/bin/bash
# Run discovery service black box tests

echo "RFC 7 Discovery Service Black Box Tests"
echo "========================================"
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to script directory
cd "$SCRIPT_DIR"

# Setup test data
echo "Setting up test environment..."
./setup.sh
echo ""

# Run all tests
TOTAL_FAILED=0

echo "Running test suites..."
echo ""

# Test RSS Feed Discovery (Section 4)
if ./test-rss-feed.sh; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Error Handling (Section 7)
if ./test-error-handling.sh; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Deduplication (Section 6)
if ./test-deduplication.sh; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Logging and Monitoring (Section 10)
if ./test-metrics.sh; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test 20-Item Cap (RFC 2 Section 2.2.3, RFC 3 Section 3.1.1)
if ./test-20-item-cap.sh; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Cleanup test data and binary
echo "Cleaning up test environment..."

# Stop HTTP server if running
if [ -f .http-server-pid ]; then
    HTTP_PID=$(cat .http-server-pid)
    if kill -0 $HTTP_PID 2>/dev/null; then
        echo "Stopping HTTP server (PID: $HTTP_PID)..."
        kill $HTTP_PID 2>/dev/null
    fi
    rm .http-server-pid
fi

rm -rf test-metadata.db test-feed/ test-fixtures/ newsfed-discover
echo ""

# Final summary
echo "========================================"
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "All discovery service tests passed! ✓"
    exit 0
else
    echo "Some tests failed ($TOTAL_FAILED total failures)"
    exit 1
fi
