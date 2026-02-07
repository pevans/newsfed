#!/bin/bash
# Test CLI: newsfed sources add

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Add RSS source
output=$(newsfed sources add --type=rss --url=https://example.com/feed.xml --name="Test RSS Feed" 2>&1)
test_contains "Successfully adds RSS source" "$output" "Created source:"
test_contains "Shows correct type" "$output" "Type: rss"
test_contains "Shows correct name" "$output" "Name: Test RSS Feed"
test_contains "Shows correct URL" "$output" "URL: https://example.com/feed.xml"

# Test 2: Add Atom source
output=$(newsfed sources add --type=atom --url=https://example.com/atom.xml --name="Test Atom Feed" 2>&1)
test_contains "Successfully adds Atom source" "$output" "Created source:"
test_contains "Shows correct type for Atom" "$output" "Type: atom"

# Test 3: Add website source with config
cat > "$TEST_DIR/scraper-config.json" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "h1.title",
    "content_selector": ".content"
  }
}
EOF
output=$(newsfed sources add --type=website --url=https://example.com/articles --name="Test Website" --config="$TEST_DIR/scraper-config.json" 2>&1)
test_contains "Successfully adds website source" "$output" "Created source:"
test_contains "Shows scraper configured" "$output" "Scraper: Configured"

# Test 4: Missing required --type flag
output=$(newsfed sources add --url=https://example.com/test.xml --name="Missing Type" 2>&1 || true)
test_contains "Returns error for missing type" "$output" "Error: --type is required"

# Test 5: Missing required --url flag
output=$(newsfed sources add --type=rss --name="Missing URL" 2>&1 || true)
test_contains "Returns error for missing URL" "$output" "Error: --url is required"

# Test 6: Missing required --name flag
output=$(newsfed sources add --type=rss --url=https://example.com/test.xml 2>&1 || true)
test_contains "Returns error for missing name" "$output" "Error: --name is required"

# Test 7: Invalid source type
output=$(newsfed sources add --type=invalid --url=https://example.com/test.xml --name="Invalid Type" 2>&1 || true)
test_contains "Returns error for invalid type" "$output" "Error: --type must be"

# Test 8: Website source without config
output=$(newsfed sources add --type=website --url=https://example.com/test --name="No Config" 2>&1 || true)
test_contains "Returns error for website without config" "$output" "Error: --config is required for website sources"

# Test 9: Verify source is enabled by default
output=$(newsfed sources add --type=rss --url=https://example.com/enabled-test.xml --name="Enabled Test" 2>&1)
source_id=$(extract_uuid "$output")
show_output=$(newsfed sources show "$source_id" 2>&1)
test_contains "Source is enabled by default" "$show_output" "Status:      ✓ Enabled"

exit $FAILED
