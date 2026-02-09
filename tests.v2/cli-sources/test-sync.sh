#!/bin/bash
# Test CLI: newsfed sync (manual source synchronization)
# RFC 8, Section 3.2.7

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Clean up all existing sources first to ensure clean state
# Get list of all sources and delete them
SOURCE_IDS=$(newsfed sources list 2>&1 | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' || true)
for source_id in $SOURCE_IDS; do
    newsfed sources delete "$source_id" > /dev/null 2>&1 || true
done

# Test 1: Sync with no sources shows zero results
output=$(newsfed sync 2>&1)
test_contains "Shows sync completed message" "$output" "Sync completed"
test_contains "Shows zero sources synced" "$output" "Sources synced: 0"

# Test 2: Sync returns exit code 0 when no sources exist
newsfed sync > /dev/null 2>&1
test_exit_code "Returns exit code 0 with no sources" "$?" "0"

# Test 3: Add a test RSS source for sync testing
# Note: This will fail to fetch since it's a fake URL, but we can test the sync behavior
output=$(newsfed sources add --type=rss --url="https://fake-test-feed.example.com/rss.xml" --name="Test Sync Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Test 4: Sync all sources attempts to sync the added source
output=$(newsfed sync 2>&1) || true  # May fail due to network error, that's OK
test_contains "Shows syncing message" "$output" "Syncing all enabled sources"
test_contains "Shows sync completed" "$output" "Sync completed"

# Test 5: Sync with specific source ID
output=$(newsfed sync "$SOURCE_ID" 2>&1) || true  # May fail due to network error
test_contains "Shows source name when syncing specific source" "$output" "Syncing source"
test_contains "Shows test source name" "$output" "Test Sync Source"

# Test 6: Sync with --verbose flag shows more details
output=$(newsfed sync --verbose 2>&1) || true
test_contains "Verbose mode shows sync completed" "$output" "Sync completed"
# Verbose mode may show errors if fetch failed
if echo "$output" | grep -q "Sources failed: [1-9]"; then
    test_contains "Verbose mode may show errors section" "$output" "Errors:"
fi

# Test 7: Sync with invalid source ID returns error
output=$(newsfed sync invalid-uuid 2>&1) || true
test_contains "Shows error for invalid source ID" "$output" "Error.*invalid"
newsfed sync invalid-uuid > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for invalid source ID" "$exit_code" "1"

# Test 8: Sync with non-existent source ID returns error
output=$(newsfed sync 99999999-9999-9999-9999-999999999999 2>&1) || true
test_contains "Shows error for non-existent source" "$output" "Error"
newsfed sync 99999999-9999-9999-9999-999999999999 > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for non-existent source" "$exit_code" "1"

# Test 9: Sync disabled source (disable the source first)
newsfed sources disable "$SOURCE_ID" > /dev/null 2>&1
output=$(newsfed sync 2>&1)
test_contains "Sync with only disabled sources shows zero synced" "$output" "Sources synced: 0"

# Re-enable for next test
newsfed sources enable "$SOURCE_ID" > /dev/null 2>&1

# Test 10: Sync exit code is non-zero when sources fail to fetch
newsfed sync > /dev/null 2>&1 || exit_code=$?
# Should be 1 because the fake URL will fail
test_exit_code "Returns non-zero exit code when sync fails" "$exit_code" "1"

# Test 11: Sync output includes summary stats
output=$(newsfed sync 2>&1) || true
test_contains "Shows sources synced count" "$output" "Sources synced:"
test_contains "Shows sources failed count" "$output" "Sources failed:"
test_contains "Shows items discovered count" "$output" "Items discovered:"

# Clean up test source
newsfed sources delete "$SOURCE_ID" > /dev/null 2>&1 || true

exit $FAILED
