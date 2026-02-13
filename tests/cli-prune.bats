#!/usr/bin/env bats
# Test CLI: newsfed prune command (Spec 8, Section 3.1.5)

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    mkdir -p "$NEWSFED_FEED_DSN"
}

teardown_file() {
    cleanup_test_env
}

setup() {
    # Clean feed directory before each test
    rm -f "$NEWSFED_FEED_DSN"/*.json
}

@test "newsfed prune -force: removes items older than 90 days" {
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)
    ONE_DAY_AGO=$(timestamp_days_ago 1)

    # Old item (should be removed)
    create_news_item "aaaa1111-1111-1111-1111-111111111111" "Old Article" "Publisher" "$NINETY_ONE_DAYS_AGO"

    # Recent item (should remain)
    create_news_item "bbbb2222-2222-2222-2222-222222222222" "Recent Article" "Publisher" "$ONE_DAY_AGO"

    run newsfed prune -force
    assert_success

    # Old item should be gone
    [ ! -f "$NEWSFED_FEED_DSN/aaaa1111-1111-1111-1111-111111111111.json" ]

    # Recent item should still exist
    [ -f "$NEWSFED_FEED_DSN/bbbb2222-2222-2222-2222-222222222222.json" ]
}

@test "newsfed prune -force: preserves pinned items" {
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)

    # Pinned old item (should remain despite age)
    create_news_item "cccc3333-3333-3333-3333-333333333333" "Pinned Old Article" "Publisher" "$NINETY_ONE_DAYS_AGO" "$NINETY_ONE_DAYS_AGO"

    run newsfed prune -force
    assert_success

    # Pinned item should still exist
    [ -f "$NEWSFED_FEED_DSN/cccc3333-3333-3333-3333-333333333333.json" ]
}

@test "newsfed prune -force -all: removes all unpinned items" {
    ONE_DAY_AGO=$(timestamp_days_ago 1)
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)

    # Recent unpinned (should be removed with -all)
    create_news_item "dddd4444-4444-4444-4444-444444444444" "Recent Unpinned" "Publisher" "$ONE_DAY_AGO"

    # Old unpinned (should be removed)
    create_news_item "eeee5555-5555-5555-5555-555555555555" "Old Unpinned" "Publisher" "$NINETY_ONE_DAYS_AGO"

    # Pinned item (should remain)
    create_news_item "ffff6666-6666-6666-6666-666666666666" "Pinned Item" "Publisher" "$ONE_DAY_AGO" "$ONE_DAY_AGO"

    run newsfed prune -force -all
    assert_success

    # Unpinned items should be gone
    [ ! -f "$NEWSFED_FEED_DSN/dddd4444-4444-4444-4444-444444444444.json" ]
    [ ! -f "$NEWSFED_FEED_DSN/eeee5555-5555-5555-5555-555555555555.json" ]

    # Pinned item should remain
    [ -f "$NEWSFED_FEED_DSN/ffff6666-6666-6666-6666-666666666666.json" ]
}

@test "newsfed prune -force: prints count of pruned items" {
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)

    create_news_item "aaaa1111-1111-1111-1111-111111111111" "Old Article 1" "Publisher" "$NINETY_ONE_DAYS_AGO"
    create_news_item "bbbb2222-2222-2222-2222-222222222222" "Old Article 2" "Publisher" "$NINETY_ONE_DAYS_AGO"

    run newsfed prune -force
    assert_success
    assert_output_contains "2 items pruned"
}

@test "newsfed prune: asks for confirmation and cancels on n" {
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)

    create_news_item "aaaa1111-1111-1111-1111-111111111111" "Old Article" "Publisher" "$NINETY_ONE_DAYS_AGO"

    run bash -c 'echo "n" | newsfed prune'
    assert_success
    assert_output_contains "Cancelled"

    # Item should still exist
    [ -f "$NEWSFED_FEED_DSN/aaaa1111-1111-1111-1111-111111111111.json" ]
}

@test "newsfed prune: confirmation with y proceeds" {
    NINETY_ONE_DAYS_AGO=$(timestamp_days_ago 91)

    create_news_item "aaaa1111-1111-1111-1111-111111111111" "Old Article" "Publisher" "$NINETY_ONE_DAYS_AGO"

    run bash -c 'echo "y" | newsfed prune'
    assert_success
    assert_output_contains "1 items pruned"

    # Item should be removed
    [ ! -f "$NEWSFED_FEED_DSN/aaaa1111-1111-1111-1111-111111111111.json" ]
}

@test "newsfed prune -force: zero items pruned when nothing is stale" {
    ONE_DAY_AGO=$(timestamp_days_ago 1)

    create_news_item "aaaa1111-1111-1111-1111-111111111111" "Recent Article" "Publisher" "$ONE_DAY_AGO"

    run newsfed prune -force
    assert_success
    assert_output_contains "0 items pruned"

    # Item should still exist
    [ -f "$NEWSFED_FEED_DSN/aaaa1111-1111-1111-1111-111111111111.json" ]
}
