#!/usr/bin/env bats
# Test CLI: error handling

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
}

teardown_file() {
    cleanup_test_env
}

# Storage error tests

@test "newsfed errors: handles missing metadata database gracefully" {
    export NEWSFED_METADATA_DSN="$TEST_DIR/nonexistent.db"
    export NEWSFED_FEED_DSN="$TEST_DIR/.news"

    run newsfed sources list
    # Command succeeds but shows no sources when DB doesn't exist
    assert_success
    assert_output_contains "No sources"
}

@test "newsfed errors: handles corrupted metadata database" {
    # Create corrupt database file
    mkdir -p "$(dirname "$NEWSFED_METADATA_DSN")"
    echo "not a valid database" > "$NEWSFED_METADATA_DSN"

    run newsfed sources list
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed errors: handles missing feed directory" {
    newsfed init > /dev/null 2>&1
    rm -rf "$NEWSFED_FEED_DSN"

    run newsfed list
    # NewNewsFeed re-creates the directory, so list succeeds with empty results
    assert_success
}

@test "newsfed errors: handles permission denied on metadata database" {
    # Create temporary test directory
    local perm_test_dir=$(create_permission_test_dir)

    # Create database then make it unreadable
    local temp_db="$perm_test_dir/metadata.db"
    local orig_metadata_dsn="$NEWSFED_METADATA_DSN"
    local orig_feed_dsn="$NEWSFED_FEED_DSN"

    export NEWSFED_METADATA_DSN="$temp_db"
    export NEWSFED_FEED_DSN="$perm_test_dir/.news"

    # Initialize database
    newsfed init > /dev/null 2>&1

    # Make database file unreadable
    chmod 000 "$temp_db"

    # Capture error immediately
    run newsfed sources list

    # Restore permissions immediately after test
    chmod 644 "$temp_db" 2>/dev/null || true

    # Cleanup
    rm -rf "$perm_test_dir"

    # Restore original environment
    export NEWSFED_METADATA_DSN="$orig_metadata_dsn"
    export NEWSFED_FEED_DSN="$orig_feed_dsn"

    # Verify error handling
    assert_failure
    assert_output_contains "Error"
    assert_output_contains "permission denied"
}

@test "newsfed errors: handles permission denied on feed directory" {
    # Create temporary test directory
    local perm_test_dir=$(create_permission_test_dir)

    # Save original environment
    local orig_metadata_dsn="$NEWSFED_METADATA_DSN"
    local orig_feed_dsn="$NEWSFED_FEED_DSN"

    # Create feed directory then make it unreadable
    export NEWSFED_METADATA_DSN="$perm_test_dir/metadata.db"
    export NEWSFED_FEED_DSN="$perm_test_dir/.news"

    mkdir -p "$NEWSFED_FEED_DSN"
    chmod 000 "$NEWSFED_FEED_DSN"

    # Capture error immediately
    run newsfed list

    # Restore permissions immediately after test
    chmod 755 "$NEWSFED_FEED_DSN" 2>/dev/null || true

    # Cleanup
    rm -rf "$perm_test_dir"

    # Restore original environment
    export NEWSFED_METADATA_DSN="$orig_metadata_dsn"
    export NEWSFED_FEED_DSN="$orig_feed_dsn"

    # Verify error handling
    assert_failure
    assert_output_contains "Error"
    assert_output_contains "permission denied"
}

@test "newsfed errors: handles corrupted JSON item files" {
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null 2>&1

    # Create corrupt JSON file
    echo "not valid json" > "$NEWSFED_FEED_DSN/11111111-1111-1111-1111-111111111111.json"

    run newsfed list
    assert_success
    assert_output_contains "Warning"
    assert_output_contains "1 item(s) could not be read"
}

# Input validation tests

@test "newsfed errors: validates UUID format" {
    run newsfed show "not-a-uuid"
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed errors: validates required flags" {
    run newsfed sources add --url=https://example.com/feed.xml
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed errors: validates URL format" {
    run newsfed sources add --type=rss --url="not a url" --name="Test"
    assert_failure
    assert_output_contains "Error"
}

@test "newsfed errors: validates enum values" {
    run newsfed sources add --type=invalid --url=https://example.com/feed.xml --name="Test"
    assert_failure
    assert_output_contains "Error"
}

# Partial failure tests (Spec 8 section 6.4)

@test "newsfed errors: list displays valid items alongside corrupted file warnings" {
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null 2>&1

    # Create two valid items
    create_news_item \
        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" \
        "Valid Article One" \
        "Publisher A"
    create_news_item \
        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" \
        "Valid Article Two" \
        "Publisher B"

    # Create a corrupted JSON file
    echo "not valid json" > "$NEWSFED_FEED_DSN/cccccccc-cccc-cccc-cccc-cccccccccccc.json"

    run newsfed list --all
    assert_success
    # Valid items are displayed
    assert_output_contains "Valid Article One"
    assert_output_contains "Valid Article Two"
    # Warning about the corrupted file is reported
    assert_output_contains "Warning"
    assert_output_contains "1 item(s) could not be read"
}

@test "newsfed errors: list reports multiple corrupted files" {
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null 2>&1

    # Create one valid item
    create_news_item \
        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" \
        "The Good Article" \
        "Publisher A"

    # Create two corrupted JSON files
    echo "{bad" > "$NEWSFED_FEED_DSN/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb.json"
    echo "" > "$NEWSFED_FEED_DSN/cccccccc-cccc-cccc-cccc-cccccccccccc.json"

    run newsfed list --all
    assert_success
    assert_output_contains "The Good Article"
    assert_output_contains "2 item(s) could not be read"
}

# Error message quality tests (Spec 8 section 6.1)

@test "newsfed errors: user-facing messages have no internal Go errors" {
    # Test various error scenarios to ensure clean error messages
    newsfed init > /dev/null 2>&1

    # Test 1: Invalid UUID format
    run newsfed show invalid-uuid
    assert_output_not_contains "panic:"
    assert_output_not_contains "goroutine"

    # Test 2: Non-existent item
    run newsfed show 99999999-9999-9999-9999-999999999999
    assert_output_not_contains "panic:"
    assert_output_not_contains "goroutine"

    # Test 3: Non-existent source
    run newsfed sources show 99999999-9999-9999-9999-999999999999
    assert_output_not_contains "panic:"
    assert_output_not_contains "goroutine"

    # Test 4: Invalid source type
    run newsfed sources add --type=invalid --url=https://example.com --name="Test"
    assert_output_not_contains "panic:"
    assert_output_not_contains "goroutine"
}
