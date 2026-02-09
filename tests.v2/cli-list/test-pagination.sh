#!/bin/bash
# Test CLI: newsfed list (pagination)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Default limit is 20 (we have 6 items, so all should show)
output=$(newsfed list --all 2>&1)
# Count how many article titles appear
count=$(count_occurrences "$output" "Article")
if [ "$count" -eq 6 ]; then
    printf "\033[32m✓\033[0m %s\n" "Default shows all items when under limit"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Default limit failed"
    echo "  Expected 6 items, got $count"
    FAILED=$((FAILED + 1))
fi

# Test 2: Limit to 2 items
output=$(newsfed list --all --limit=2 2>&1)
count=$(count_occurrences "$output" "Article")
if [ "$count" -eq 2 ]; then
    printf "\033[32m✓\033[0m %s\n" "Limit=2 shows 2 items"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Limit=2 failed"
    echo "  Expected 2 items, got $count"
    FAILED=$((FAILED + 1))
fi

# Test 3: Limit to 1 item
output=$(newsfed list --all --limit=1 2>&1)
count=$(count_occurrences "$output" "Article")
if [ "$count" -eq 1 ]; then
    printf "\033[32m✓\033[0m %s\n" "Limit=1 shows 1 item"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Limit=1 failed"
    echo "  Expected 1 item, got $count"
    FAILED=$((FAILED + 1))
fi

# Test 4: Offset skips items
output=$(newsfed list --all --sort=published --offset=1 --limit=1 2>&1)
# With offset=1, we should skip the first (most recent) item
test_not_contains "Offset skips first item" "$output" "Very Recent Article"

# Test 5: Offset + limit pagination
# Get first page (items 0-1)
page1=$(newsfed list --all --sort=published --limit=2 --offset=0 2>&1)
# Get second page (items 2-3)
page2=$(newsfed list --all --sort=published --limit=2 --offset=2 2>&1)
# Pages should have different content
if [ "$page1" != "$page2" ]; then
    printf "\033[32m✓\033[0m %s\n" "Pagination works with offset+limit"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Pagination failed"
    echo "  Page 1 and Page 2 are identical"
    FAILED=$((FAILED + 1))
fi

# Test 6: Offset beyond available items returns empty
output=$(newsfed list --all --offset=100 2>&1)
count=$(count_occurrences "$output" "Article")
if [ "$count" -eq 0 ]; then
    printf "\033[32m✓\033[0m %s\n" "Offset beyond items returns empty"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Offset beyond items failed"
    echo "  Expected 0 items, got $count"
    FAILED=$((FAILED + 1))
fi

# Test 7: Limit=0 shows no items
output=$(newsfed list --all --limit=0 2>&1)
count=$(count_occurrences "$output" "Article")
if [ "$count" -eq 0 ]; then
    printf "\033[32m✓\033[0m %s\n" "Limit=0 shows no items"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Limit=0 failed"
    echo "  Expected 0 items, got $count"
    FAILED=$((FAILED + 1))
fi

exit $FAILED
