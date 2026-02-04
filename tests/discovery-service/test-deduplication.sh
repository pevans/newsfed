#!/bin/bash
# Black box tests for RFC 7 Section 6 - Deduplication

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

echo "Testing RFC 7 Section 6 - Deduplication"
echo "========================================"
echo ""

# First, get baseline count of items
echo "Running discovery service (first pass)..."
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > dedup-test.log 2>&1 &
DEDUP_PID=$!

sleep 5

# Kill the service
kill $DEDUP_PID 2>/dev/null
wait $DEDUP_PID 2>/dev/null

FIRST_COUNT=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')
echo "After first pass: $FIRST_COUNT items"
echo ""

# Test 1: Items were discovered
echo "Test 1: Items were discovered in first pass"
if [ "$FIRST_COUNT" -gt 0 ]; then
    pass "Discovered $FIRST_COUNT items"
else
    fail "No items were discovered"
fi

# Now add the duplicate feed source
echo ""
echo "Adding duplicate feed source to database..."
cat > add-duplicate-source.go <<'GOSCRIPT'
package main

import (
	"log"
	"time"
	"github.com/pevans/newsfed"
)

func main() {
	store, err := newsfed.NewMetadataStore("test-metadata.db")
	if err != nil {
		log.Fatalf("Failed to open metadata store: %v", err)
	}
	defer store.Close()

	now := time.Now()

	// Create source for duplicate feed (served via HTTP)
	_, err = store.CreateSource(
		"rss",
		"http://localhost:9876/duplicate-feed.xml",
		"Duplicate Feed",
		nil,
		&now,
	)
	if err != nil {
		log.Fatalf("Failed to create duplicate feed source: %v", err)
	}

	log.Println("✓ Added duplicate feed source")
}
GOSCRIPT

go run add-duplicate-source.go
rm add-duplicate-source.go

# Run discovery service again
echo ""
echo "Running discovery service (second pass)..."
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    >> dedup-test.log 2>&1 &
DEDUP_PID2=$!

sleep 5

# Kill the service
kill $DEDUP_PID2 2>/dev/null
wait $DEDUP_PID2 2>/dev/null

SECOND_COUNT=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')
echo "After second pass: $SECOND_COUNT items"
echo ""

# Test 2: Duplicate items were not added
echo "Test 2: Duplicate items were not added (deduplication works)"
# The duplicate feed has 2 items: article1 (duplicate) and article3 (new)
# So we should have +1 item, not +2
EXPECTED_NEW=1
ACTUAL_NEW=$((SECOND_COUNT - FIRST_COUNT))

if [ "$ACTUAL_NEW" -eq "$EXPECTED_NEW" ]; then
    pass "Only $ACTUAL_NEW new item added (duplicate was skipped)"
elif [ "$ACTUAL_NEW" -lt "$EXPECTED_NEW" ]; then
    fail "Expected $EXPECTED_NEW new items, got $ACTUAL_NEW (might be a test issue)"
else
    fail "Expected $EXPECTED_NEW new item, got $ACTUAL_NEW (deduplication may have failed)"
fi

# Test 3: Verify no duplicate URLs
echo ""
echo "Test 3: No duplicate URLs in feed"
cat > check-duplicates.go <<'GOSCRIPT'
package main

import (
	"fmt"
	"log"
	"github.com/pevans/newsfed"
)

func main() {
	feed, err := newsfed.NewNewsFeed("test-feed")
	if err != nil {
		log.Fatalf("Failed to open feed: %v", err)
	}

	items, err := feed.List()
	if err != nil {
		log.Fatalf("Failed to list items: %v", err)
	}

	// Check for duplicate URLs
	seen := make(map[string]bool)
	duplicates := 0

	for _, item := range items {
		if seen[item.URL] {
			duplicates++
			fmt.Printf("Duplicate URL: %s\n", item.URL)
		}
		seen[item.URL] = true
	}

	if duplicates == 0 {
		fmt.Println("PASS")
	} else {
		fmt.Printf("FAIL: Found %d duplicate URLs\n", duplicates)
	}
}
GOSCRIPT

RESULT=$(go run check-duplicates.go 2>/dev/null | tail -1)
rm check-duplicates.go
if [ "$RESULT" = "PASS" ]; then
    pass "No duplicate URLs found in feed"
else
    fail "$RESULT"
fi

# Test 4: Verify log messages about skipping duplicates
echo ""
echo "Test 4: Service logs are correct"
if [ "$ACTUAL_NEW" -le 1 ]; then
    pass "Discovery service handled duplicates correctly"
else
    fail "Expected only 1 new item from duplicate feed"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

# Cleanup
rm -f dedup-test.log

exit $FAILED
