#!/bin/bash
# Run all test suites recursively

set -e

BASE_URL="http://localhost:8080/api/v1"
NEWSFEED_PID=""
METADATA_PID=""

# Cleanup function to stop servers on exit
cleanup() {
    if [ -n "$NEWSFEED_PID" ]; then
        echo ""
        echo "Stopping newsfeed-api (PID: $NEWSFEED_PID)..."
        kill $NEWSFEED_PID 2>/dev/null || true
        wait $NEWSFEED_PID 2>/dev/null || true
    fi

    if [ -n "$METADATA_PID" ]; then
        echo "Stopping metadata-api (PID: $METADATA_PID)..."
        kill $METADATA_PID 2>/dev/null || true
        wait $METADATA_PID 2>/dev/null || true
    fi

    # Also kill any servers running on ports 8080 and 8081
    local PORT_8080=$(lsof -ti:8080 2>/dev/null)
    if [ -n "$PORT_8080" ]; then
        echo "Stopping server on port 8080 (PID: $PORT_8080)..."
        kill $PORT_8080 2>/dev/null || true
    fi

    local PORT_8081=$(lsof -ti:8081 2>/dev/null)
    if [ -n "$PORT_8081" ]; then
        echo "Stopping server on port 8081 (PID: $PORT_8081)..."
        kill $PORT_8081 2>/dev/null || true
    fi
}

# Register cleanup function to run on exit, interrupt, or termination
trap cleanup EXIT INT TERM

echo "newsfed Test Suite"
echo "=================="
echo ""

# Check if newsfeed-api is already running
if curl -s -f "$BASE_URL/items" > /dev/null 2>&1; then
    echo "✓ newsfeed-api already running at $BASE_URL"
else
    # Start the newsfeed-api server
    echo "Starting newsfeed-api..."
    go run ./cmd/newsfeed-api > /dev/null 2>&1 &
    NEWSFEED_PID=$!

    # Wait for server to be ready (max 10 seconds)
    echo "Waiting for newsfeed-api to be ready..."
    for i in {1..20}; do
        if curl -s -f "$BASE_URL/items" > /dev/null 2>&1; then
            echo "✓ newsfeed-api started (PID: $NEWSFEED_PID)"
            break
        fi
        if [ $i -eq 20 ]; then
            echo "Error: newsfeed-api failed to start after 10 seconds"
            exit 1
        fi
        sleep 0.5
    done
fi

# Check if metadata-api is already running
if curl -s -f "http://localhost:8081/api/v1/meta/sources" > /dev/null 2>&1; then
    echo "✓ metadata-api already running at http://localhost:8081"
else
    # Start the metadata-api server
    echo "Starting metadata-api..."
    go run ./cmd/metadata-api > /dev/null 2>&1 &
    METADATA_PID=$!

    # Wait for server to be ready (max 10 seconds)
    echo "Waiting for metadata-api to be ready..."
    for i in {1..20}; do
        if curl -s -f "http://localhost:8081/api/v1/meta/sources" > /dev/null 2>&1; then
            echo "✓ metadata-api started (PID: $METADATA_PID)"
            break
        fi
        if [ $i -eq 20 ]; then
            echo "Error: metadata-api failed to start after 10 seconds"
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
