#!/bin/bash
# Test CLI: Storage error handling
# RFC 8, Section 6.1

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Create temporary test directory
TEMP_TEST_DIR=$(mktemp -d)
trap "rm -rf $TEMP_TEST_DIR" EXIT

# Test 1: File not found error -- metadata database doesn't exist
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/nonexistent/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/.news"

output=$(newsfed sources list 2>&1) || exit_code=$?
test_contains "Shows error for missing metadata database" "$output" "Error"
test_contains "Shows database error in message" "$output" "\(database\|open\)"
test_contains "Mentions file not found or no such file" "$output" "\(no such file\|not found\)"
test_exit_code "Returns non-zero exit code for missing database" "$exit_code" "1"

# Test 2: Permission error -- feed directory not readable
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/.news-noperm"

mkdir -p "$TEMP_TEST_DIR/.news-noperm"
chmod 000 "$TEMP_TEST_DIR/.news-noperm"

output=$(newsfed list 2>&1) || exit_code=$?
test_contains "Shows error for permission denied" "$output" "Error"
test_contains "Mentions permission or access issue" "$output" "permission denied"
test_exit_code "Returns non-zero exit code for permission error" "$exit_code" "1"

# Restore permissions for cleanup
chmod 755 "$TEMP_TEST_DIR/.news-noperm"

# Test 3: Not found error -- invalid item ID (verify error format per RFC 8 section 6.1)
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/.news"

output=$(newsfed show 99999999-9999-9999-9999-999999999999 2>&1) || exit_code=$?
test_contains "Shows friendly error for non-existent item" "$output" "Error"
test_contains "Mentions item not found" "$output" "not found"
test_exit_code "Returns non-zero exit code for non-existent item" "$exit_code" "1"

# Test 4: Not found error -- invalid source ID
output=$(newsfed sources show 99999999-9999-9999-9999-999999999999 2>&1) || exit_code=$?
test_contains "Shows friendly error for non-existent source" "$output" "Error"
test_contains "Mentions source not found or failure" "$output" "\(not found\|failed\)"
test_exit_code "Returns non-zero exit code for non-existent source" "$exit_code" "1"

# Test 5: Invalid UUID format provides helpful error
output=$(newsfed show invalid-uuid-format 2>&1) || exit_code=$?
test_contains "Shows error for invalid UUID format" "$output" "Error"
test_contains "Mentions invalid ID or UUID" "$output" "\(invalid\|UUID\)"
test_exit_code "Returns non-zero exit code for invalid UUID" "$exit_code" "1"

# Test 6: Error message clarity -- should not expose internal errors (RFC 8 section 6.1)
# Good error messages are user-friendly, not raw Go panic messages
output=$(newsfed sources list 2>&1 || true)
test_not_contains "No Go panic messages in output" "$output" "panic:"
test_not_contains "No Go stack traces in output" "$output" "goroutine"
test_not_contains "No raw pointer addresses in output" "$output" "0x[0-9a-f]\\{8,\\}"

exit $FAILED
