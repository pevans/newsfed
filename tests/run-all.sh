#!/bin/bash
# Run all test suites recursively

set -e

BASE_URL="http://localhost:8080/api/v1"
SERVER_PID=""

# Cleanup function to stop server on exit
cleanup() {
    if [ -n "$SERVER_PID" ]; then
        echo ""
        echo "Stopping API server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
}

# Register cleanup function to run on exit
trap cleanup EXIT

echo "newsfed Test Suite"
echo "=================="
echo ""

# Check if server is already running
if curl -s -f "$BASE_URL/items" > /dev/null 2>&1; then
    echo "✓ API server already running at $BASE_URL"
else
    # Start the API server
    echo "Starting API server..."
    go run . > /dev/null 2>&1 &
    SERVER_PID=$!

    # Wait for server to be ready (max 10 seconds)
    echo "Waiting for server to be ready..."
    for i in {1..20}; do
        if curl -s -f "$BASE_URL/items" > /dev/null 2>&1; then
            echo "✓ API server started (PID: $SERVER_PID)"
            break
        fi
        if [ $i -eq 20 ]; then
            echo "Error: Server failed to start after 10 seconds"
            exit 1
        fi
        sleep 0.5
    done
fi
echo ""

# Find and run all run-all.sh scripts in subdirectories
TOTAL_FAILED=0
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for test_suite in "$SCRIPT_DIR"/*/run-all.sh; do
    if [ -f "$test_suite" ]; then
        suite_name=$(basename "$(dirname "$test_suite")")
        echo ""
        echo "Running test suite: $suite_name"
        echo "-----------------------------------"

        if "$test_suite"; then
            echo ""
        else
            TOTAL_FAILED=$((TOTAL_FAILED + $?))
        fi
    fi
done

# Final summary
echo ""
echo "===================="
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "All tests passed! ✓"
    exit 0
else
    echo "Some tests failed"
    exit 1
fi
