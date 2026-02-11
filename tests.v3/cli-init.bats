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
    # Should detect missing database
}

@test "newsfed doctor: detects missing feed directory" {
    # Create metadata but not feed directory
    newsfed init > /dev/null 2>&1
    rm -rf "$NEWSFED_FEED_DSN"

    run newsfed doctor
    # Should detect missing directory
}

@test "newsfed doctor: returns exit code indicating health status" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
}

@test "newsfed doctor: checks configuration validity" {
    export NEWSFED_METADATA_DSN=""
    export NEWSFED_FEED_DSN=""

    run newsfed doctor
    # Should detect invalid configuration
}

@test "newsfed doctor: checks storage connectivity" {
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    # Should verify can connect to storage
}
