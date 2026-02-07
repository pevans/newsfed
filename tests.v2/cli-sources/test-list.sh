#!/bin/bash
# Test CLI: newsfed sources list

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Setup: Add some test sources
newsfed sources add --type=rss --url=https://example.com/list-test-1.xml --name="List Test Feed 1" > /dev/null 2>&1
newsfed sources add --type=atom --url=https://example.com/list-test-2.xml --name="List Test Feed 2" > /dev/null 2>&1
newsfed sources add --type=rss --url=https://example.com/list-test-3.xml --name="List Test Feed 3" > /dev/null 2>&1

# Test 1: List all sources
output=$(newsfed sources list 2>&1)
test_contains "Shows first source" "$output" "List Test Feed 1"
test_contains "Shows second source" "$output" "List Test Feed 2"
test_contains "Shows third source" "$output" "List Test Feed 3"

# Test 2: Output includes table header
output=$(newsfed sources list 2>&1)
test_contains "Shows ID header" "$output" "ID"
test_contains "Shows TYPE header" "$output" "TYPE"
test_contains "Shows NAME header" "$output" "NAME"
test_contains "Shows URL header" "$output" "URL"

# Test 3: Shows correct source types
output=$(newsfed sources list 2>&1)
test_contains "Shows rss type" "$output" "rss"
test_contains "Shows atom type" "$output" "atom"

# Test 4: Shows source URLs
output=$(newsfed sources list 2>&1)
test_contains "Shows first URL" "$output" "https://example.com/list-test-1.xml"
test_contains "Shows second URL" "$output" "https://example.com/list-test-2.xml"

# Test 5: Empty database shows appropriate message
rm -f "$NEWSFED_METADATA_DSN"
output=$(newsfed sources list 2>&1)
test_contains "Shows no sources message" "$output" "No sources configured"

exit $FAILED
