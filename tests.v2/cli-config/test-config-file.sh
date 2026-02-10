#!/bin/bash
# Test CLI: Configuration File Support
# RFC 8, Section 4.1

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Create a fake HOME directory for testing
FAKE_HOME="$TEST_DIR/fake-home"
mkdir -p "$FAKE_HOME/.newsfed"
export HOME="$FAKE_HOME"

# Clear any environment variables to ensure config file is used
unset NEWSFED_METADATA_TYPE
unset NEWSFED_METADATA_DSN
unset NEWSFED_FEED_TYPE
unset NEWSFED_FEED_DSN

# Test 1: Config file with full configuration works
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    type: "sqlite"
    dsn: "$TEST_DIR/config-file-test/metadata.db"
  feed:
    type: "file"
    dsn: "$TEST_DIR/config-file-test/.news"
EOF

# Replace $TEST_DIR with actual value
sed -i.bak "s|\$TEST_DIR|$TEST_DIR|g" "$FAKE_HOME/.newsfed/config.yaml"
rm "$FAKE_HOME/.newsfed/config.yaml.bak"

# Create parent directories
mkdir -p "$TEST_DIR/config-file-test"

# Should successfully add a source using config file
output=$(newsfed sources add --type=rss --url="https://example.com/feed.xml" --name="Config File Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata DB was created at config file location
test_file_exists "Metadata DB created from config file" "$TEST_DIR/config-file-test/metadata.db"

# Verify source can be listed
output=$(newsfed sources list 2>&1)
test_contains "Source from config file appears" "$output" "Config File Source"

# Test 2: Partial config file (only metadata) uses defaults for feed
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    type: "sqlite"
    dsn: "$TEST_DIR/config-file-test2/metadata.db"
EOF

sed -i.bak "s|\$TEST_DIR|$TEST_DIR|g" "$FAKE_HOME/.newsfed/config.yaml"
rm "$FAKE_HOME/.newsfed/config.yaml.bak"

# Create parent directories
mkdir -p "$TEST_DIR/config-file-test2"

# Should use config for metadata but defaults for feed
mkdir -p "$PWD/.news"  # Default feed location
output=$(newsfed sources add --type=rss --url="https://example.com/partial.xml" --name="Partial Config Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata DB created at config location
test_file_exists "Metadata DB from partial config" "$TEST_DIR/config-file-test2/metadata.db"

# Test 3: Empty/missing config file uses defaults
rm -f "$FAKE_HOME/.newsfed/config.yaml"

# Export explicit paths since defaults would pollute working directory
export NEWSFED_METADATA_DSN="$TEST_DIR/no-config/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/no-config/.news"

# Create parent directories
mkdir -p "$TEST_DIR/no-config"

output=$(newsfed sources add --type=rss --url="https://example.com/noconfig.xml" --name="No Config Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify it worked with explicit env vars (since config file doesn't exist)
output=$(newsfed sources list 2>&1)
test_contains "No config file uses env vars" "$output" "No Config Source"

# Test 4: Invalid YAML in config file shows error
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    - invalid
    - yaml
    - structure
EOF

# Clear env vars to force config file usage
unset NEWSFED_METADATA_DSN
unset NEWSFED_FEED_DSN

# Should show config file parse error (but continue with defaults)
output=$(newsfed sources list 2>&1) || exit_code=$?
test_contains "Invalid config shows warning" "$output" "failed to load config file"

exit $FAILED
