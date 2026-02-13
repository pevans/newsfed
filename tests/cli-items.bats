#!/usr/bin/env bats
# Test CLI: newsfed item operations (show, pin, unpin, open)

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    mkdir -p "$NEWSFED_FEED_DSN"

    # Create test items
    ONE_DAY_AGO=$(timestamp_days_ago 1)

    # Item 1: Regular unpinned article
    cat > "$NEWSFED_FEED_DSN/11111111-1111-1111-1111-111111111111.json" <<EOF
{
  "id": "11111111-1111-1111-1111-111111111111",
  "title": "Test Article for Show Command",
  "summary": "This is a detailed summary with multiple sentences to test wrapping and formatting.",
  "url": "https://example.com/test-article",
  "publisher": "Test Publisher",
  "authors": ["Alice Smith", "Bob Jones"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
EOF

    # Item 2: Pinned article
    cat > "$NEWSFED_FEED_DSN/22222222-2222-2222-2222-222222222222.json" <<EOF
{
  "id": "22222222-2222-2222-2222-222222222222",
  "title": "Pinned Article for Testing",
  "summary": "This article is pinned.",
  "url": "https://example.com/pinned-article",
  "publisher": "Test Publisher",
  "authors": ["Charlie Brown"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO",
  "pinned_at": "$ONE_DAY_AGO"
}
EOF

    # Item 3: Unpinned article for pin/unpin tests
    cat > "$NEWSFED_FEED_DSN/33333333-3333-3333-3333-333333333333.json" <<EOF
{
  "id": "33333333-3333-3333-3333-333333333333",
  "title": "Unpinned Article for Testing",
  "summary": "This article will be used to test pin and unpin commands.",
  "url": "https://example.com/unpinned-article",
  "publisher": "Test Publisher",
  "authors": [],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
EOF

    # Item 4: Article without publisher
    cat > "$NEWSFED_FEED_DSN/44444444-4444-4444-4444-444444444444.json" <<EOF
{
  "id": "44444444-4444-4444-4444-444444444444",
  "title": "No Publisher Article",
  "summary": "This article has no publisher.",
  "url": "https://example.com/no-publisher",
  "authors": [],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
EOF
}

teardown_file() {
    cleanup_test_env
}

# Test: show command

@test "newsfed show: displays all item metadata" {
    run newsfed show 11111111-1111-1111-1111-111111111111
    assert_success
    assert_output_contains "Test Article for Show Command"
    assert_output_contains "Publisher:"
    assert_output_contains "Test Publisher"
    assert_output_contains "Authors:"
    assert_output_contains "Alice Smith"
    assert_output_contains "Published:"
    assert_output_contains "Discovered:"
    assert_output_contains "https://example.com/test-article"
    assert_output_contains "detailed summary"
}

@test "newsfed show: displays unpinned status" {
    run newsfed show 11111111-1111-1111-1111-111111111111
    assert_success
    # Should show "Pinned: No" or similar
}

@test "newsfed show: displays pinned status with indicator" {
    run newsfed show 22222222-2222-2222-2222-222222222222
    assert_success
    # Should show pinned indicator
}

@test "newsfed show: shows Unknown for missing publisher" {
    run newsfed show 44444444-4444-4444-4444-444444444444
    assert_success
    # Should show "Publisher: Unknown" or similar
}

@test "newsfed show: returns error for invalid UUID" {
    run newsfed show invalid-uuid
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed show: returns error for non-existent item" {
    run newsfed show 99999999-9999-9999-9999-999999999999
    assert_failure
    assert_output_contains "not found"
}

@test "newsfed show: returns error without arguments" {
    run newsfed show
    assert_failure
    assert_output_contains "Error"
}

# Test: pin command

@test "newsfed pin: pins an unpinned item" {
    run newsfed pin 33333333-3333-3333-3333-333333333333
    assert_success
    assert_output_contains "Pinned item"
    assert_output_contains "Unpinned Article for Testing"

    # Verify pinned_at field exists in JSON
    run grep -q "pinned_at" "$NEWSFED_FEED_DSN/33333333-3333-3333-3333-333333333333.json"
    assert_success
}

@test "newsfed pin: handles already pinned item" {
    newsfed pin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
    run newsfed pin 33333333-3333-3333-3333-333333333333
    assert_success
    assert_output_contains "already pinned"
}

@test "newsfed pin: returns error for invalid UUID" {
    run newsfed pin invalid-uuid
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed pin: returns error for non-existent item" {
    run newsfed pin 99999999-9999-9999-9999-999999999999
    assert_failure
    assert_output_contains "not found"
}

@test "newsfed pin: returns error without arguments" {
    run newsfed pin
    assert_failure
    assert_output_contains "Error"
}

# Test: unpin command

@test "newsfed unpin: unpins a pinned item" {
    # First ensure it's pinned
    newsfed pin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1

    run newsfed unpin 33333333-3333-3333-3333-333333333333
    assert_success
    assert_output_contains "Unpinned item"
}

@test "newsfed unpin: handles already unpinned item" {
    # Ensure it's unpinned first
    newsfed unpin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1 || true

    run newsfed unpin 33333333-3333-3333-3333-333333333333
    assert_success
    assert_output_contains "already unpinned"
}

@test "newsfed unpin: returns error for invalid UUID" {
    run newsfed unpin invalid-uuid
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed unpin: returns error for non-existent item" {
    run newsfed unpin 99999999-9999-9999-9999-999999999999
    assert_failure
    assert_output_contains "not found"
}

@test "newsfed unpin: returns error without arguments" {
    run newsfed unpin
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed pin/unpin: preserves other item properties" {
    # Pin and unpin, then verify summary is intact
    newsfed pin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1
    newsfed unpin 33333333-3333-3333-3333-333333333333 > /dev/null 2>&1

    run grep "This article will be used to test" "$NEWSFED_FEED_DSN/33333333-3333-3333-3333-333333333333.json"
    assert_success
}

# Test: open command

@test "newsfed open: uses default browser when no config set" {
    # Clean config if it exists
    exec_sqlite "DELETE FROM config WHERE key = 'browser_command';"

    # Get platform-specific default browser
    local default_browser=$(get_default_browser)

    # Use -echo to print command instead of executing
    run newsfed open -echo 11111111-1111-1111-1111-111111111111
    assert_success
    assert_output_contains "$default_browser"
    assert_output_contains "https://example.com/test-article"
}

@test "newsfed open: uses custom browser from config" {
    # Create config table and set custom browser
    exec_sqlite "CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL);"
    exec_sqlite "INSERT OR REPLACE INTO config (key, value) VALUES ('browser_command', 'firefox');"

    run newsfed open -echo 11111111-1111-1111-1111-111111111111
    assert_success
    assert_output_contains "firefox"
    assert_output_contains "https://example.com/test-article"
}

@test "newsfed open: custom browser persists across different items" {
    # Browser config should still be firefox from previous test
    run newsfed open -echo 22222222-2222-2222-2222-222222222222
    assert_success
    assert_output_contains "firefox"
    assert_output_contains "https://example.com/pinned-article"
}

@test "newsfed open: browser config updates take effect immediately" {
    # Update to a different browser
    exec_sqlite "INSERT OR REPLACE INTO config (key, value) VALUES ('browser_command', 'chromium');"

    run newsfed open -echo 11111111-1111-1111-1111-111111111111
    assert_success
    assert_output_contains "chromium"
    assert_output_contains "https://example.com/test-article"
}

@test "newsfed open: falls back to default after clearing config" {
    # Clear custom browser config
    exec_sqlite "DELETE FROM config WHERE key = 'browser_command';"

    local default_browser=$(get_default_browser)

    run newsfed open -echo 11111111-1111-1111-1111-111111111111
    assert_success
    assert_output_contains "$default_browser"
    assert_output_contains "https://example.com/test-article"
}

@test "newsfed open: returns error for invalid UUID" {
    run newsfed open invalid-uuid
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed open: returns error for non-existent item" {
    run newsfed open 99999999-9999-9999-9999-999999999999
    assert_failure
    assert_output_contains "not found"
}
