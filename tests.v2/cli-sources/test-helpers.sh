#!/bin/bash
# Common test helpers for CLI tests

# Test counters
PASSED=0
FAILED=0

# Get the calling script name for error reporting
SCRIPT_NAME=$(basename "${BASH_SOURCE[1]}")

# Get test environment paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Export test environment variables
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

# Test if output contains expected string
test_contains() {
    local test_name="$1"
    local output="$2"
    local expected="$3"

    if echo "$output" | grep -q "$expected"; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected to find: $expected"
        echo "  Got: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test if output does not contain string
test_not_contains() {
    local test_name="$1"
    local output="$2"
    local unexpected="$3"

    if ! echo "$output" | grep -q "$unexpected"; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected NOT to find: $unexpected"
        echo "  Got: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test exit code
test_exit_code() {
    local test_name="$1"
    local actual="$2"
    local expected="$3"

    if [ "$actual" -eq "$expected" ]; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected exit code: $expected"
        echo "  Got: $actual"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test if output is empty
test_empty() {
    local test_name="$1"
    local output="$2"

    if [ -z "$output" ]; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected empty output"
        echo "  Got: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test if output is not empty
test_not_empty() {
    local test_name="$1"
    local output="$2"

    if [ -n "$output" ]; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected non-empty output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Extract UUID from output (first UUID found)
extract_uuid() {
    local output="$1"
    echo "$output" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1
}
