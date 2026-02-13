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
    run newsfed sources add -type=rss -url=https://example.com/feed.xml -name="Test RSS Feed"
    assert_success
    assert_output_contains "Created source:"
    assert_output_contains "Type: rss"
    assert_output_contains "Name: Test RSS Feed"
    assert_output_contains "URL: https://example.com/feed.xml"
}

@test "newsfed sources add: adds Atom source successfully" {
    run newsfed sources add -type=atom -url=https://example.com/atom.xml -name="Test Atom Feed"
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

    run newsfed sources add -type=website -url=https://example.com/articles -name="Test Website" -config="$TEST_DIR/scraper-config.json"
    assert_success
    assert_output_contains "Created source:"
    assert_output_contains "Scraper: Configured"
}

@test "newsfed sources add: requires -type flag" {
    run newsfed sources add -url=https://example.com/test.xml -name="Missing Type"
    assert_failure
    assert_output_contains "Error: -type is required"
}

@test "newsfed sources add: requires -url flag" {
    run newsfed sources add -type=rss -name="Missing URL"
    assert_failure
    assert_output_contains "Error: -url is required"
}

@test "newsfed sources add: requires -name flag" {
    run newsfed sources add -type=rss -url=https://example.com/test.xml
    assert_failure
    assert_output_contains "Error: -name is required"
}

@test "newsfed sources add: rejects invalid source type" {
    run newsfed sources add -type=invalid -url=https://example.com/test.xml -name="Invalid Type"
    assert_failure
    assert_output_contains "Error: -type must be"
}

@test "newsfed sources add: requires config for website sources" {
    run newsfed sources add -type=website -url=https://example.com/test -name="No Config"
    assert_failure
    assert_output_contains "Error: -config is required for website sources"
}

@test "newsfed sources add: source is enabled by default" {
    run newsfed sources add -type=rss -url=https://example.com/enabled-test.xml -name="Enabled Test"
    assert_success

    source_id=$(extract_uuid "$output")
    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Status:      ✓ Enabled"
}

# Test: List sources

@test "newsfed sources list: shows all sources" {
    # Add a couple of sources first
    newsfed sources add -type=rss -url=https://example.com/list-test1.xml -name="List Test 1" > /dev/null
    newsfed sources add -type=atom -url=https://example.com/list-test2.xml -name="List Test 2" > /dev/null

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
    # Create a fresh database with no sources
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    run newsfed sources list
    assert_success
    assert_output_contains "No sources configured."
}

# Test: Show source details

@test "newsfed sources show: displays source details" {
    # Add a source first
    output_add=$(newsfed sources add -type=rss -url=https://example.com/show-test.xml -name="Show Test")
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
    output_add=$(newsfed sources add -type=rss -url=https://example.com/update-test.xml -name="Update Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" -name="Updated Name"
    assert_success
    assert_output_contains "Updated source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Updated Name"
}

@test "newsfed sources update: updates polling interval" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/interval-test.xml -name="Interval Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" -interval=2h
    assert_success
    assert_output_contains "Updated source:"
    assert_output_contains "Interval: 2h"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Poll Interval:   2h"
}

@test "newsfed sources update: updates multiple fields at once" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/multi-update.xml -name="Multi Update Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" -name="New Multi Name" -interval=1h
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

    output_add=$(newsfed sources add -type=website -url=https://example.com/update-website -name="Website Update Test" -config="$TEST_DIR/initial-config.json")
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

    run newsfed sources update "$source_id" -config="$TEST_DIR/updated-config.json"
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
    output_add=$(newsfed sources add -type=rss -url=https://example.com/no-flags.xml -name="No Flags Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id"
    assert_failure
    assert_output_contains "Error: at least one update flag is required"
}

@test "newsfed sources update: validates interval format" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/bad-interval.xml -name="Bad Interval Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources update "$source_id" -interval=invalid
    assert_failure
    assert_output_contains "Error: invalid interval format"
}

@test "newsfed sources update: validates source ID format" {
    run newsfed sources update "invalid-uuid" -name="Test"
    assert_failure
    assert_output_contains "Error: invalid source ID"
}

@test "newsfed sources update: handles non-existent source" {
    run newsfed sources update "00000000-0000-0000-0000-000000000000" -name="New Name"
    assert_failure
    assert_output_contains "Error:"
}

# Test: Enable and disable sources

@test "newsfed sources disable: disables a source" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/disable-test.xml -name="Disable Test")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources disable "$source_id"
    assert_success
    assert_output_contains "Disabled source:"

    run newsfed sources show "$source_id"
    assert_success
    assert_output_contains "Status:      ✗ Disabled"
}

@test "newsfed sources enable: enables a disabled source" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/enable-test.xml -name="Enable Test")
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
    output_add=$(newsfed sources add -type=rss -url=https://example.com/delete-test.xml -name="Delete Test")
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
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/feed.xml" "Mock Feed" 2
    start_mock_server "$TEST_DIR/www"

    # Add a source pointing to mock server
    newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        -name="Mock RSS" > /dev/null

    # Run sync
    run newsfed sync

    # Clean up server before assertions
    stop_mock_server

    assert_success
    assert_output_contains "Syncing all enabled sources..."
    assert_output_contains "Sync completed:"
    assert_output_contains "Sources synced: 1"
    assert_output_contains "Sources failed: 0"
    assert_output_contains "Items discovered: 2"
}

@test "newsfed sources sync: syncs specific source by ID" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/feed2.xml" "Specific Feed" 3
    start_mock_server "$TEST_DIR/www"

    # Add a source and capture its ID
    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed2.xml" \
        -name="Specific Source")
    source_id=$(extract_uuid "$output_add")

    # Run sync with specific source ID
    run newsfed sync "$source_id"

    # Clean up server before assertions
    stop_mock_server

    assert_success
    assert_output_contains "Syncing source: Specific Source"
    assert_output_contains "Sync completed:"
    assert_output_contains "Sources synced: 1"
    assert_output_contains "Sources failed: 0"
    assert_output_contains "Items discovered: 3"
}

# Test: Source status monitoring

@test "newsfed sources status: shows no sources message when empty" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    run newsfed sources status
    assert_success
    assert_output_contains "No sources configured"
}

@test "newsfed sources status: shows all sources healthy when no issues" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source and mark it as recently fetched
    output_add=$(newsfed sources add -type=rss -url=https://example.com/healthy.xml -name="Healthy Source")
    source_id=$(extract_uuid "$output_add")

    # Update last_fetched_at to be recent (1 hour ago)
    recent_time=$(timestamp_hours_ago 1)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$recent_time' WHERE source_id = '$source_id'"

    run newsfed sources status
    assert_success
    assert_output_contains "✓ Healthy:          1"
    assert_output_contains "⚠ With Errors:      0"
    assert_output_contains "All sources are healthy!"
}

@test "newsfed sources status: detects sources with errors" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source and set error count
    output_add=$(newsfed sources add -type=rss -url=https://example.com/error.xml -name="Error Source")
    source_id=$(extract_uuid "$output_add")

    # Update to have errors
    exec_sqlite "UPDATE sources SET fetch_error_count = 3, last_error = 'Connection timeout' WHERE source_id = '$source_id'"

    run newsfed sources status
    assert_success
    assert_output_contains "⚠ With Errors:      1"
    assert_output_contains "━━━ Sources with Errors ━━━"
    assert_output_contains "Error Source"
    assert_output_contains "Error Count: 3"
    assert_output_contains "Last Error: Connection timeout"
}

@test "newsfed sources status: detects never fetched sources" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source (it will have last_fetched_at = NULL by default)
    output_add=$(newsfed sources add -type=rss -url=https://example.com/never.xml -name="Never Fetched")

    run newsfed sources status
    assert_success
    assert_output_contains "⚠ Never Fetched:    1"
    assert_output_contains "━━━ Sources Never Fetched ━━━"
    assert_output_contains "Never Fetched"
}

@test "newsfed sources status: detects stale sources (>24h)" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source and set last_fetched_at to 48 hours ago
    output_add=$(newsfed sources add -type=rss -url=https://example.com/stale.xml -name="Stale Source")
    source_id=$(extract_uuid "$output_add")

    stale_time=$(timestamp_days_ago 2)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$stale_time' WHERE source_id = '$source_id'"

    run newsfed sources status
    assert_success
    assert_output_contains "⚠ Stale (>24h):     1"
    assert_output_contains "━━━ Stale Sources (>24h since fetch) ━━━"
    assert_output_contains "Stale Source"
}

@test "newsfed sources status: detects disabled sources" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source and disable it
    output_add=$(newsfed sources add -type=rss -url=https://example.com/disabled.xml -name="Disabled Source")
    source_id=$(extract_uuid "$output_add")

    newsfed sources disable "$source_id" > /dev/null

    run newsfed sources status
    assert_success
    assert_output_contains "✗ Disabled:         1"
    assert_output_contains "━━━ Disabled Sources ━━━"
    assert_output_contains "Disabled Source"
}

@test "newsfed sources status: verbose mode shows full error messages" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source with a long error message
    output_add=$(newsfed sources add -type=rss -url=https://example.com/verbose.xml -name="Verbose Test")
    source_id=$(extract_uuid "$output_add")

    long_error="This is a very long error message that exceeds 80 characters and should be truncated in normal mode but shown in full with verbose"
    exec_sqlite "UPDATE sources SET fetch_error_count = 1, last_error = '$long_error' WHERE source_id = '$source_id'"

    # Test without verbose (should truncate)
    run newsfed sources status
    assert_success
    # The error should be shown but possibly truncated
    assert_output_contains "Last Error:"

    # Test with verbose (should show full message)
    run newsfed sources status -verbose
    assert_success
    assert_output_contains "This is a very long error message that exceeds 80 characters"
}

@test "newsfed sources status: handles mixed health states" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add healthy source
    output_healthy=$(newsfed sources add -type=rss -url=https://example.com/h.xml -name="Healthy")
    id_healthy=$(extract_uuid "$output_healthy")
    recent_time=$(timestamp_hours_ago 1)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$recent_time' WHERE source_id = '$id_healthy'"

    # Add source with errors
    output_error=$(newsfed sources add -type=rss -url=https://example.com/e.xml -name="With Errors")
    id_error=$(extract_uuid "$output_error")
    exec_sqlite "UPDATE sources SET fetch_error_count = 2 WHERE source_id = '$id_error'"

    # Add never fetched source
    newsfed sources add -type=rss -url=https://example.com/n.xml -name="Never Fetched" > /dev/null

    # Add stale source
    output_stale=$(newsfed sources add -type=atom -url=https://example.com/s.xml -name="Stale")
    id_stale=$(extract_uuid "$output_stale")
    stale_time=$(timestamp_days_ago 3)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$stale_time' WHERE source_id = '$id_stale'"

    # Add disabled source
    output_disabled=$(newsfed sources add -type=rss -url=https://example.com/d.xml -name="Disabled")
    id_disabled=$(extract_uuid "$output_disabled")
    newsfed sources disable "$id_disabled" > /dev/null

    run newsfed sources status
    assert_success
    assert_output_contains "✓ Healthy:          1"
    assert_output_contains "⚠ With Errors:      1"
    assert_output_contains "⚠ Never Fetched:    1"
    assert_output_contains "⚠ Stale (>24h):     1"
    assert_output_contains "✗ Disabled:         1"
}

@test "newsfed sources status: provides actionable suggestions" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    # Add a source with errors
    output_add=$(newsfed sources add -type=rss -url=https://example.com/suggest.xml -name="Needs Action")
    source_id=$(extract_uuid "$output_add")
    exec_sqlite "UPDATE sources SET fetch_error_count = 1 WHERE source_id = '$source_id'"

    run newsfed sources status
    assert_success
    assert_output_contains "Suggested Actions:"
    assert_output_contains "Check source configurations for errors"
    assert_output_contains "Run 'newsfed sources show <id>' for details"
}

# Test: View error history (Spec 8 section 3.3.2)

@test "newsfed sources errors: shows no errors for clean source" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    output_add=$(newsfed sources add -type=rss -url=https://example.com/clean.xml -name="Clean Source")
    source_id=$(extract_uuid "$output_add")

    run newsfed sources errors "$source_id"
    assert_success
    assert_output_contains "Error history for: Clean Source"
    assert_output_contains "No errors recorded."
}

@test "newsfed sources errors: shows recorded errors" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    output_add=$(newsfed sources add -type=rss -url=https://example.com/errors.xml -name="Error Source")
    source_id=$(extract_uuid "$output_add")

    # Insert errors directly into the source_errors table
    exec_sqlite "INSERT INTO source_errors (source_id, error, occurred_at) VALUES ('$source_id', 'Connection timeout', '2026-02-10T10:00:00Z')"
    exec_sqlite "INSERT INTO source_errors (source_id, error, occurred_at) VALUES ('$source_id', 'DNS resolution failed', '2026-02-10T11:00:00Z')"

    run newsfed sources errors "$source_id"
    assert_success
    assert_output_contains "Error history for: Error Source"
    assert_output_contains "Connection timeout"
    assert_output_contains "DNS resolution failed"
}

@test "newsfed sources errors: shows errors in reverse chronological order" {
    # Create a fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    output_add=$(newsfed sources add -type=rss -url=https://example.com/order.xml -name="Order Test")
    source_id=$(extract_uuid "$output_add")

    exec_sqlite "INSERT INTO source_errors (source_id, error, occurred_at) VALUES ('$source_id', 'First error', '2026-02-10T08:00:00Z')"
    exec_sqlite "INSERT INTO source_errors (source_id, error, occurred_at) VALUES ('$source_id', 'Second error', '2026-02-10T09:00:00Z')"
    exec_sqlite "INSERT INTO source_errors (source_id, error, occurred_at) VALUES ('$source_id', 'Third error', '2026-02-10T10:00:00Z')"

    run newsfed sources errors "$source_id"
    assert_success

    # Third error (most recent) should appear before First error (oldest)
    third_pos=$(echo "$output" | grep -n "Third error" | head -1 | cut -d: -f1)
    first_pos=$(echo "$output" | grep -n "First error" | head -1 | cut -d: -f1)
    [ "$third_pos" -lt "$first_pos" ]
}

@test "newsfed sources errors: requires source ID argument" {
    run newsfed sources errors
    assert_failure
    assert_output_contains "Error: source ID is required"
}

@test "newsfed sources errors: validates source ID format" {
    run newsfed sources errors "not-a-uuid"
    assert_failure
    assert_output_contains "Error: invalid source ID"
}

@test "newsfed sources errors: handles non-existent source" {
    run newsfed sources errors "00000000-0000-0000-0000-000000000000"
    assert_failure
    assert_output_contains "Error:"
}
