#!/bin/bash
# Test CLI: newsfed list (filtering)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Filter by --pinned
output=$(newsfed list --pinned --all 2>&1)
test_contains "Shows pinned items" "$output" "Old Pinned Article"
test_contains "Shows recent pinned item" "$output" "Recent Pinned Article"
test_not_contains "Does not show unpinned items" "$output" "Recent Article from Publisher A"

# Test 2: Filter by --unpinned
output=$(newsfed list --unpinned --all 2>&1)
test_contains "Shows unpinned recent item" "$output" "Recent Article from Publisher A"
test_contains "Shows unpinned old item" "$output" "Old Article from Publisher A"
test_not_contains "Does not show pinned items" "$output" "Old Pinned Article"
test_not_contains "Does not show recent pinned" "$output" "Recent Pinned Article"

# Test 3: Filter by --publisher
output=$(newsfed list --publisher="Publisher A" --all 2>&1)
test_contains "Shows Publisher A items" "$output" "Recent Article from Publisher A"
test_contains "Shows old Publisher A item" "$output" "Old Article from Publisher A"
test_not_contains "Does not show Publisher B" "$output" "Publisher B"
test_not_contains "Does not show Publisher C" "$output" "Publisher C"

# Test 4: Filter by --publisher with partial match
output=$(newsfed list --publisher="publisher b" --all 2>&1)
test_contains "Case-insensitive publisher filter" "$output" "Publisher B"
test_not_contains "Does not show other publishers" "$output" "Publisher A"

# Test 5: Filter by --since duration (use 13h to catch 12h-old item but not 1d-old)
output=$(newsfed list --since=13h 2>&1)
test_contains "Shows very recent item from last 13h" "$output" "Very Recent Article"
test_not_contains "Does not show older items from 1d ago" "$output" "Recent Article from Publisher A"

# Test 6: Filter by --since with days (2d should catch 1d-old items but not 10d-old)
output=$(newsfed list --since=2d 2>&1)
test_contains "Shows items from last 2 days" "$output" "Recent Article from Publisher A"
test_not_contains "Does not show items older than 2 days" "$output" "Old Article from Publisher A"

# Test 7: Combine --pinned and --publisher
output=$(newsfed list --pinned --publisher="Publisher B" --all 2>&1)
test_contains "Shows pinned Publisher B item" "$output" "Recent Pinned Article"
test_not_contains "Does not show Publisher C pinned" "$output" "Publisher C"
test_not_contains "Does not show unpinned Publisher B" "$output" "Recent Article from Publisher B"

exit $FAILED
