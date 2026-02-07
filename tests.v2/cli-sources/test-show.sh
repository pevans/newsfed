#!/bin/bash
# Test CLI: newsfed sources show

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Setup: Add a test source
add_output=$(newsfed sources add --type=rss --url=https://example.com/show-test.xml --name="Show Test Feed" 2>&1)
source_id=$(extract_uuid "$add_output")

# Test 1: Show source details
output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows source name" "$output" "Show Test Feed"
test_contains "Shows source type" "$output" "Type:        rss"
test_contains "Shows source URL" "$output" "URL:         https://example.com/show-test.xml"
test_contains "Shows source ID" "$output" "$source_id"

# Test 2: Shows operational info section
output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows operational info header" "$output" "Operational Info:"
test_contains "Shows last fetched field" "$output" "Last Fetched:"
test_contains "Shows poll interval field" "$output" "Poll Interval:"

# Test 3: Shows health section
output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows health header" "$output" "Health:"
test_contains "Shows error count" "$output" "Error Count:"
test_contains "Shows last error field" "$output" "Last Error:"

# Test 4: Shows created/updated timestamps
output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows created timestamp" "$output" "Created:"
test_contains "Shows updated timestamp" "$output" "Updated:"

# Test 5: Shows enabled status
output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Shows enabled status" "$output" "Status:      ✓ Enabled"

# Test 6: Invalid source ID
output=$(newsfed sources show "invalid-id" 2>&1 || true)
test_contains "Returns error for invalid ID" "$output" "Error: invalid source ID"

# Test 7: Non-existent source ID
fake_id="00000000-0000-0000-0000-000000000000"
output=$(newsfed sources show "$fake_id" 2>&1 || true)
test_contains "Returns error for non-existent source" "$output" "Error:"

# Test 8: Missing source ID argument
output=$(newsfed sources show 2>&1 || true)
test_contains "Returns error for missing ID" "$output" "Error: source ID is required"

# Test 9: Website source shows scraper config
cat > "$TEST_DIR/scraper-config-show.json" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "h1.post-title",
    "content_selector": "div.post-content"
  }
}
EOF
add_output=$(newsfed sources add --type=website --url=https://example.com/website-show --name="Website Show Test" --config="$TEST_DIR/scraper-config-show.json" 2>&1)
website_id=$(extract_uuid "$add_output")
output=$(newsfed sources show "$website_id" 2>&1)
test_contains "Shows scraper configuration header" "$output" "Scraper Configuration:"
test_contains "Shows discovery mode" "$output" "Discovery Mode:"
test_contains "Shows title selector" "$output" "Title Selector:"
test_contains "Shows content selector" "$output" "Content Selector:"

exit $FAILED
