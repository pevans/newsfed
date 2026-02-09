#!/bin/bash
# Test CLI: newsfed list (basic functionality)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: List with no arguments (default: shows recent + pinned items from past 3 days)
output=$(newsfed list 2>&1)
test_contains "Shows recent items" "$output" "Recent Article from Publisher A"
test_contains "Shows recent items from Publisher B" "$output" "Recent Article from Publisher B"
test_contains "Shows pinned recent item" "$output" "Recent Pinned Article"
test_contains "Shows old pinned item" "$output" "Old Pinned Article"
test_not_contains "Does not show old unpinned items" "$output" "Old Article from Publisher A"

# Test 2: List returns success exit code
newsfed list > /dev/null 2>&1
test_exit_code "Returns exit code 0" "$?" "0"

# Test 3: List with no items (empty feed)
# Save current feed and create empty one
mv "$TEST_DIR/.news" "$TEST_DIR/.news.backup"
mkdir -p "$TEST_DIR/.news"
output=$(newsfed list 2>&1)
# Should succeed with no output (or just headers)
test_exit_code "Returns exit code 0 for empty feed" "$?" "0"
# Restore feed
rm -rf "$TEST_DIR/.news"
mv "$TEST_DIR/.news.backup" "$TEST_DIR/.news"

# Test 4: List shows item titles
output=$(newsfed list 2>&1)
test_contains "Shows item titles" "$output" "Recent Article"

# Test 5: List --all flag shows all items including old ones
output=$(newsfed list --all 2>&1)
test_contains "Shows old article with --all" "$output" "Old Article from Publisher A"
test_contains "Shows recent article with --all" "$output" "Recent Article from Publisher A"

exit $FAILED
