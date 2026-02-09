#!/bin/bash
# Setup script for CLI list command tests
# Prepares isolated test environment with sample news items

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

echo "Setting up CLI list test environment..."
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
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

# Create sample news items with various characteristics
echo "Creating sample news items..."

# Pre-calculate timestamps (macOS compatible)
ONE_DAY_AGO=$(date -u -v-1d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='1 day ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
TWO_DAYS_AGO=$(date -u -v-2d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='2 days ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
TEN_DAYS_AGO=$(date -u -v-10d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='10 days ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
TWO_HOURS_AGO=$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='2 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
TWELVE_HOURS_AGO=$(date -u -v-12H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u --date='12 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)

# Item 1: Recent item (1 day old), Publisher A
cat > "$TEST_DIR/.news/11111111-1111-1111-1111-111111111111.json" <<JSONEOF
{
  "id": "11111111-1111-1111-1111-111111111111",
  "title": "Recent Article from Publisher A",
  "summary": "This is a recent article",
  "url": "https://example.com/recent-a",
  "publisher": "Publisher A",
  "authors": ["Author One"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
JSONEOF

# Item 2: Recent item (2 days old), Publisher B
cat > "$TEST_DIR/.news/22222222-2222-2222-2222-222222222222.json" <<JSONEOF
{
  "id": "22222222-2222-2222-2222-222222222222",
  "title": "Recent Article from Publisher B",
  "summary": "Another recent article",
  "url": "https://example.com/recent-b",
  "publisher": "Publisher B",
  "authors": ["Author Two"],
  "published_at": "$TWO_DAYS_AGO",
  "discovered_at": "$TWO_DAYS_AGO"
}
JSONEOF

# Item 3: Old item (10 days old), Publisher A
cat > "$TEST_DIR/.news/33333333-3333-3333-3333-333333333333.json" <<JSONEOF
{
  "id": "33333333-3333-3333-3333-333333333333",
  "title": "Old Article from Publisher A",
  "summary": "This is an old article",
  "url": "https://example.com/old-a",
  "publisher": "Publisher A",
  "authors": ["Author One"],
  "published_at": "$TEN_DAYS_AGO",
  "discovered_at": "$TEN_DAYS_AGO"
}
JSONEOF

# Item 4: Old pinned item (10 days old), Publisher C
cat > "$TEST_DIR/.news/44444444-4444-4444-4444-444444444444.json" <<JSONEOF
{
  "id": "44444444-4444-4444-4444-444444444444",
  "title": "Old Pinned Article",
  "summary": "This is an old but pinned article",
  "url": "https://example.com/old-pinned",
  "publisher": "Publisher C",
  "authors": ["Author Three"],
  "published_at": "$TEN_DAYS_AGO",
  "discovered_at": "$TEN_DAYS_AGO",
  "pinned_at": "$ONE_DAY_AGO"
}
JSONEOF

# Item 5: Recent pinned item (1 day old), Publisher B
cat > "$TEST_DIR/.news/55555555-5555-5555-5555-555555555555.json" <<JSONEOF
{
  "id": "55555555-5555-5555-5555-555555555555",
  "title": "Recent Pinned Article",
  "summary": "This is a recent pinned article",
  "url": "https://example.com/recent-pinned",
  "publisher": "Publisher B",
  "authors": ["Author Two"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO",
  "pinned_at": "$TWO_HOURS_AGO"
}
JSONEOF

# Item 6: Very recent item (12 hours old), no publisher
cat > "$TEST_DIR/.news/66666666-6666-6666-6666-666666666666.json" <<JSONEOF
{
  "id": "66666666-6666-6666-6666-666666666666",
  "title": "Very Recent Article No Publisher",
  "summary": "This is a very recent article without publisher",
  "url": "https://example.com/very-recent",
  "authors": [],
  "published_at": "$TWELVE_HOURS_AGO",
  "discovered_at": "$TWELVE_HOURS_AGO"
}
JSONEOF

echo "✓ Test environment ready"
echo "  CLI binary: $TEST_DIR/newsfed"
echo "  Feed storage: $NEWSFED_FEED_DSN"
echo "  Sample items: 6 items created"
echo ""
