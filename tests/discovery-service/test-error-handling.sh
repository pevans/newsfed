#!/bin/bash
# Black box tests for RFC 7 Section 7 - Error Handling

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

echo "Testing RFC 7 Section 7 - Error Handling"
echo "========================================="
echo ""

# Re-enable and reset sources that may have been disabled in previous tests
cat > reset-error-sources.go <<'GOSCRIPT'
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

	sources, err := store.ListSources()
	if err != nil {
		log.Fatalf("Failed to list sources: %v", err)
	}

	now := time.Now()
	past := now.Add(-2 * time.Hour) // Make sources due by setting last_fetched_at in past

	// Re-enable all sources and reset fetch times
	for _, source := range sources {
		updates := map[string]any{
			"enabled_at": &now,
			"fetch_error_count": 0,
			"last_fetched_at": &past, // Set to past to make source due
		}
		if err := store.UpdateSource(source.SourceID, updates); err != nil {
			log.Printf("Failed to reset source %s: %v", source.Name, err)
		}
	}
}
GOSCRIPT

go run reset-error-sources.go > /dev/null 2>&1
rm reset-error-sources.go

# Run discovery service once to process sources (including invalid ones)
echo "Running discovery service to test error handling..."
./newsfed-discover \
    -metadata="$METADATA_DB" \
    -feed="$FEED_DIR" \
    -poll-interval="1h" \
    > error-test.log 2>&1 &
ERROR_PID=$!

# Give it enough time to fetch all sources (including failures)
sleep 5

# Kill the service
kill $ERROR_PID 2>/dev/null
wait $ERROR_PID 2>/dev/null

# Test 1: Invalid feed errors are logged
echo ""
echo "Test 1: Invalid feed errors are logged"
if grep -q "ERROR.*Failed to fetch source.*Invalid Test Feed" error-test.log; then
    pass "Invalid feed error was logged"
else
    fail "Invalid feed error not logged"
fi

# Test 2: Non-existent feed errors are logged
echo ""
echo "Test 2: Non-existent feed errors are logged"
if grep -q "ERROR.*Failed to fetch source.*Nonexistent Feed" error-test.log; then
    pass "Non-existent feed error was logged"
else
    fail "Non-existent feed error not logged"
fi

# Test 3: Error metadata is recorded
echo ""
echo "Test 3: Error count is incremented for failed sources"
cat > check-errors.go <<'GOSCRIPT'
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

	// Find sources with errors
	errorCount := 0
	for _, source := range sources {
		if source.FetchErrorCount > 0 {
			errorCount++
			fmt.Printf("Source '%s' has %d errors\n", source.Name, source.FetchErrorCount)
		}
	}

	if errorCount >= 2 {
		fmt.Println("PASS")
	} else {
		fmt.Printf("FAIL: Expected at least 2 sources with errors, got %d\n", errorCount)
	}
}
GOSCRIPT

RESULT=$(go run check-errors.go 2>/dev/null | tail -1)
rm check-errors.go
if [ "$RESULT" = "PASS" ]; then
    pass "Error counts were incremented"
else
    fail "$RESULT"
fi

# Test 4: Successful source has no errors
echo ""
echo "Test 4: Successful source has error count of 0"
cat > check-success.go <<'GOSCRIPT'
package main

import (
	"fmt"
	"log"
	"strings"
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

	// Find the valid feed source
	for _, source := range sources {
		if strings.Contains(source.Name, "Valid Test Feed") {
			if source.FetchErrorCount == 0 {
				fmt.Println("PASS")
				return
			} else {
				fmt.Printf("FAIL: Valid source has error count %d\n", source.FetchErrorCount)
				return
			}
		}
	}
	fmt.Println("FAIL: Valid Test Feed source not found")
}
GOSCRIPT

RESULT=$(go run check-success.go 2>/dev/null)
rm check-success.go
if [ "$RESULT" = "PASS" ]; then
    pass "Successful source has error count 0"
else
    fail "$RESULT"
fi

# Test 5: Last error message is recorded
echo ""
echo "Test 5: Last error message is recorded for failed sources"
cat > check-error-msg.go <<'GOSCRIPT'
package main

import (
	"fmt"
	"log"
	"strings"
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

	// Find sources with errors and check last_error is set
	hasErrorMsg := false
	for _, source := range sources {
		if source.FetchErrorCount > 0 && source.LastError != nil && *source.LastError != "" {
			hasErrorMsg = true
			if strings.Contains(source.Name, "Invalid") {
				fmt.Printf("Invalid feed error: %s\n", *source.LastError)
			}
		}
	}

	if hasErrorMsg {
		fmt.Println("PASS")
	} else {
		fmt.Println("FAIL: No error messages recorded")
	}
}
GOSCRIPT

RESULT=$(go run check-error-msg.go 2>/dev/null | tail -1)
rm check-error-msg.go
if [ "$RESULT" = "PASS" ]; then
    pass "Error messages are recorded"
else
    fail "$RESULT"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

# Cleanup
rm -f error-test.log

exit $FAILED
