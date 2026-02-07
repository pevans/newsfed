#!/bin/bash
# Test CLI: newsfed sources delete

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Delete an existing source
add_output=$(newsfed sources add --type=rss --url=https://example.com/delete-test-1.xml --name="Delete Test 1" 2>&1)
source_id=$(extract_uuid "$add_output")
output=$(newsfed sources delete "$source_id" 2>&1)
test_contains "Confirms deletion" "$output" "Deleted source:"
test_contains "Shows deleted source ID" "$output" "$source_id"

# Test 2: Verify deleted source no longer appears in list
list_output=$(newsfed sources list 2>&1)
test_not_contains "Source not in list after deletion" "$list_output" "Delete Test 1"

# Test 3: Verify deleted source cannot be shown
show_output=$(newsfed sources show "$source_id" 2>&1 || true)
test_contains "Returns error when showing deleted source" "$show_output" "Error:"

# Test 4: Delete multiple sources
add_output1=$(newsfed sources add --type=rss --url=https://example.com/delete-test-2.xml --name="Delete Test 2" 2>&1)
id1=$(extract_uuid "$add_output1")
add_output2=$(newsfed sources add --type=rss --url=https://example.com/delete-test-3.xml --name="Delete Test 3" 2>&1)
id2=$(extract_uuid "$add_output2")
newsfed sources delete "$id1" > /dev/null 2>&1
newsfed sources delete "$id2" > /dev/null 2>&1
list_output=$(newsfed sources list 2>&1)
test_not_contains "First source deleted" "$list_output" "Delete Test 2"
test_not_contains "Second source deleted" "$list_output" "Delete Test 3"

# Test 5: Invalid source ID
output=$(newsfed sources delete "invalid-id" 2>&1 || true)
test_contains "Returns error for invalid ID" "$output" "Error: invalid source ID"

# Test 6: Non-existent source ID
fake_id="00000000-0000-0000-0000-000000000000"
output=$(newsfed sources delete "$fake_id" 2>&1 || true)
test_contains "Returns error for non-existent source" "$output" "Error:"

# Test 7: Missing source ID argument
output=$(newsfed sources delete 2>&1 || true)
test_contains "Returns error for missing ID" "$output" "Error: source ID is required"

# Test 8: Cannot delete same source twice
add_output=$(newsfed sources add --type=rss --url=https://example.com/delete-twice.xml --name="Delete Twice Test" 2>&1)
source_id=$(extract_uuid "$add_output")
newsfed sources delete "$source_id" > /dev/null 2>&1
output=$(newsfed sources delete "$source_id" 2>&1 || true)
test_contains "Returns error on second deletion" "$output" "Error:"

exit $FAILED
