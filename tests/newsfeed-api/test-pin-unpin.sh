#!/bin/bash
# Black box tests for RFC 4 sections 3.3 and 3.4 - Pin/Unpin Items

BASE_URL="http://localhost:8080/api/v1"
PASSED=0
FAILED=0

# Test UUID from setup.sh (unpinned item)
TEST_ID="550e8400-e29b-41d4-a716-446655440000"
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

echo "Testing RFC 4 Sections 3.3 and 3.4 - Pin/Unpin Items"
echo "====================================================="
echo ""

# Test 1: Pin an item
echo "Test 1: POST /api/v1/items/{id}/pin"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/items/$TEST_ID/pin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    PINNED_AT=$(echo "$BODY" | jq -r '.pinned_at')
    if [ "$PINNED_AT" != "null" ] && [ -n "$PINNED_AT" ]; then
        pass "Returns 200 OK and sets pinned_at"
    else
        fail "pinned_at not set after pinning"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 2: Verify item is pinned
echo ""
echo "Test 2: Verify item is pinned (GET after POST /pin)"
RESPONSE=$(curl -s "$BASE_URL/items/$TEST_ID")
PINNED_AT=$(echo "$RESPONSE" | jq -r '.pinned_at')

if [ "$PINNED_AT" != "null" ] && [ -n "$PINNED_AT" ]; then
    pass "Item remains pinned after GET request"
else
    fail "Item not pinned after pin operation"
fi

# Test 3: Unpin the item
echo ""
echo "Test 3: POST /api/v1/items/{id}/unpin"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/items/$TEST_ID/unpin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    PINNED_AT=$(echo "$BODY" | jq -r '.pinned_at')
    if [ "$PINNED_AT" = "null" ]; then
        pass "Returns 200 OK and sets pinned_at to null"
    else
        fail "pinned_at not null after unpinning"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 4: Verify item is unpinned
echo ""
echo "Test 4: Verify item is unpinned (GET after POST /unpin)"
RESPONSE=$(curl -s "$BASE_URL/items/$TEST_ID")
PINNED_AT=$(echo "$RESPONSE" | jq -r '.pinned_at')

if [ "$PINNED_AT" = "null" ]; then
    pass "Item remains unpinned after GET request"
else
    fail "Item still pinned after unpin operation"
fi

# Test 5: Pin non-existent item
echo ""
echo "Test 5: POST /api/v1/items/{non_existent_id}/pin"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/items/$INVALID_ID/pin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "404" ]; then
    pass "Returns 404 when pinning non-existent item"
else
    fail "Expected 404, got $HTTP_CODE"
fi

# Test 6: Unpin non-existent item
echo ""
echo "Test 6: POST /api/v1/items/{non_existent_id}/unpin"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/items/$INVALID_ID/unpin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "404" ]; then
    pass "Returns 404 when unpinning non-existent item"
else
    fail "Expected 404, got $HTTP_CODE"
fi

# Test 7: Wrong HTTP method on pin endpoint
echo ""
echo "Test 7: GET /api/v1/items/{id}/pin (wrong method)"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/items/$TEST_ID/pin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "405" ]; then
    pass "Returns 405 for wrong HTTP method"
else
    fail "Expected 405, got $HTTP_CODE"
fi

# Test 8: Pin already pinned item (idempotency)
echo ""
echo "Test 8: Pin already pinned item (idempotency)"
# First pin it
curl -s -X POST "$BASE_URL/items/$TEST_ID/pin" > /dev/null
# Pin it again
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/items/$TEST_ID/pin")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Pinning already pinned item returns 200 OK"
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Cleanup: Unpin the test item
curl -s -X POST "$BASE_URL/items/$TEST_ID/unpin" > /dev/null

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
