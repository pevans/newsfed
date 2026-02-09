#!/bin/bash
# Setup script for CLI item command tests
# Prepares isolated test environment with sample news items

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

echo "Setting up CLI items test environment..."
echo ""

# Clean up any existing test data
if [ -d "$TEST_DIR" ]; then
    echo "Cleaning up existing test data..."
    rm -rf "$TEST_DIR"
fi

# Create fresh test directory
mkdir -p "$TEST_DIR/.news"

# Build the CLI if not already built or if source has changed
echo "Building newsfed CLI..."
(cd "$SCRIPT_DIR/../.." && go build -o "$TEST_DIR/newsfed" ./cmd/newsfed 2>&1 | grep -v "operation not permitted" || true)

# Verify the binary was created
if [ ! -f "$TEST_DIR/newsfed" ]; then
    echo "Error: Failed to build newsfed CLI"
    exit 1
fi

# Export test environment variables
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

# Create sample news items
echo "Creating sample news items..."

# Pre-calculate timestamps (macOS compatible)
ONE_DAY_AGO=$(date -u -v-1d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='1 day ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
TWO_DAYS_AGO=$(date -u -v-2d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='2 days ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)

# Item 1: Regular unpinned item
cat > "$TEST_DIR/.news/11111111-1111-1111-1111-111111111111.json" <<JSONEOF
{
  "id": "11111111-1111-1111-1111-111111111111",
  "title": "Test Article for Show Command",
  "summary": "This is a detailed summary that will be displayed in the show command output. It contains multiple sentences to test text wrapping and formatting.",
  "url": "https://example.com/test-article",
  "publisher": "Test Publisher",
  "authors": ["Alice Smith", "Bob Jones"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
JSONEOF

# Item 2: Pinned item (for testing pin/unpin)
cat > "$TEST_DIR/.news/22222222-2222-2222-2222-222222222222.json" <<JSONEOF
{
  "id": "22222222-2222-2222-2222-222222222222",
  "title": "Already Pinned Article",
  "summary": "This article is already pinned",
  "url": "https://example.com/pinned-article",
  "publisher": "Test Publisher",
  "authors": ["Charlie Brown"],
  "published_at": "$TWO_DAYS_AGO",
  "discovered_at": "$TWO_DAYS_AGO",
  "pinned_at": "$ONE_DAY_AGO"
}
JSONEOF

# Item 3: Regular unpinned item (for pin/unpin tests)
cat > "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json" <<JSONEOF
{
  "id": "33333333-3333-3333-3333-333333333333",
  "title": "Unpinned Article for Testing",
  "summary": "This article will be used to test pin and unpin commands",
  "url": "https://example.com/unpinned-article",
  "publisher": "Another Publisher",
  "authors": [],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
JSONEOF

# Item 4: Article without publisher or authors
cat > "$TEST_DIR/.news/44444444-4444-4444-4444-444444444444.json" <<JSONEOF
{
  "id": "44444444-4444-4444-4444-444444444444",
  "title": "Article Without Metadata",
  "summary": "This article has minimal metadata",
  "url": "https://example.com/minimal",
  "authors": [],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
JSONEOF

echo "✓ Test environment ready"
echo "  CLI binary: $TEST_DIR/newsfed"
echo "  Metadata DB: $NEWSFED_METADATA_DSN"
echo "  Feed storage: $NEWSFED_FEED_DSN"
echo "  Sample items: 4 items created"
echo ""
