#!/bin/bash
# Run newsfeed API black box tests

BASE_URL="http://localhost:8080/api/v1"

echo "RFC 4 API Black Box Tests"
echo "=========================="
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Setup test data
echo "Setting up test data..."
"$SCRIPT_DIR/setup.sh"
echo ""

# Run all tests
TOTAL_FAILED=0

echo "Running test suites..."
echo ""

# Test List Items (Section 3.1)
if "$SCRIPT_DIR/test-list-items.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Get Item (Section 3.2)
if "$SCRIPT_DIR/test-get-item.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Pin/Unpin (Sections 3.3 and 3.4)
if "$SCRIPT_DIR/test-pin-unpin.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test CORS Support (Section 6)
if "$SCRIPT_DIR/test-cors.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Final summary
echo ""
echo "=========================="
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "All API tests passed! ✓"
    exit 0
else
    echo "Some API tests failed ($TOTAL_FAILED total failures)"
    exit 1
fi
