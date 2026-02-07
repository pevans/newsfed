#!/bin/bash
# Test CLI: newsfed sources update

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Setup: Add a test source
add_output=$(newsfed sources add --type=rss --url=https://example.com/update-test.xml --name="Update Test Feed" 2>&1)
source_id=$(extract_uuid "$add_output")

# Test 1: Update source name
output=$(newsfed sources update "$source_id" --name="Updated Feed Name" 2>&1)
test_contains "Confirms update" "$output" "Updated source:"
test_contains "Shows new name in output" "$output" "Name: Updated Feed Name"
# Verify update persisted
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Name persisted" "$show_output" "Updated Feed Name"

# Test 2: Update polling interval
output=$(newsfed sources update "$source_id" --interval=2h 2>&1)
test_contains "Confirms interval update" "$output" "Updated source:"
test_contains "Shows new interval" "$output" "Interval: 2h"
# Verify update persisted
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Interval persisted" "$show_output" "Poll Interval:   2h"

# Test 3: Update multiple fields at once
add_output=$(newsfed sources add --type=rss --url=https://example.com/multi-update.xml --name="Multi Update Test" 2>&1)
multi_id=$(extract_uuid "$add_output")
output=$(newsfed sources update "$multi_id" --name="New Multi Name" --interval=1h 2>&1)
test_contains "Updates both fields" "$output" "Updated source:"
show_output=$(newsfed sources show "$multi_id" 2>&1)
test_contains "Name updated" "$show_output" "New Multi Name"
test_contains "Interval updated" "$show_output" "Poll Interval:   1h"

# Test 4: Update scraper config for website source
# Create initial config
cat > "$TEST_DIR/initial-config.json" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "h1",
    "content_selector": ".content"
  }
}
EOF
add_output=$(newsfed sources add --type=website --url=https://example.com/update-website --name="Website Update Test" --config="$TEST_DIR/initial-config.json" 2>&1)
website_id=$(extract_uuid "$add_output")
# Create updated config
cat > "$TEST_DIR/updated-config.json" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "h1.new-title",
    "content_selector": ".new-content"
  }
}
EOF
output=$(newsfed sources update "$website_id" --config="$TEST_DIR/updated-config.json" 2>&1)
test_contains "Confirms scraper update" "$output" "Scraper: Updated"
show_output=$(newsfed sources show "$website_id" 2>&1)
test_contains "New title selector" "$show_output" "Title Selector:     h1.new-title"

# Test 5: Invalid source ID
output=$(newsfed sources update "invalid-id" --name="Test" 2>&1 || true)
test_contains "Returns error for invalid ID" "$output" "Error: invalid source ID"

# Test 6: Non-existent source ID
fake_id="00000000-0000-0000-0000-000000000000"
output=$(newsfed sources update "$fake_id" --name="Test" 2>&1 || true)
test_contains "Returns error for non-existent source" "$output" "Error:"

# Test 7: Missing source ID argument
output=$(newsfed sources update 2>&1 || true)
test_contains "Returns error for missing ID" "$output" "Error: source ID is required"

# Test 8: No update flags provided
output=$(newsfed sources update "$source_id" 2>&1 || true)
test_contains "Returns error for no updates" "$output" "Error: at least one update flag is required"

# Test 9: Invalid interval format
output=$(newsfed sources update "$source_id" --interval=invalid 2>&1 || true)
test_contains "Returns error for invalid interval" "$output" "Error: invalid interval format"

exit $FAILED
