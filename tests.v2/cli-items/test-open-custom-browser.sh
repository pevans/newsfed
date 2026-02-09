#!/bin/bash
# Test CLI: newsfed open with custom browser configuration
# RFC 8, Section 4.3

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Determine expected default browser command based on platform
case "$(uname -s)" in
    Darwin)
        DEFAULT_BROWSER_CMD="open"
        ;;
    Linux)
        DEFAULT_BROWSER_CMD="xdg-open"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        DEFAULT_BROWSER_CMD="cmd /c start"
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

# Clean up: ensure no config exists at the start
if [ -f "$NEWSFED_METADATA_DSN" ]; then
    sqlite3 "$NEWSFED_METADATA_DSN" "DELETE FROM config WHERE key = 'browser_command';" 2>/dev/null || true
fi

# Test 1: Verify default browser command when no config is set
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Uses default browser when no config" "$output" "$DEFAULT_BROWSER_CMD"

# Test 2: Create config table and set custom browser command
# First create the config table (simulating what ConfigStore.initSchema does)
sqlite3 "$NEWSFED_METADATA_DSN" "CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL);"
# Now insert the custom browser command
sqlite3 "$NEWSFED_METADATA_DSN" "INSERT OR REPLACE INTO config (key, value) VALUES ('browser_command', 'firefox');"

# Test 3: Verify custom browser command is used
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Uses custom browser from config" "$output" "firefox"
test_contains "Shows correct URL with custom browser" "$output" "https://example.com/test-article"

# Test 4: Verify exact output format with custom browser
expected_output="firefox https://example.com/test-article"
if [ "$output" = "$expected_output" ]; then
    PASSED=$((PASSED + 1))
else
    echo "$SCRIPT_NAME: FAIL - Custom browser echo output format matches expected"
    echo "  Expected: $expected_output"
    echo "  Got: $output"
    FAILED=$((FAILED + 1))
fi

# Test 5: Test with different item to ensure URL changes but browser stays custom
output=$(newsfed open --echo 22222222-2222-2222-2222-222222222222 2>&1)
test_contains "Uses custom browser for different item" "$output" "firefox"
test_contains "Shows correct URL for pinned item" "$output" "https://example.com/pinned-article"

# Test 6: Set a different custom browser command
sqlite3 "$NEWSFED_METADATA_DSN" "INSERT OR REPLACE INTO config (key, value) VALUES ('browser_command', 'chromium');"

# Test 7: Verify new custom browser command is used
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Uses updated custom browser from config" "$output" "chromium"
test_contains "Shows correct URL with updated browser" "$output" "https://example.com/test-article"

# Test 8: Verify exact output format with updated browser
expected_output="chromium https://example.com/test-article"
if [ "$output" = "$expected_output" ]; then
    PASSED=$((PASSED + 1))
else
    echo "$SCRIPT_NAME: FAIL - Updated browser echo output format matches expected"
    echo "  Expected: $expected_output"
    echo "  Got: $output"
    FAILED=$((FAILED + 1))
fi

# Test 9: Clear custom browser config and verify fallback to default
sqlite3 "$NEWSFED_METADATA_DSN" "DELETE FROM config WHERE key = 'browser_command';"
output=$(newsfed open --echo 11111111-1111-1111-1111-111111111111 2>&1)
test_contains "Falls back to default browser after clearing config" "$output" "$DEFAULT_BROWSER_CMD"

exit $FAILED
