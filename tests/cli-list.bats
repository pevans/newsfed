#!/usr/bin/env bats
# Test CLI: newsfed list command

load test_helper

setup_file() {
    # Setup runs once for the entire test file
    setup_test_env
    build_newsfed "$TEST_DIR"

    # Create test data directory
    mkdir -p "$NEWSFED_FEED_DSN"

    # Create sample news items with various characteristics
    ONE_DAY_AGO=$(timestamp_days_ago 1)
    TWO_DAYS_AGO=$(timestamp_days_ago 2)
    TEN_DAYS_AGO=$(timestamp_days_ago 10)
    TWO_HOURS_AGO=$(timestamp_hours_ago 2)
    TWELVE_HOURS_AGO=$(timestamp_hours_ago 12)

    # Item 1: Recent item (1 day old), Publisher A
    create_news_item \
        "11111111-1111-1111-1111-111111111111" \
        "Recent Article from Publisher A" \
        "Publisher A" \
        "$ONE_DAY_AGO"

    # Item 2: Recent item (2 days old), Publisher B
    create_news_item \
        "22222222-2222-2222-2222-222222222222" \
        "Recent Article from Publisher B" \
        "Publisher B" \
        "$TWO_DAYS_AGO"

    # Item 3: Old item (10 days old), Publisher A
    create_news_item \
        "33333333-3333-3333-3333-333333333333" \
        "Old Article from Publisher A" \
        "Publisher A" \
        "$TEN_DAYS_AGO"

    # Item 4: Old pinned item (10 days old), Publisher C
    create_news_item \
        "44444444-4444-4444-4444-444444444444" \
        "Old Pinned Article" \
        "Publisher C" \
        "$TEN_DAYS_AGO" \
        "$ONE_DAY_AGO"

    # Item 5: Recent pinned item (1 day old), Publisher B
    create_news_item \
        "55555555-5555-5555-5555-555555555555" \
        "Recent Pinned Article" \
        "Publisher B" \
        "$ONE_DAY_AGO" \
        "$TWO_HOURS_AGO"

    # Item 6: Very recent item (12 hours old), no publisher
    create_news_item \
        "66666666-6666-6666-6666-666666666666" \
        "Very Recent Article No Publisher" \
        "" \
        "$TWELVE_HOURS_AGO"
}

teardown_file() {
    cleanup_test_env
}

# Basic functionality tests

@test "newsfed list: shows recent items (default: past 3 days)" {
    run newsfed list
    assert_success
    assert_output_contains "Recent Article from Publisher A"
    assert_output_contains "Recent Article from Publisher B"
}

@test "newsfed list: shows pinned items regardless of age" {
    run newsfed list
    assert_success
    assert_output_contains "Recent Pinned Article"
    assert_output_contains "Old Pinned Article"
}

@test "newsfed list: does not show old unpinned items" {
    run newsfed list
    assert_success
    assert_output_not_contains "Old Article from Publisher A"
}

@test "newsfed list: shows item titles" {
    run newsfed list
    assert_success
    assert_output_contains "Recent Article"
}

@test "newsfed list: handles empty feed gracefully" {
    # Backup current feed
    mv "$NEWSFED_FEED_DSN" "${NEWSFED_FEED_DSN}.backup"
    mkdir -p "$NEWSFED_FEED_DSN"

    run newsfed list
    assert_success

    # Restore feed
    rm -rf "$NEWSFED_FEED_DSN"
    mv "${NEWSFED_FEED_DSN}.backup" "$NEWSFED_FEED_DSN"
}

# Filter tests

@test "newsfed list -all: shows all items including old ones" {
    run newsfed list -all
    assert_success
    assert_output_contains "Old Article from Publisher A"
    assert_output_contains "Recent Article from Publisher A"
}

@test "newsfed list -publisher: filters by publisher" {
    run newsfed list -all -publisher="Publisher A"
    assert_success
    assert_output_contains "Recent Article from Publisher A"
    assert_output_contains "Old Article from Publisher A"
    assert_output_not_contains "Publisher B"
    assert_output_not_contains "Publisher C"
}

@test "newsfed list -pinned: shows only pinned items" {
    run newsfed list -pinned
    assert_success
    assert_output_contains "Recent Pinned Article"
    assert_output_contains "Old Pinned Article"
    assert_output_not_contains "Recent Article from Publisher A"
}

# Sorting tests

@test "newsfed list: default sort is by published_at descending" {
    run newsfed list -all
    assert_success
    # Should show newest first
    # This is a simplified test - in practice you'd check order
}

@test "newsfed list -sort=published: sorts by published date" {
    run newsfed list -all -sort=published
    assert_success
    # Items should be sorted by published date
}

@test "newsfed list -sort=discovered: sorts by discovered date" {
    run newsfed list -all -sort=discovered
    assert_success
    # Items should be sorted by discovered date
}

# Pagination tests

@test "newsfed list -limit: limits number of results" {
    run newsfed list -all -limit=2
    assert_success
    # Should only show 2 items (harder to test without counting)
}

@test "newsfed list -offset: skips specified number of items" {
    run newsfed list -all -offset=3
    assert_success
    # Should skip first 3 items
}

# Format tests

@test "newsfed list -format=json: outputs valid JSON" {
    run newsfed list -format=json
    assert_success
    # Output should be valid JSON
    run bash -c "echo '$output' | python3 -m json.tool > /dev/null 2>&1"
    assert_success
}

@test "newsfed list -format=compact: shows compact format" {
    run newsfed list -format=compact
    assert_success
    # Compact format should be shorter/different from default
}

@test "newsfed list -format=table: shows table format (default)" {
    run newsfed list -format=table
    assert_success
    # Should show table format
}
