#!/bin/bash
# Black box tests for RFC 7 - RSS Feed Discovery

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

echo "Testing RFC 7 Section 4 - RSS/Atom Feed Processing"
echo "==================================================="
echo ""

# Test 1: Run discovery service with timeout
echo "Test 1: Discovery service runs successfully"
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > discovery.log 2>&1 &
DISCOVERY_PID=$!

# Wait for it to process sources
sleep 5

# Kill the service
kill $DISCOVERY_PID 2>/dev/null
wait $DISCOVERY_PID 2>/dev/null

# Check if it logged startup
if grep -q "Discovery service starting" discovery.log; then
    pass "Discovery service started successfully"
else
    fail "Discovery service failed to start"
    echo "Log contents:"
    cat discovery.log
fi

# Test 2: Check that items were discovered
echo ""
echo "Test 2: Items were discovered from RSS feed"
ITEM_COUNT=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')
if [ "$ITEM_COUNT" -gt 0 ]; then
    pass "Discovered $ITEM_COUNT items"
else
    fail "No items were discovered"
fi

# Test 3: Verify item structure
echo ""
echo "Test 3: Discovered items have correct structure"
if [ "$ITEM_COUNT" -gt 0 ]; then
    FIRST_ITEM=$(ls -1 "$FEED_DIR"/*.json 2>/dev/null | head -1)
    if jq -e '.id, .title, .url, .published_at, .discovered_at' "$FIRST_ITEM" > /dev/null 2>&1; then
        pass "Items have required fields (id, title, url, published_at, discovered_at)"
    else
        fail "Items missing required fields"
    fi
else
    fail "No items to verify"
fi

# Test 4: Verify metadata was updated
echo ""
echo "Test 4: Source metadata was updated after fetch"
# Use Go to check the database
cat > check-metadata.go <<'GOSCRIPT'
package main

import (
	"fmt"
	"log"
	"github.com/pevans/newsfed"
)

func main() {
	store, err := newsfed.NewMetadataStore("test-metadata.db")
	if err != nil {
		log.Fatalf("Failed to open metadata store: %v", err)
	}
	defer store.Close()

	sources, err := store.ListSources()
	if err != nil {
		log.Fatalf("Failed to list sources: %v", err)
	}

	// Check first source (valid feed)
	if len(sources) > 0 {
		source := sources[len(sources)-1] // Most recent (our valid feed)
		if source.LastFetchedAt != nil {
			fmt.Println("PASS")
		} else {
			fmt.Println("FAIL: last_fetched_at not set")
		}
	} else {
		fmt.Println("FAIL: no sources found")
	}
}
GOSCRIPT

RESULT=$(go run check-metadata.go 2>/dev/null)
rm check-metadata.go
if [ "$RESULT" = "PASS" ]; then
    pass "Source last_fetched_at was updated"
else
    fail "$RESULT"
fi

# Test 5: Verify logging
echo ""
echo "Test 5: Service logs correctly"
if grep -q "INFO.*Discovery service starting" discovery.log; then
    pass "Logged service startup"
else
    fail "Missing service startup log"
fi

if grep -q "INFO.*Fetched" discovery.log; then
    pass "Logged fetch results"
else
    fail "Missing fetch result log"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

# Cleanup
rm -f discovery.log

exit $FAILED
