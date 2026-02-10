#!/bin/bash
# Test CLI: Configuration Precedence Order
# RFC 8, Section 4.1

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Create a fake HOME directory for testing
FAKE_HOME="$TEST_DIR/fake-home-precedence"
mkdir -p "$FAKE_HOME/.newsfed"
export HOME="$FAKE_HOME"

# Test 1: Environment variables override config file
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    type: "sqlite"
    dsn: "$TEST_DIR/precedence-config/metadata.db"
  feed:
    type: "file"
    dsn: "$TEST_DIR/precedence-config/.news"
EOF

sed -i.bak "s|\$TEST_DIR|$TEST_DIR|g" "$FAKE_HOME/.newsfed/config.yaml"
rm "$FAKE_HOME/.newsfed/config.yaml.bak"

# Create parent directories
mkdir -p "$TEST_DIR/precedence-config"
mkdir -p "$TEST_DIR/precedence-env"

# Set env vars to different paths
export NEWSFED_METADATA_TYPE="sqlite"
export NEWSFED_METADATA_DSN="$TEST_DIR/precedence-env/metadata.db"
export NEWSFED_FEED_TYPE="file"
export NEWSFED_FEED_DSN="$TEST_DIR/precedence-env/.news"

# Add a source -- should use env var paths, not config file paths
output=$(newsfed sources add --type=rss --url="https://example.com/env.xml" --name="Env Override Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata DB created at ENV path (not config file path)
test_file_exists "Metadata DB at env var path" "$TEST_DIR/precedence-env/metadata.db"

# Verify config file path was NOT used
if [ -f "$TEST_DIR/precedence-config/metadata.db" ]; then
    echo "$SCRIPT_NAME: FAIL - Config file path should not be used when env vars set"
    echo "  Found unexpected file: $TEST_DIR/precedence-config/metadata.db"
    FAILED=$((FAILED + 1))
else
    PASSED=$((PASSED + 1))
fi

# Test 2: Config file overrides defaults
unset NEWSFED_METADATA_TYPE
unset NEWSFED_METADATA_DSN
unset NEWSFED_FEED_TYPE
unset NEWSFED_FEED_DSN

# Config file should now be used
output=$(newsfed sources add --type=rss --url="https://example.com/config.xml" --name="Config File Override" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata DB created at config file path (not defaults)
test_file_exists "Metadata DB at config file path" "$TEST_DIR/precedence-config/metadata.db"

# Test 3: Partial env var override (TYPE from env, DSN from config)
rm "$FAKE_HOME/.newsfed/config.yaml"
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    type: "sqlite"
    dsn: "$TEST_DIR/precedence-partial/metadata.db"
  feed:
    type: "file"
    dsn: "$TEST_DIR/precedence-partial/.news"
EOF

sed -i.bak "s|\$TEST_DIR|$TEST_DIR|g" "$FAKE_HOME/.newsfed/config.yaml"
rm "$FAKE_HOME/.newsfed/config.yaml.bak"

# Create parent directories
mkdir -p "$TEST_DIR/precedence-partial"

# Only override TYPE via env var, let DSN come from config
export NEWSFED_METADATA_TYPE="sqlite"
unset NEWSFED_METADATA_DSN
export NEWSFED_FEED_TYPE="file"
unset NEWSFED_FEED_DSN

# Should use TYPE from env, DSN from config file
output=$(newsfed sources add --type=rss --url="https://example.com/partial-env.xml" --name="Partial Env Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata DB created at config file DSN path
test_file_exists "Metadata DB at config DSN with env TYPE" "$TEST_DIR/precedence-partial/metadata.db"

# Test 4: Full precedence chain - Env > Config > Defaults
# Create new config file with one setting
cat > "$FAKE_HOME/.newsfed/config.yaml" <<'EOF'
storage:
  metadata:
    dsn: "$TEST_DIR/precedence-chain/metadata.db"
EOF

sed -i.bak "s|\$TEST_DIR|$TEST_DIR|g" "$FAKE_HOME/.newsfed/config.yaml"
rm "$FAKE_HOME/.newsfed/config.yaml.bak"

# Create parent directories
mkdir -p "$TEST_DIR/precedence-chain"

# Set only feed DSN via env (metadata DSN from config, types from defaults)
unset NEWSFED_METADATA_TYPE
unset NEWSFED_METADATA_DSN
unset NEWSFED_FEED_TYPE
export NEWSFED_FEED_DSN="$TEST_DIR/precedence-chain/.news"

# Should use:
# - METADATA_TYPE: default (sqlite)
# - METADATA_DSN: config file
# - FEED_TYPE: default (file)
# - FEED_DSN: env var
output=$(newsfed sources add --type=rss --url="https://example.com/chain.xml" --name="Chain Source" 2>&1)
SOURCE_ID=$(extract_uuid "$output")

# Verify metadata at config path
test_file_exists "Metadata DB at config path in precedence chain" "$TEST_DIR/precedence-chain/metadata.db"

# Create feed item to verify feed location
mkdir -p "$TEST_DIR/precedence-chain/.news"
cat > "$TEST_DIR/precedence-chain/.news/33333333-3333-3333-3333-333333333333.json" <<'JSONEOF'
{
  "id": "33333333-3333-3333-3333-333333333333",
  "title": "Chain Test Item",
  "summary": "Testing precedence chain",
  "url": "https://example.com/chain-item",
  "authors": [],
  "published_at": "2026-02-09T00:00:00Z",
  "discovered_at": "2026-02-09T00:00:00Z"
}
JSONEOF

# Verify item found at env var feed path
output=$(newsfed list --all 2>&1)
test_contains "Feed item at env var path in precedence chain" "$output" "Chain Test Item"

exit $FAILED
