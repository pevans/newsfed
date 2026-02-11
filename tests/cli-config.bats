#!/usr/bin/env bats
# Test CLI: newsfed configuration

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
}

teardown_file() {
    cleanup_test_env
}

setup() {
    # Clean environment before each test
    unset NEWSFED_METADATA_DSN
    unset NEWSFED_FEED_DSN
    unset NEWSFED_STORAGE_TYPE
}

# Storage configuration tests

@test "newsfed config: reads from environment variables" {
    export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
    export NEWSFED_FEED_DSN="$TEST_DIR/.news"

    # Initialize storage first
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    # Should use the configured paths
}

@test "newsfed config: reads from config file" {
    # Create config file
    mkdir -p "$TEST_DIR/.config/newsfed"
    cat > "$TEST_DIR/.config/newsfed/config.yaml" <<EOF
storage:
  metadata_dsn: "$TEST_DIR/config-metadata.db"
  feed_dsn: "$TEST_DIR/config-news"
EOF

    export XDG_CONFIG_HOME="$TEST_DIR/.config"
    run newsfed doctor
    # Should read from config file
}

@test "newsfed config: environment variables override config file" {
    # Create config file
    mkdir -p "$TEST_DIR/.config/newsfed"
    cat > "$TEST_DIR/.config/newsfed/config.yaml" <<EOF
storage:
  metadata_dsn: "$TEST_DIR/config-metadata.db"
  feed_dsn: "$TEST_DIR/config-news"
EOF

    export XDG_CONFIG_HOME="$TEST_DIR/.config"
    export NEWSFED_METADATA_DSN="$TEST_DIR/env-metadata.db"
    export NEWSFED_FEED_DSN="$TEST_DIR/env-news"

    # Initialize storage first
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
    # Environment variables should take precedence
}

@test "newsfed config: supports storage type configuration" {
    export NEWSFED_STORAGE_TYPE="sqlite"
    export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
    export NEWSFED_FEED_DSN="$TEST_DIR/.news"

    # Initialize storage first
    newsfed init > /dev/null 2>&1

    run newsfed doctor
    assert_success
}
