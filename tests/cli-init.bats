#!/usr/bin/env bats
# Test CLI: newsfed init and doctor commands

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

# Init command tests

@test "newsfed init: creates metadata database" {
    run newsfed init
    assert_success
    assert_output_contains "initialized"

    # Verify database was created
    [ -f "$NEWSFED_METADATA_DSN" ]
}

@test "newsfed init: creates feed directory" {
    run newsfed init
    assert_success

    # Verify directory was created
    [ -d "$NEWSFED_FEED_DSN" ]
}

@test "newsfed init: handles already initialized storage" {
    newsfed init > /dev/null 2>&1

    run newsfed init
    assert_success
    # Should handle gracefully
}

@test "newsfed init: returns success exit code" {
    run newsfed init
    assert_success
}

# Doctor command tests

@test "newsfed doctor: verifies healthy storage" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    # Should report storage is healthy
}

@test "newsfed doctor: detects missing metadata database" {
    mkdir -p "$NEWSFED_FEED_DSN"
    # Don't create metadata database

    run newsfed doctor
    assert_failure
    assert_output_contains "does not exist"
    assert_output_contains "Storage has errors"
}

@test "newsfed doctor: detects missing feed directory" {
    # Create metadata but not feed directory
    newsfed init > /dev/null 2>&1
    rm -rf "$NEWSFED_FEED_DSN"

    run newsfed doctor
    assert_failure
    assert_output_contains "does not exist"
    assert_output_contains "Storage has errors"
}

@test "newsfed doctor: detects feed path that is not a directory" {
    newsfed init > /dev/null 2>&1
    rm -rf "$NEWSFED_FEED_DSN"

    # Create a regular file where the directory should be
    echo "not a directory" > "$NEWSFED_FEED_DSN"

    run newsfed doctor
    assert_failure
    assert_output_contains "not a directory"
}

@test "newsfed doctor: returns exit code 0 on healthy storage" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    assert_output_contains "All checks passed"
}

@test "newsfed doctor: returns exit code 0 on warnings" {
    newsfed init > /dev/null 2>&1

    # Introduce a warning (permissive permissions) but no errors
    chmod 644 "$NEWSFED_METADATA_DSN"

    run newsfed doctor
    assert_success
    assert_output_contains "functional but has warnings"
}

@test "newsfed doctor --verbose: shows detailed diagnostic information" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor --verbose
    assert_success
    assert_output_contains "Permissions:"
}

@test "newsfed doctor: reports unreadable feed items" {
    newsfed init > /dev/null 2>&1

    # Create a malformed JSON file in the feed directory
    echo "not valid json" > "$NEWSFED_FEED_DSN/bad-item.json"

    run newsfed doctor
    assert_output_contains "could not be read"
}

@test "newsfed doctor: checks storage connectivity" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    assert_output_contains "Database is accessible"
    assert_output_contains "Storage directory is accessible"
}
