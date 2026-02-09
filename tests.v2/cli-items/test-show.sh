#!/bin/bash
# Test CLI: newsfed show (view individual items)
# RFC 8, Section 3.1.2

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Show item with valid ID displays all metadata
output=$(newsfed show 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Displays item title" "$output" "Test Article for Show Command"
test_contains "Displays publisher" "$output" "Publisher:"
test_contains "Displays publisher name" "$output" "Test Publisher"
test_contains "Displays authors" "$output" "Authors:"
test_contains "Displays author names" "$output" "Alice Smith"
test_contains "Displays published date" "$output" "Published:"
test_contains "Displays discovered date" "$output" "Discovered:"
test_contains "Displays URL" "$output" "https://example.com/test-article"
test_contains "Displays summary" "$output" "This is a detailed summary"
test_contains "Displays ID" "$output" "11111111-1111-1111-1111-111111111111"

# Test 2: Show returns success exit code
newsfed show 11111111-1111-1111-1111-111111111111 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for valid show" "$?" "0"

# Test 3: Show displays pinned status for unpinned item
output=$(newsfed show 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Shows unpinned status" "$output" "Pinned:.*No"

# Test 4: Show displays pinned status for pinned item
output=$(newsfed show 22222222-2222-2222-2222-222222222222 2>&1)
test_contains "Shows pinned indicator" "$output" "Pinned:.*📌"
test_not_contains "Does not show 'No' for pinned item" "$output" "Pinned:.*No"

# Test 5: Show with invalid UUID returns error
output=$(newsfed show invalid-uuid 2>&1) || true
test_contains "Shows error for invalid UUID" "$output" "Error.*invalid"
newsfed show invalid-uuid > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for invalid UUID" "$exit_code" "1"

# Test 6: Show with non-existent ID returns error
output=$(newsfed show 99999999-9999-9999-9999-999999999999 2>&1) || true
test_contains "Shows error for non-existent item" "$output" "Error.*not found"
newsfed show 99999999-9999-9999-9999-999999999999 > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for non-existent item" "$exit_code" "1"

# Test 7: Show without arguments returns error
output=$(newsfed show 2>&1) || true
test_contains "Shows error when ID missing" "$output" "Error.*required"
newsfed show > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code when no ID provided" "$exit_code" "1"

# Test 8: Show item without publisher displays "Unknown"
output=$(newsfed show 44444444-4444-4444-4444-444444444444 2>&1)
test_contains "Displays Unknown for missing publisher" "$output" "Publisher:.*Unknown"

# Test 9: Show item formats summary text (no test for wrapping, just that summary appears)
output=$(newsfed show 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Displays summary section" "$output" "Summary:"
test_contains "Displays summary content" "$output" "multiple sentences"

exit $FAILED
