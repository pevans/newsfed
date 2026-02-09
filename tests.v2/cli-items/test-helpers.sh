#!/bin/bash
# Common test helpers for CLI item tests

# Test counters
PASSED=0
FAILED=0

# Get the calling script name for error reporting
SCRIPT_NAME=$(basename "${BASH_SOURCE[1]}")

# Get test environment paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Export test environment variables
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

# Check if a file exists
test_file_exists() {
    local test_name="$1"
    local file_path="$2"

    if [ -f "$file_path" ]; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected file to exist: $file_path"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Check if a file contains a substring
test_file_contains() {
    local test_name="$1"
    local file_path="$2"
    local expected="$3"

    if [ ! -f "$file_path" ]; then
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  File does not exist: $file_path"
        FAILED=$((FAILED + 1))
        return 1
    fi

    if grep -q "$expected" "$file_path"; then
        PASSED=$((PASSED + 1))
        return 0
    else
        echo "$SCRIPT_NAME: FAIL - $test_name"
        echo "  Expected to find in file: $expected"
        echo "  File: $file_path"
        FAILED=$((FAILED + 1))
        return 1
    fi
}
