#!/bin/bash
# Test CLI: newsfed open (open item URL in browser)
# RFC 8, Section 3.1.4

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Determine expected browser command based on platform
case "$(uname -s)" in
    Darwin)
        BROWSER_CMD="open"
        ;;
    Linux)
        BROWSER_CMD="xdg-open"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        BROWSER_CMD="cmd /c start"
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

# Test 1: Open with --echo flag shows correct command for item
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Shows browser command" "$output" "$BROWSER_CMD"
test_contains "Shows correct URL" "$output" "https://example.com/test-article"

# Test 2: Open --echo returns success exit code
newsfed open --echo 11111111-1111-1111-1111-111111111111 > /dev/null 2>&1
test_exit_code "Returns exit code 0 for echo mode" "$?" "0"

# Test 3: Open --echo with different item shows different URL
output=$(newsfed open --echo 22222222-2222-2222-2222-222222222222 2>&1)
test_contains "Shows browser command for pinned item" "$output" "$BROWSER_CMD"
test_contains "Shows correct URL for pinned item" "$output" "https://example.com/pinned-article"

# Test 4: Open --echo with item without publisher works
output=$(newsfed open --echo 44444444-4444-4444-4444-444444444444 2>&1)
test_contains "Shows browser command for item without metadata" "$output" "$BROWSER_CMD"
test_contains "Shows correct URL for item without metadata" "$output" "https://example.com/minimal"

# Test 5: Open with invalid UUID returns error
output=$(newsfed open --echo invalid-uuid 2>&1) || true
test_contains "Shows error for invalid UUID" "$output" "Error.*invalid"
newsfed open --echo invalid-uuid > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for invalid UUID" "$exit_code" "1"

# Test 6: Open with non-existent ID returns error
output=$(newsfed open --echo 99999999-9999-9999-9999-999999999999 2>&1) || true
test_contains "Shows error for non-existent item" "$output" "Error.*not found"
newsfed open --echo 99999999-9999-9999-9999-999999999999 > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code for non-existent item" "$exit_code" "1"

# Test 7: Open without arguments returns error
output=$(newsfed open --echo 2>&1) || true
test_contains "Shows error when ID missing" "$output" "Error.*required"
newsfed open --echo > /dev/null 2>&1 || exit_code=$?
test_exit_code "Returns non-zero exit code when no ID provided" "$exit_code" "1"

# Test 8: Open shows correct usage in error message
output=$(newsfed open 2>&1) || true
test_contains "Shows usage message" "$output" "Usage:.*newsfed open"

# Test 9: Verify echo output format is exactly "BROWSER_CMD URL"
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
# The output should be exactly the browser command followed by the URL
expected_output="${BROWSER_CMD} https://example.com/test-article"
if [ "$output" = "$expected_output" ]; then
    PASSED=$((PASSED + 1))
else
    echo "$SCRIPT_NAME: FAIL - Echo output format matches expected"
    echo "  Expected: $expected_output"
    echo "  Got: $output"
    FAILED=$((FAILED + 1))
fi

exit $FAILED
