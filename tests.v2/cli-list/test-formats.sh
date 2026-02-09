#!/bin/bash
# Test CLI: newsfed list (output formats)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Default format is table
output=$(newsfed list --all 2>&1)
test_contains "Default shows table format" "$output" "Showing"
test_contains "Table format has publisher" "$output" "Publisher"

# Test 2: JSON format flag
output=$(newsfed list --format=json --all 2>&1)
test_contains "JSON format has items array" "$output" "\"items\":"
test_contains "JSON format has total count" "$output" "\"total\":"

# Test 3: JSON format structure validation
output=$(newsfed list --format=json --all 2>&1)
# Validate it's valid JSON by parsing with jq (if available) or python
if command -v jq > /dev/null 2>&1; then
    echo "$output" | jq -e '.items' > /dev/null 2>&1
    test_exit_code "JSON is valid and has items array" "$?" "0"

    echo "$output" | jq -e '.total' > /dev/null 2>&1
    test_exit_code "JSON has total field" "$?" "0"
else
    python3 -c "import json, sys; data = json.loads(sys.stdin.read()); assert 'items' in data; assert 'total' in data" <<< "$output" 2>&1
    test_exit_code "JSON is valid (via python)" "$?" "0"
fi

# Test 4: JSON format includes required fields
output=$(newsfed list --format=json --all --limit=1 2>&1)
test_contains "JSON items have id" "$output" "\"id\":"
test_contains "JSON items have title" "$output" "\"title\":"
test_contains "JSON items have url" "$output" "\"url\":"
test_contains "JSON items have published_at" "$output" "\"published_at\":"
test_contains "JSON items have discovered_at" "$output" "\"discovered_at\":"

# Test 5: JSON format respects filters
output=$(newsfed list --format=json --pinned --all 2>&1)
# Should only have 2 pinned items
if command -v jq > /dev/null 2>&1; then
    count=$(echo "$output" | jq '.items | length')
    if [ "$count" -eq 2 ]; then
        printf "\033[32m✓\033[0m %s\n" "JSON format respects --pinned filter"
        PASSED=$((PASSED + 1))
    else
        printf "\033[31m✗\033[0m %s\n" "JSON format filter failed"
        echo "  Expected 2 items, got $count"
        FAILED=$((FAILED + 1))
    fi
else
    test_contains "JSON has limited items" "$output" "\"pinned_at\":"
fi

# Test 6: Compact format flag
output=$(newsfed list --format=compact --all 2>&1)
test_contains "Compact format shows ID prefix" "$output" "..."
test_contains "Compact format shows title" "$output" "Article"
test_contains "Compact format shows publisher in parens" "$output" ")"

# Test 7: Compact format is actually compact (one line per item)
output=$(newsfed list --format=compact --all 2>&1)
line_count=$(echo "$output" | wc -l | tr -d ' ')
# Should have 6 items = 6 lines
if [ "$line_count" -eq 6 ]; then
    printf "\033[32m✓\033[0m %s\n" "Compact format is one line per item"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Compact format line count wrong"
    echo "  Expected 6 lines, got $line_count"
    FAILED=$((FAILED + 1))
fi

# Test 8: Compact format respects filters
output=$(newsfed list --format=compact --publisher="Publisher A" --all 2>&1)
line_count=$(echo "$output" | wc -l | tr -d ' ')
# Should have 2 Publisher A items
if [ "$line_count" -eq 2 ]; then
    printf "\033[32m✓\033[0m %s\n" "Compact format respects filters"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Compact format filter failed"
    echo "  Expected 2 lines, got $line_count"
    FAILED=$((FAILED + 1))
fi

# Test 9: Invalid format returns error
output=$(newsfed list --format=invalid 2>&1 || true)
test_contains "Invalid format returns error" "$output" "Error.*invalid format"

# Test 10: Format option works with other flags
output=$(newsfed list --format=json --limit=2 --all 2>&1)
if command -v jq > /dev/null 2>&1; then
    count=$(echo "$output" | jq '.items | length')
    if [ "$count" -eq 2 ]; then
        printf "\033[32m✓\033[0m %s\n" "JSON format works with --limit"
        PASSED=$((PASSED + 1))
    else
        printf "\033[31m✗\033[0m %s\n" "JSON format with --limit failed"
        echo "  Expected 2 items, got $count"
        FAILED=$((FAILED + 1))
    fi
else
    test_contains "JSON has items" "$output" "\"items\":"
fi

# Test 11: Compact format handles items without publisher
output=$(newsfed list --format=compact --all 2>&1)
test_contains "Compact shows Unknown for missing publisher" "$output" "(Unknown)"

# Test 12: JSON format includes pinned_at when present
output=$(newsfed list --format=json --pinned --all 2>&1)
test_contains "JSON includes pinned_at" "$output" "\"pinned_at\":"

exit $FAILED
