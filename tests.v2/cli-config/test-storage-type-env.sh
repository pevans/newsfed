#!/bin/bash
# Test CLI: Storage Type Environment Variables
# RFC 8, Section 4.1

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Test 1: Valid NEWSFED_METADATA_TYPE (sqlite) works
export NEWSFED_METADATA_TYPE="sqlite"
export NEWSFED_METADATA_DSN="$TEST_DIR/type-test/metadata.db"
export NEWSFED_FEED_TYPE="file"
export NEWSFED_FEED_DSN="$TEST_DIR/type-test/.news"

# Create parent directories
mkdir -p "$TEST_DIR/type-test"

# Should successfully add a source
output=$(newsfed sources add --type=rss --url="https://example.com/feed.xml" --name="Type Test Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify it worked
output=$(newsfed sources list 2>&1)
test_contains "Valid sqlite type works" "$output" "Type Test Source"

# Test 2: Invalid NEWSFED_METADATA_TYPE fails with error
export NEWSFED_METADATA_TYPE="postgres"
export NEWSFED_METADATA_DSN="$TEST_DIR/type-test2/metadata.db"
export NEWSFED_FEED_TYPE="file"
export NEWSFED_FEED_DSN="$TEST_DIR/type-test2/.news"

# Create parent directories
mkdir -p "$TEST_DIR/type-test2"

# Should fail with unsupported type error
output=$(newsfed sources list 2>&1) || exit_code=$?
test_contains "Unsupported metadata type shows error" "$output" "unsupported metadata storage type"
test_contains "Error mentions postgres" "$output" "postgres"
test_contains "Error shows supported types" "$output" "Supported types: sqlite"

# Test 3: Invalid NEWSFED_FEED_TYPE fails with error
export NEWSFED_METADATA_TYPE="sqlite"
export NEWSFED_METADATA_DSN="$TEST_DIR/type-test3/metadata.db"
export NEWSFED_FEED_TYPE="sqlite"
export NEWSFED_FEED_DSN="$TEST_DIR/type-test3/feed.db"

# Create parent directories
mkdir -p "$TEST_DIR/type-test3"

# Should fail with unsupported type error
output=$(newsfed sources list 2>&1) || exit_code=$?
test_contains "Unsupported feed type shows error" "$output" "unsupported feed storage type"
test_contains "Error mentions sqlite feed type" "$output" "sqlite"
test_contains "Error shows supported feed types" "$output" "Supported types: file"

# Test 4: Default types work when not specified
unset NEWSFED_METADATA_TYPE
unset NEWSFED_FEED_TYPE
export NEWSFED_METADATA_DSN="$TEST_DIR/type-test4/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/type-test4/.news"

# Create parent directories
mkdir -p "$TEST_DIR/type-test4"

# Should use defaults (sqlite for metadata, file for feed)
output=$(newsfed sources add --type=rss --url="https://example.com/default.xml" --name="Default Type Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify it worked with defaults
output=$(newsfed sources list 2>&1)
test_contains "Default types work" "$output" "Default Type Source"

exit $FAILED
