#!/bin/bash
# Common helper functions for CLI list tests

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Track test results
PASSED=0
FAILED=0

# Test if output contains a substring
test_contains() {
    local test_name="$1"
    local output="$2"
    local expected="$3"

    if echo "$output" | grep -q "$expected"; then
        printf "\033[32m✓\033[0m %s\n" "$test_name"
        PASSED=$((PASSED + 1))
        return 0
    else
        printf "\033[31m✗\033[0m %s\n" "$test_name"
        echo "  Expected to find: $expected"
        echo "  Got: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test if output does NOT contain a substring
test_not_contains() {
    local test_name="$1"
    local output="$2"
    local unexpected="$3"

    if echo "$output" | grep -q "$unexpected"; then
        printf "\033[31m✗\033[0m %s\n" "$test_name"
        echo "  Expected NOT to find: $unexpected"
        echo "  Got: $output"
        FAILED=$((FAILED + 1))
        return 1
    else
        printf "\033[32m✓\033[0m %s\n" "$test_name"
        PASSED=$((PASSED + 1))
        return 0
    fi
}

# Count lines in output
count_lines() {
    echo "$1" | grep -c "^" || echo "0"
}

# Count occurrences of a pattern
count_occurrences() {
    local output="$1"
    local pattern="$2"
    local count=$(echo "$output" | grep -c "$pattern" || echo "0")
    echo "$count" | tr -d '\n'
}

# Test that line count matches expected
test_line_count() {
    local test_name="$1"
    local output="$2"
    local expected_count="$3"

    local actual_count=$(count_lines "$output")

    if [ "$actual_count" -eq "$expected_count" ]; then
        printf "\033[32m✓\033[0m %s\n" "$test_name"
        PASSED=$((PASSED + 1))
        return 0
    else
        printf "\033[31m✗\033[0m %s\n" "$test_name"
        echo "  Expected $expected_count lines, got $actual_count"
        echo "  Output: $output"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test exit code
test_exit_code() {
    local test_name="$1"
    local actual_code="$2"
    local expected_code="$3"

    if [ "$actual_code" -eq "$expected_code" ]; then
        printf "\033[32m✓\033[0m %s\n" "$test_name"
        PASSED=$((PASSED + 1))
        return 0
    else
        printf "\033[31m✗\033[0m %s\n" "$test_name"
        echo "  Expected exit code $expected_code, got $actual_code"
        FAILED=$((FAILED + 1))
        return 1
    fi
}
