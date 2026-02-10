#!/bin/bash
# Setup script for CLI error handling tests
# Prepares minimal test environment for error scenarios
# Runs silently -- only prints errors

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Clean up any existing test data
if [ -d "$TEST_DIR" ]; then
    rm -rf "$TEST_DIR"
fi

# Create fresh test directory
mkdir -p "$TEST_DIR/.news"

# Build the CLI if not already built or if source has changed
(cd "$SCRIPT_DIR/../.." && go build -o "$TEST_DIR/newsfed" ./cmd/newsfed 2>&1 | grep -v "operation not permitted" || true) > /dev/null 2>&1

# Verify the binary was created
if [ ! -f "$TEST_DIR/newsfed" ]; then
    echo "Error: Failed to build newsfed CLI" >&2
    exit 1
fi

# Export test environment variables
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

# Create a simple metadata database (SQLite)
touch "$TEST_DIR/metadata.db"

# Create a sample news item (for tests that need valid storage)
cat > "$TEST_DIR/.news/11111111-1111-1111-1111-111111111111.json" <<'JSONEOF'
{
  "id": "11111111-1111-1111-1111-111111111111",
  "title": "Sample Article",
  "summary": "Sample summary",
  "url": "https://example.com/sample",
  "publisher": "Test Publisher",
  "authors": [],
  "published_at": "2026-02-01T10:00:00Z",
  "discovered_at": "2026-02-01T10:00:00Z"
}
JSONEOF
