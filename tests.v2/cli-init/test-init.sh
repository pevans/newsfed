#!/bin/bash
# Test CLI: newsfed init (storage initialization)
# RFC 8, Section 6.3

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Create a temporary test directory for each test
TEMP_TEST_DIR=$(mktemp -d)
trap "rm -rf $TEMP_TEST_DIR" EXIT

# Test 1: Initialize fresh storage (no existing files)
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test1/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test1/.news"

output=$(newsfed init 2>&1)
exit_code=$?

test_exit_code "Returns exit code 0 for successful init" "$exit_code" "0"
test_contains "Shows success message" "$output" "Storage initialized successfully"
test_contains "Shows metadata database created" "$output" "Metadata database.*metadata.db"
test_contains "Shows feed storage created" "$output" "Feed storage.*\.news"
test_file_exists "Creates metadata database file" "$NEWSFED_METADATA_DSN"
test_dir_exists "Creates feed storage directory" "$NEWSFED_FEED_DSN"

# Test 2: Initialize when storage already exists
output=$(newsfed init 2>&1)
exit_code=$?

test_exit_code "Returns exit code 0 when already initialized" "$exit_code" "0"
test_contains "Reports storage already exists" "$output" "already exists"
test_contains "Suggests using doctor command" "$output" "newsfed doctor"

# Test 3: Initialize with nested directories
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test3/data/db/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test3/data/feed/.news"

output=$(newsfed init 2>&1)
exit_code=$?

test_exit_code "Returns exit code 0 for nested directories" "$exit_code" "0"
test_file_exists "Creates nested metadata database" "$NEWSFED_METADATA_DSN"
test_dir_exists "Creates nested feed storage" "$NEWSFED_FEED_DSN"

# Test 4: Force reinitialization
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test4/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test4/.news"

# Initialize first time
newsfed init > /dev/null 2>&1

# Force reinit
output=$(newsfed init --force 2>&1)
exit_code=$?

test_exit_code "Returns exit code 0 for force reinit" "$exit_code" "0"
test_contains "Force reinit succeeds" "$output" "Storage initialized successfully"

# Test 5: Verify directory permissions (should be 700 per RFC 8 section 8.1)
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test5/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test5/.news"

newsfed init > /dev/null 2>&1

# Check feed directory permissions (should be 700 = drwx------)
perm=$(stat -f "%Lp" "$NEWSFED_FEED_DSN" 2>/dev/null || stat -c "%a" "$NEWSFED_FEED_DSN" 2>/dev/null)
if [ "$perm" = "700" ]; then
    PASSED=$((PASSED + 1))
else
    echo "$SCRIPT_NAME: FAIL - Feed directory has correct permissions (700)"
    echo "  Expected: 700"
    echo "  Got: $perm"
    FAILED=$((FAILED + 1))
fi

# Test 6: Init provides helpful next steps
export NEWSFED_METADATA_DSN="$TEMP_TEST_DIR/test6/metadata.db"
export NEWSFED_FEED_DSN="$TEMP_TEST_DIR/test6/.news"

output=$(newsfed init 2>&1)

test_contains "Suggests adding sources" "$output" "newsfed sources add"
test_contains "Suggests checking health" "$output" "newsfed doctor"

exit $FAILED
