#!/bin/bash
# Test CLI: newsfed list (sorting)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Default sort is by published date (most recent first)
output=$(newsfed list --all 2>&1)
# Extract titles in order they appear
first_title=$(echo "$output" | grep -m1 "Article" | head -1)
test_contains "Default sort shows recent first" "$first_title" "Recent\|Very Recent"

# Test 2: Sort by --sort=published
output=$(newsfed list --all --sort=published 2>&1)
# Very Recent (12h old) should appear before Recent (1d old)
very_recent_line=$(echo "$output" | grep -n "Very Recent Article" | cut -d: -f1)
recent_line=$(echo "$output" | grep -n "Recent Article from Publisher A" | cut -d: -f1)
if [ ! -z "$very_recent_line" ] && [ ! -z "$recent_line" ] && [ "$very_recent_line" -lt "$recent_line" ]; then
    printf "\033[32m✓\033[0m %s\n" "Sort by published date works"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Sort by published date failed"
    echo "  Very recent line: $very_recent_line, Recent line: $recent_line"
    FAILED=$((FAILED + 1))
fi

# Test 3: Sort by --sort=discovered
output=$(newsfed list --all --sort=discovered 2>&1)
# Very Recent (12h ago) should appear before Old (10d ago)
very_recent_line=$(echo "$output" | grep -n "Very Recent Article" | cut -d: -f1)
old_line=$(echo "$output" | grep -n "Old Article from Publisher A" | cut -d: -f1)
if [ ! -z "$very_recent_line" ] && [ ! -z "$old_line" ] && [ "$very_recent_line" -lt "$old_line" ]; then
    printf "\033[32m✓\033[0m %s\n" "Sort by discovered date works"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Sort by discovered date failed"
    FAILED=$((FAILED + 1))
fi

# Test 4: Sort by --sort=pinned (pinned items first, sorted by pinned time)
output=$(newsfed list --all --sort=pinned 2>&1)
# Recent Pinned (2h ago) should appear before Old Pinned (1d ago)
# And both should appear before unpinned items
recent_pinned_line=$(echo "$output" | grep -n "Recent Pinned Article" | cut -d: -f1)
old_pinned_line=$(echo "$output" | grep -n "Old Pinned Article" | cut -d: -f1)
unpinned_line=$(echo "$output" | grep -n "Recent Article from Publisher A" | cut -d: -f1)

if [ ! -z "$recent_pinned_line" ] && [ ! -z "$old_pinned_line" ] && [ ! -z "$unpinned_line" ] && \
   [ "$recent_pinned_line" -lt "$old_pinned_line" ] && [ "$old_pinned_line" -lt "$unpinned_line" ]; then
    printf "\033[32m✓\033[0m %s\n" "Sort by pinned works"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Sort by pinned failed"
    echo "  Recent pinned: $recent_pinned_line, Old pinned: $old_pinned_line, Unpinned: $unpinned_line"
    FAILED=$((FAILED + 1))
fi

# Test 5: Invalid sort option returns error
output=$(newsfed list --sort=invalid 2>&1 || true)
test_contains "Invalid sort returns error" "$output" "Error.*invalid sort option"

exit $FAILED
