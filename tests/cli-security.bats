#!/usr/bin/env bats
# Test CLI: file permission security (Spec 8 Section 7.1)

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
}

teardown_file() {
    cleanup_test_env
}

setup() {
    # Clean up before each test
    rm -rf "$NEWSFED_METADATA_DSN" "$NEWSFED_FEED_DSN"
}

# Init permission tests

@test "newsfed init: creates feed directory with 0700 permissions" {
    run newsfed init
    assert_success

    # Verify directory permissions are 0700 (owner-only)
    perm=$(stat -f '%Lp' "$NEWSFED_FEED_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_FEED_DSN" 2>/dev/null)
    [ "$perm" = "700" ]
}

@test "newsfed init: creates metadata database with 0600 permissions" {
    run newsfed init
    assert_success

    # Verify database file permissions are 0600 (owner read/write only)
    perm=$(stat -f '%Lp' "$NEWSFED_METADATA_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_METADATA_DSN" 2>/dev/null)
    [ "$perm" = "600" ]
}

@test "newsfed init: never creates world-readable storage files" {
    run newsfed init
    assert_success

    # Check metadata database is not world-readable
    db_perm=$(stat -f '%Lp' "$NEWSFED_METADATA_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_METADATA_DSN" 2>/dev/null)
    world_read=$((db_perm % 10))
    [ "$((world_read & 4))" -eq 0 ]

    # Check feed directory is not world-readable
    dir_perm=$(stat -f '%Lp' "$NEWSFED_FEED_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_FEED_DSN" 2>/dev/null)
    world_read=$((dir_perm % 10))
    [ "$((world_read & 4))" -eq 0 ]
}

# Doctor permission warning tests

@test "newsfed doctor: warns when database has overly permissive permissions" {
    newsfed init > /dev/null 2>&1

    # Make database world-readable
    chmod 644 "$NEWSFED_METADATA_DSN"

    run newsfed doctor
    assert_output_contains "overly permissive permissions"
    assert_output_contains "chmod 600"
}

@test "newsfed doctor: warns when feed directory has overly permissive permissions" {
    newsfed init > /dev/null 2>&1

    # Make feed directory world-readable
    chmod 755 "$NEWSFED_FEED_DSN"

    run newsfed doctor
    assert_output_contains "overly permissive permissions"
    assert_output_contains "chmod 700"
}

@test "newsfed doctor: warns when feed files have overly permissive permissions" {
    newsfed init > /dev/null 2>&1

    # Create a feed file with loose permissions
    mkdir -p "$NEWSFED_FEED_DSN"
    echo '{}' > "$NEWSFED_FEED_DSN/test.json"
    chmod 644 "$NEWSFED_FEED_DSN/test.json"

    run newsfed doctor
    assert_output_contains "file(s) have overly permissive permissions"
}

@test "newsfed doctor: no warnings when all permissions are correct" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    assert_output_contains "All checks passed"
}

# Source operations produce correctly permissioned files

@test "newsfed sources add: database retains 0600 permissions after source operations" {
    newsfed init > /dev/null 2>&1

    newsfed sources add -type=rss -url="https://example.com/feed.xml" -name="Test Feed" > /dev/null 2>&1

    perm=$(stat -f '%Lp' "$NEWSFED_METADATA_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_METADATA_DSN" 2>/dev/null)
    [ "$perm" = "600" ]
}

@test "newsfed sources add: implicitly created database gets 0600 permissions" {
    # Don't run init -- let sources add create the database implicitly
    run newsfed sources add -type=rss -url="https://example.com/feed.xml" -name="Test Feed"
    assert_success

    perm=$(stat -f '%Lp' "$NEWSFED_METADATA_DSN" 2>/dev/null || stat -c '%a' "$NEWSFED_METADATA_DSN" 2>/dev/null)
    [ "$perm" = "600" ]
}
