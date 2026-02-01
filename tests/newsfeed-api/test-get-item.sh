#!/bin/bash
# Black box tests for RFC 4 section 3.2 - Get News Item by ID

BASE_URL="http://localhost:8080/api/v1"
PASSED=0
FAILED=0

# Test UUIDs from setup.sh
VALID_ID="550e8400-e29b-41d4-a716-446655440000"
INVALID_ID="00000000-0000-0000-0000-000000000000"

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

echo "Testing RFC 4 Section 3.2 - Get News Item by ID"
echo "================================================"
echo ""

# Test 1: Get existing item
echo "Test 1: GET /api/v1/items/{valid_id}"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items/$VALID_ID")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    RETURNED_ID=$(echo "$BODY" | jq -r '.id')
    if [ "$RETURNED_ID" = "$VALID_ID" ]; then
        pass "Returns 200 OK with correct item"
    else
        fail "Returned wrong item ID: $RETURNED_ID"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 2: Item has all required fields
echo ""
echo "Test 2: Verify item has all required fields"
RESPONSE=$(curl -s "$BASE_URL/items/$VALID_ID")
if echo "$RESPONSE" | jq -e '.id, .title, .summary, .url, .authors, .published_at, .discovered_at' > /dev/null 2>&1; then
    pass "Item has all required fields"
else
    fail "Item missing required fields"
fi

# Test 3: Get non-existent item
echo ""
echo "Test 3: GET /api/v1/items/{non_existent_id}"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items/$INVALID_ID")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "404" ]; then
    pass "Returns 404 for non-existent item"
else
    fail "Expected 404, got $HTTP_CODE"
fi

# Test 4: Invalid UUID format
echo ""
echo "Test 4: GET /api/v1/items/invalid-uuid"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items/invalid-uuid")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "400" ]; then
    pass "Returns 400 for invalid UUID format"
else
    fail "Expected 400, got $HTTP_CODE"
fi

# Test 5: Error response format
echo ""
echo "Test 5: Verify error response format for 404"
RESPONSE=$(curl -s "$BASE_URL/items/$INVALID_ID")
if echo "$RESPONSE" | jq -e '.error.code, .error.message' > /dev/null 2>&1; then
    pass "Error response has correct format (code, message)"
else
    fail "Error response missing required fields"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
