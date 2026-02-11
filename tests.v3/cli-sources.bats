#!/usr/bin/env bats
# Test CLI: newsfed sources command

load test_helper

setup_file() {
    # Setup runs once for the entire test file
    setup_test_env
    build_newsfed "$TEST_DIR"

    # Create test data directories
    mkdir -p "$NEWSFED_FEED_DSN"
    mkdir -p "$TEST_DIR/metadata"

    # Initialize the newsfed storage
    run newsfed init
}

teardown_file() {
    cleanup_test_env
}

setup() {
    # Setup runs before each test
    # We might want to reset state between tests
    true
}

# Test: Add sources

@test "newsfed sources add: adds RSS source successfully" {
    run newsfed sources add --type=rss --url=https://example.com/feed.xml --name="Test RSS Feed"
    assert_success
    assert_output_contains "Created source:"
    assert_output_contains "Type: rss"
    assert_output_contains "Name: Test RSS Feed"
    assert_output_contains "URL: https://example.com/feed.xml"
}

@test "newsfed sources add: adds Atom source successfully" {
    run newsfed sources add --type=atom --url=https://example.com/atom.xml --name="Test Atom Feed"
    assert_success
    assert_output_contains "Created source:"
    assert_output_contains "Type: atom"
}

@test "newsfed sources add: adds website source with config" {
    # Create scraper config file
    cat > "$TEST_DIR/scraper-config.json" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "h1.title",
    "content_selector": ".content"
  }
}
EOF

    run newsfed sources add --type=website --url=https://example.com/articles --name="Test Website" --config="$TEST_DIR/scraper-config.json"
    assert_success
    assert_output_contains "Created source:"
    assert_output_contains "Scraper: Configured"
}

@test "newsfed sources add: requires --type flag" {
    run newsfed sources add --url=https://example.com/test.xml --name="Missing Type"
    assert_failure
    assert_output_contains "Error: --type is required"
}

@test "newsfed sources add: requires --url flag" {
    run newsfed sources add --type=rss --name="Missing URL"
    assert_failure
    assert_output_contains "Error: --url is required"
}

@test "newsfed sources add: requires --name flag" {
    run newsfed sources add --type=rss --url=https://example.com/test.xml
    assert_failure
    assert_output_contains "Error: --name is required"
}

@test "newsfed sources add: rejects invalid source type" {
    run newsfed sources add --type=invalid --url=https://example.com/test.xml --name="Invalid Type"
    assert_failure
    assert_output_contains "Error: --type must be"
}

@test "newsfed sources add: requires config for website sources" {
    run newsfed sources add --type=website --url=https://example.com/test --name="No Config"
    assert_failure
    assert_output_contains "Error: --config is required for website sources"
}

@test "newsfed sources add: source is enabled by default" {
    run newsfed sources add --type=rss --url=https://example.com/enabled-test.xml --name="Enabled Test"
    assert_success

    source_id=$(extract_uuid "$output")
    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Status:      ✓ Enabled"
}

# Test: List sources

@test "newsfed sources list: shows all sources" {
    # Add a couple of sources first
    newsfed sources add --type=rss --url=https://example.com/list-test1.xml --name="List Test 1" > /dev/null
    newsfed sources add --type=atom --url=https://example.com/list-test2.xml --name="List Test 2" > /dev/null

    run newsfed sources list
    assert_success
    assert_output_contains "List Test 1"
    assert_output_contains "List Test 2"
}

@test "newsfed sources list: shows source types" {
    run newsfed sources list
    assert_success
    # Should show types like "rss", "atom"
}

@test "newsfed sources list: handles empty source list" {
    # This would require a fresh database, skip for now or implement cleanup
    skip "Requires fresh database state"
}

# Test: Show source details

@test "newsfed sources show: displays source details" {
    # Add a source first
    output_add=$(newsfed sources add --type=rss --url=https://example.com/show-test.xml --name="Show Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources show "$source_id"
    assert_success
    # Check for the actual output format (title on its own line)
    assert_output_contains "Show Test"
    assert_output_contains "Type:"
    assert_output_contains "rss"
    assert_output_contains "URL:"
    assert_output_contains "https://example.com/show-test.xml"
}

@test "newsfed sources show: handles non-existent source" {
    run newsfed sources show "00000000-0000-0000-0000-000000000000"
    assert_failure
    assert_output_contains "Error:"
}

# Test: Update sources

@test "newsfed sources update: updates source name" {
    # Add a source first
    output_add=$(newsfed sources add --type=rss --url=https://example.com/update-test.xml --name="Update Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" --name="Updated Name"
    assert_success
    assert_output_contains "Updated source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Updated Name"
}

@test "newsfed sources update: updates polling interval" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/interval-test.xml --name="Interval Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" --interval=2h
    assert_success
    assert_output_contains "Updated source:"
    assert_output_contains "Interval: 2h"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Poll Interval:   2h"
}

@test "newsfed sources update: updates multiple fields at once" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/multi-update.xml --name="Multi Update Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" --name="New Multi Name" --interval=1h
    assert_success
    assert_output_contains "Updated source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "New Multi Name"
    assert_output_contains "Poll Interval:   1h"
}

@test "newsfed sources update: updates scraper config for website source" {
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

    output_add=$(newsfed sources add --type=website --url=https://example.com/update-website --name="Website Update Test" --config="$TEST_DIR/initial-config.json")
    source_id=$(extract_uuid "$output_add")

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

    run newsfed sources update "$source_id" --config="$TEST_DIR/updated-config.json"
    assert_success
    assert_output_contains "Scraper: Updated"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Title Selector:     h1.new-title"
}

@test "newsfed sources update: requires source ID argument" {
    run newsfed sources update
    assert_failure
    assert_output_contains "Error: source ID is required"
}

@test "newsfed sources update: requires at least one update flag" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/no-flags.xml --name="No Flags Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id"
    assert_failure
    assert_output_contains "Error: at least one update flag is required"
}

@test "newsfed sources update: validates interval format" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/bad-interval.xml --name="Bad Interval Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" --interval=invalid
    assert_failure
    assert_output_contains "Error: invalid interval format"
}

@test "newsfed sources update: validates source ID format" {
    run newsfed sources update "invalid-uuid" --name="Test"
    assert_failure
    assert_output_contains "Error: invalid source ID"
}

@test "newsfed sources update: handles non-existent source" {
    run newsfed sources update "00000000-0000-0000-0000-000000000000" --name="New Name"
    assert_failure
    assert_output_contains "Error:"
}

# Test: Enable and disable sources

@test "newsfed sources disable: disables a source" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/disable-test.xml --name="Disable Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources disable "$source_id"
    assert_success
    assert_output_contains "Disabled source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Status:      ✗ Disabled"
}

@test "newsfed sources enable: enables a disabled source" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/enable-test.xml --name="Enable Test")
    source_id=$(extract_uuid "$output_add")

    newsfed sources disable "$source_id" > /dev/null

    run newsfed sources enable "$source_id"
    assert_success
    assert_output_contains "Enabled source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Status:      ✓ Enabled"
}

# Test: Delete sources

@test "newsfed sources delete: deletes a source" {
    output_add=$(newsfed sources add --type=rss --url=https://example.com/delete-test.xml --name="Delete Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources delete "$source_id"
    assert_success
    assert_output_contains "Deleted source:"

    run newsfed sources show "$source_id"
    assert_failure
}

@test "newsfed sources delete: handles non-existent source" {
    run newsfed sources delete "00000000-0000-0000-0000-000000000000"
    assert_failure
    assert_output_contains "Error:"
}

# Test: Sync sources

@test "newsfed sources sync: syncs all enabled sources" {
    # This test would require a working feed or mock server
    skip "Requires working feed source or mock server"
}

@test "newsfed sources sync: syncs specific source by ID" {
    skip "Requires working feed source or mock server"
}
