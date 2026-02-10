#!/bin/bash
# Test CLI: newsfed doctor (storage health check)
# RFC 8, Section 6.3

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Create a temporary test directory for each test
TEMP_TEST_DIR=$(mktemp -d)
trap "rm -rf $TEMP_TEST_DIR" EXIT

# Test 1: Doctor detects missing storage
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test1/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test1/.news"

output=$(newsfed doctor 2>&1) || exit_code=$?

test_exit_code "Returns non-zero exit code for missing storage" "$exit_code" "1"
test_contains "Reports missing metadata database" "$output" "Database file does not exist"
test_contains "Reports missing feed storage" "$output" "Storage directory does not exist"
test_contains "Suggests running init" "$output" "newsfed init"

# Test 2: Doctor confirms healthy storage
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test2/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test2/.news"

# Initialize storage first
newsfed init > /dev/null 2>&1

output=$(newsfed doctor 2>&1)
exit_code=$?

test_exit_code "Returns exit code 0 for healthy storage" "$exit_code" "0"
test_contains "Reports database accessible" "$output" "Database is accessible"
test_contains "Reports storage accessible" "$output" "Storage directory is accessible"

# Test 3: Doctor verbose mode shows details
output=$(newsfed doctor --verbose 2>&1)

test_contains "Verbose shows permissions" "$output" "Permissions:"
test_contains "Verbose shows source count" "$output" "Sources configured:"
test_contains "Verbose shows item count" "$output" "News items stored:"

# Test 4: Doctor detects permission issues
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test4/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test4/.news"

# Initialize storage
newsfed init > /dev/null 2>&1

# Make metadata file world-readable (too permissive)
chmod 644 "$NEWSFED_METADATA_DSN"

output=$(newsfed doctor 2>&1)
exit_code=$?

test_contains "Warns about permissive permissions" "$output" "overly permissive permissions"
test_contains "Suggests chmod command" "$output" "chmod"
# Should still exit 0 (warning, not error)
test_exit_code "Returns exit code 0 for warnings only" "$exit_code" "0"

# Test 5: Doctor checks both metadata and feed storage
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test5/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test5/.news"

# Create only metadata, not feed
mkdir -p "$TEMP_TEST_DIR/test5"
touch "$NEWSFED_METADATA_DSN"

output=$(newsfed doctor 2>&1) || exit_code=$?

test_contains "Reports metadata status" "$output" "Metadata Database:"
test_contains "Reports feed status" "$output" "Feed Storage:"
test_contains "Shows paths being checked" "$output" "Path:"
test_exit_code "Returns non-zero when feed missing" "$exit_code" "1"

# Test 6: Doctor provides actionable suggestions
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test6/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test6/.news"

output=$(newsfed doctor 2>&1) || true

test_contains "Provides clear error indicators" "$output" "✗"
test_contains "Suggests init for missing storage" "$output" "Run 'newsfed init'"

# Test 7: Doctor verbose flag works without errors
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test7/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test7/.news"

newsfed init > /dev/null 2>&1

# Fix permissions to avoid warnings
chmod 600 "$NEWSFED_METADATA_DSN"

output=$(newsfed doctor --verbose 2>&1)
exit_code=$?

test_exit_code "Verbose mode succeeds with healthy storage" "$exit_code" "0"
test_contains "Shows all checks passed" "$output" "All checks passed"

exit $FAILED
