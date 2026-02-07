#!/bin/bash
# Test CLI: newsfed sources enable/disable

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Disable an enabled source
add_output=$(newsfed sources add --type=rss --url=https://example.com/disable-test.xml --name="Disable Test" 2>&1)
source_id=$(extract_uuid "$add_output")
output=$(newsfed sources disable "$source_id" 2>&1)
test_contains "Confirms disable" "$output" "Disabled source:"
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows disabled status" "$show_output" "Status:      ✗ Disabled"

# Test 2: Re-enable a disabled source
output=$(newsfed sources enable "$source_id" 2>&1)
test_contains "Confirms enable" "$output" "Enabled source:"
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows enabled status" "$show_output" "Status:      ✓ Enabled"

# Test 3: Enable already enabled source (idempotent)
output=$(newsfed sources enable "$source_id" 2>&1)
test_contains "Shows already enabled message" "$output" "already enabled"

# Test 4: Disable already disabled source (idempotent)
newsfed sources disable "$source_id" > /dev/null 2>&1
output=$(newsfed sources disable "$source_id" 2>&1)
test_contains "Shows already disabled message" "$output" "already disabled"

# Test 5: Multiple enable/disable cycles
newsfed sources enable "$source_id" > /dev/null 2>&1
newsfed sources disable "$source_id" > /dev/null 2>&1
newsfed sources enable "$source_id" > /dev/null 2>&1
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Final state is enabled" "$show_output" "Status:      ✓ Enabled"

# Test 6: Invalid source ID for enable
output=$(newsfed sources enable "invalid-id" 2>&1 || true)
test_contains "Returns error for invalid ID" "$output" "Error: invalid source ID"

# Test 7: Invalid source ID for disable
output=$(newsfed sources disable "invalid-id" 2>&1 || true)
test_contains "Returns error for invalid ID" "$output" "Error: invalid source ID"

# Test 8: Non-existent source ID for enable
fake_id="00000000-0000-0000-0000-000000000000"
output=$(newsfed sources enable "$fake_id" 2>&1 || true)
test_contains "Returns error for non-existent source" "$output" "Error:"

# Test 9: Non-existent source ID for disable
output=$(newsfed sources disable "$fake_id" 2>&1 || true)
test_contains "Returns error for non-existent source" "$output" "Error:"

# Test 10: Missing source ID for enable
output=$(newsfed sources enable 2>&1 || true)
test_contains "Returns error for missing ID" "$output" "Error: source ID is required"

# Test 11: Missing source ID for disable
output=$(newsfed sources disable 2>&1 || true)
test_contains "Returns error for missing ID" "$output" "Error: source ID is required"

exit $FAILED
