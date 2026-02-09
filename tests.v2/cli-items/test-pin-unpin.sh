#!/bin/bash
# Test CLI: newsfed pin and unpin commands
# RFC 8, Section 3.1.3

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Pin an unpinned item
output=$(newsfed pin 33333333-3333-3333-3333-333333333333 2>&1)
test_contains "Shows success message for pin" "$output" "Pinned item"
test_contains "Shows item title in pin message" "$output" "Unpinned Article for Testing"
newsfed pin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for successful pin" "$?" "0"

# Test 2: Verify item is pinned (check the file)
test_file_contains "Item file contains pinned_at field" \
    "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json" \
    "pinned_at"

# Test 3: Pin an already pinned item
output=$(newsfed pin 33333333-3333-3333-3333-333333333333 2>&1)
test_contains "Shows message for already pinned item" "$output" "already pinned"
newsfed pin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for already pinned item" "$?" "0"

# Test 4: Unpin a pinned item
output=$(newsfed unpin 33333333-3333-3333-3333-333333333333 2>&1)
test_contains "Shows success message for unpin" "$output" "Unpinned item"
test_contains "Shows item title in unpin message" "$output" "Unpinned Article for Testing"
newsfed unpin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for successful unpin" "$?" "0"

# Test 5: Verify item is unpinned (pinned_at should be null or absent)
# First check if file has pinned_at with null value or no pinned_at field
if grep -q '"pinned_at":null' "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json" || \
   ! grep -q '"pinned_at"' "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json"; then
    PASSED=$((PASSED + 1))
else
    echo "$SCRIPT_NAME: FAIL - Item file shows unpinned state"
    echo "  Expected pinned_at to be null or absent"
    FAILED=$((FAILED + 1))
fi

# Test 6: Unpin an already unpinned item
output=$(newsfed unpin 33333333-3333-3333-3333-333333333333 2>&1)
test_contains "Shows message for already unpinned item" "$output" "already unpinned"
newsfed unpin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for already unpinned item" "$?" "0"

# Test 7: Pin with invalid UUID returns error
output=$(newsfed pin invalid-uuid 2>&1) || true
test_contains "Shows error for invalid UUID in pin" "$output" "Error.*invalid"
newsfed pin invalid-uuid > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for invalid UUID in pin" "$exit_code" "1"

# Test 8: Unpin with invalid UUID returns error
output=$(newsfed unpin invalid-uuid 2>&1) || true
test_contains "Shows error for invalid UUID in unpin" "$output" "Error.*invalid"
newsfed unpin invalid-uuid > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for invalid UUID in unpin" "$exit_code" "1"

# Test 9: Pin non-existent item returns error
output=$(newsfed pin 99999999-9999-9999-9999-999999999999 2>&1) || true
test_contains "Shows error for non-existent item in pin" "$output" "Error.*not found"
newsfed pin 99999999-9999-9999-9999-999999999999 > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for non-existent item in pin" "$exit_code" "1"

# Test 10: Unpin non-existent item returns error
output=$(newsfed unpin 99999999-9999-9999-9999-999999999999 2>&1) || true
test_contains "Shows error for non-existent item in unpin" "$output" "Error.*not found"
newsfed unpin 99999999-9999-9999-9999-999999999999 > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for non-existent item in unpin" "$exit_code" "1"

# Test 11: Pin without arguments returns error
output=$(newsfed pin 2>&1) || true
test_contains "Shows error when ID missing in pin" "$output" "Error.*required"
newsfed pin > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code when no ID provided to pin" "$exit_code" "1"

# Test 12: Unpin without arguments returns error
output=$(newsfed unpin 2>&1) || true
test_contains "Shows error when ID missing in unpin" "$output" "Error.*required"
newsfed unpin > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code when no ID provided to unpin" "$exit_code" "1"

# Test 13: Pin and unpin maintain other item properties
# First, verify the summary is intact after pin/unpin operations
test_file_contains "Item properties preserved after pin/unpin" \
    "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json" \
    "This article will be used to test pin and unpin commands"

exit $FAILED
