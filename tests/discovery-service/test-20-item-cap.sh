#!/bin/bash
# Black box tests for RFC 2 section 2.2.3 and RFC 3 section 3.1.1
# Tests the conditional 20-item cap on feed ingestion and web scraping
#
# The 20-item cap applies when:
# - First-time sync: source has never been fetched (last_fetched_at is null)
# - Stale source: source has not been synced for more than 15 days
#
# The cap does NOT apply for regular polling (source fetched within 15 days)

METADATA_DB="test-metadata.db"
FEED_DIR="test-feed"
PASSED=0
FAILED=0

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

echo "Testing RFC 2 Section 2.2.3 and RFC 3 Section 3.1.1 - Conditional 20 Item Cap"
echo "=============================================================================="
echo ""

# Test 1: First-time sync with more than 20 items
echo "Test 1: First-time sync with >20 items -- only 20 most recent are ingested"

# Create a test RSS feed with 30 items
cat > test-large-feed.xml <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>http://example.com</link>
    <description>Test feed with 30 items</description>
EOF

# Generate 30 items with different dates
for i in {1..30}; do
    # Create dates from 2024-01-01 to 2024-01-30
    DATE=$(date -j -f "%Y-%m-%d" "2024-01-$(printf "%02d" $i)" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    if [ -z "$DATE" ]; then
        # Fallback for non-BSD date
        DATE=$(date -d "2024-01-$(printf "%02d" $i)" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    fi

    cat >> test-large-feed.xml <<ITEM
    <item>
      <title>Article $i</title>
      <link>http://example.com/article$i</link>
      <description>Description for article $i</description>
      <pubDate>$DATE</pubDate>
    </item>
ITEM
done

cat >> test-large-feed.xml <<'EOF'
  </channel>
</rss>
EOF

# Start a simple HTTP server to serve the feed
python3 -m http.server 8888 > /dev/null 2>&1 &
HTTP_SERVER_PID=$!
sleep 2

# Add the source to metadata
./newsfed sources add \
    --type=rss \
    --url="http://localhost:8888/test-large-feed.xml" \
    --name="Large Test Feed" \
    > /dev/null 2>&1

# Run discovery service
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > discovery.log 2>&1 &
DISCOVERY_PID=$!

# Wait for processing
sleep 5

# Stop discovery service
kill $DISCOVERY_PID 2>/dev/null
wait $DISCOVERY_PID 2>/dev/null

# Stop HTTP server
kill $HTTP_SERVER_PID 2>/dev/null

# Count items in feed
ITEM_COUNT=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')

if [ "$ITEM_COUNT" -eq 20 ]; then
    pass "Exactly 20 items were ingested (feed had 30)"
else
    fail "Expected 20 items, got $ITEM_COUNT"
fi

# Test 2: Verify the 20 items are the most recent
echo ""
echo "Test 2: The 20 items ingested are the most recent (Jan 11-30)"

# Check that we have articles from the end of January (most recent)
# and not from the beginning
NEWEST_FOUND=false
OLDEST_MISSING=true

for i in {25..30}; do
    if ls "$FEED_DIR"/*.json 2>/dev/null | xargs grep -l "Article $i" > /dev/null 2>&1; then
        NEWEST_FOUND=true
        break
    fi
done

for i in {1..5}; do
    if ls "$FEED_DIR"/*.json 2>/dev/null | xargs grep -l "Article $i" > /dev/null 2>&1; then
        OLDEST_MISSING=false
        break
    fi
done

if [ "$NEWEST_FOUND" = true ] && [ "$OLDEST_MISSING" = true ]; then
    pass "Most recent items (25-30) found, oldest items (1-5) not present"
else
    fail "Items selection not correct (newest=$NEWEST_FOUND, oldest_missing=$OLDEST_MISSING)"
fi

# Test 3: Feed with exactly 20 items -- all ingested
echo ""
echo "Test 3: Feed with exactly 20 items -- all are ingested"

# Clean up
rm -rf "$FEED_DIR"/*.json 2>/dev/null
mkdir -p "$FEED_DIR"

# Create a feed with exactly 20 items
cat > test-exact-feed.xml <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Exact Feed</title>
    <link>http://example.com</link>
    <description>Test feed with exactly 20 items</description>
EOF

for i in {1..20}; do
    DATE=$(date -j -f "%Y-%m-%d" "2024-02-$(printf "%02d" $i)" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    if [ -z "$DATE" ]; then
        DATE=$(date -d "2024-02-$(printf "%02d" $i)" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    fi

    cat >> test-exact-feed.xml <<ITEM
    <item>
      <title>Exact Article $i</title>
      <link>http://example.com/exact$i</link>
      <description>Description for exact article $i</description>
      <pubDate>$DATE</pubDate>
    </item>
ITEM
done

cat >> test-exact-feed.xml <<'EOF'
  </channel>
</rss>
EOF

# Start HTTP server again
python3 -m http.server 8889 > /dev/null 2>&1 &
HTTP_SERVER_PID=$!
sleep 2

# Add another source
./newsfed sources add \
    --type=rss \
    --url="http://localhost:8889/test-exact-feed.xml" \
    --name="Exact Test Feed" \
    > /dev/null 2>&1

# Run discovery
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > discovery2.log 2>&1 &
DISCOVERY_PID=$!
sleep 5
kill $DISCOVERY_PID 2>/dev/null
wait $DISCOVERY_PID 2>/dev/null
kill $HTTP_SERVER_PID 2>/dev/null

# Count new items (should be 20 from this feed plus the 20 from before)
TOTAL_ITEMS=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')
NEW_ITEMS=$(grep -l "Exact Article" "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')

if [ "$NEW_ITEMS" -eq 20 ]; then
    pass "All 20 items from exact-size feed were ingested"
else
    fail "Expected 20 items from exact feed, got $NEW_ITEMS"
fi

# Test 4: Feed with fewer than 20 items -- all ingested
echo ""
echo "Test 4: Feed with <20 items -- all are ingested"

# Clean feed dir
rm -rf "$FEED_DIR"/*.json 2>/dev/null

# Create a feed with only 5 items
cat > test-small-feed.xml <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Small Feed</title>
    <link>http://example.com</link>
    <description>Test feed with only 5 items</description>
EOF

for i in {1..5}; do
    DATE=$(date -j -f "%Y-%m-%d" "2024-03-0$i" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    if [ -z "$DATE" ]; then
        DATE=$(date -d "2024-03-0$i" "+%a, %d %b %Y 00:00:00 GMT" 2>/dev/null)
    fi

    cat >> test-small-feed.xml <<ITEM
    <item>
      <title>Small Article $i</title>
      <link>http://example.com/small$i</link>
      <description>Description for small article $i</description>
      <pubDate>$DATE</pubDate>
    </item>
ITEM
done

cat >> test-small-feed.xml <<'EOF'
  </channel>
</rss>
EOF

# Start server
python3 -m http.server 8890 > /dev/null 2>&1 &
HTTP_SERVER_PID=$!
sleep 2

./newsfed sources add \
    --type=rss \
    --url="http://localhost:8890/test-small-feed.xml" \
    --name="Small Test Feed" \
    > /dev/null 2>&1

./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > discovery3.log 2>&1 &
DISCOVERY_PID=$!
sleep 5
kill $DISCOVERY_PID 2>/dev/null
wait $DISCOVERY_PID 2>/dev/null
kill $HTTP_SERVER_PID 2>/dev/null

SMALL_ITEMS=$(grep -l "Small Article" "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')

if [ "$SMALL_ITEMS" -eq 5 ]; then
    pass "All 5 items from small feed were ingested"
else
    fail "Expected 5 items from small feed, got $SMALL_ITEMS"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

# Cleanup
rm -f test-large-feed.xml test-exact-feed.xml test-small-feed.xml
rm -f discovery.log discovery2.log discovery3.log

exit $FAILED
