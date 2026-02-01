#!/bin/bash
# Black box tests for RFC 4 section 3.1 - List News Items

BASE_URL="http://localhost:8080/api/v1"
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

echo "Testing RFC 4 Section 3.1 - List News Items"
echo "============================================"
echo ""

# Test 1: Basic list (no parameters)
echo "Test 1: GET /api/v1/items (no parameters)"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    if echo "$BODY" | jq -e '.items | length >= 0' > /dev/null 2>&1; then
        pass "Returns 200 OK with items array"
    else
        fail "Response missing items array"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 2: Filter by pinned=true
echo ""
echo "Test 2: GET /api/v1/items?pinned=true"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?pinned=true")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    # Check that all items have pinned_at not null
    if echo "$BODY" | jq -e '.items | all(.pinned_at)' >/dev/null 2>&1; then
        pass "Filters pinned items correctly"
    else
        fail "Response contains unpinned items"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 3: Filter by pinned=false
echo ""
echo "Test 3: GET /api/v1/items?pinned=false"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?pinned=false")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    # Check that all items have pinned_at null
    if echo "$BODY" | jq -e '.items | all(.pinned_at | not)' > /dev/null 2>&1; then
        pass "Filters unpinned items correctly"
    else
        fail "Response contains pinned items"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 4: Filter by publisher
echo ""
echo "Test 4: GET /api/v1/items?publisher=Test+Publisher"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?publisher=Test+Publisher")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    if echo "$BODY" | jq -e '.items | all(.publisher == "Test Publisher")' > /dev/null 2>&1; then
        pass "Filters by publisher correctly"
    else
        fail "Response contains items from other publishers"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 5: Filter by author
echo ""
echo "Test 5: GET /api/v1/items?author=Alice"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?author=Alice")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    if echo "$BODY" | jq -e '.items | all(.authors | contains(["Alice"]))' > /dev/null 2>&1; then
        pass "Filters by author correctly"
    else
        fail "Response contains items without Alice as author"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 6: Pagination with limit
echo ""
echo "Test 6: GET /api/v1/items?limit=2"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?limit=2")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    ITEM_COUNT=$(echo "$BODY" | jq '.items | length')
    if [ "$ITEM_COUNT" -le 2 ]; then
        pass "Respects limit parameter"
    else
        fail "Returned $ITEM_COUNT items, expected <= 2"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 7: Pagination with offset
echo ""
echo "Test 7: GET /api/v1/items?offset=1&limit=1"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?offset=1&limit=1")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    OFFSET=$(echo "$BODY" | jq '.offset')
    if [ "$OFFSET" = "1" ]; then
        pass "Returns correct offset in response"
    else
        fail "Expected offset=1, got $OFFSET"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 8: Sort by published_desc (default)
echo ""
echo "Test 8: GET /api/v1/items?sort=published_desc"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?sort=published_desc")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    # Check that items are sorted by published_at descending
    FIRST_DATE=$(echo "$BODY" | jq -r '.items[0].published_at // empty')
    SECOND_DATE=$(echo "$BODY" | jq -r '.items[1].published_at // empty')
    if [ -n "$FIRST_DATE" ] && [ -n "$SECOND_DATE" ]; then
        if [[ "$FIRST_DATE" > "$SECOND_DATE" ]] || [[ "$FIRST_DATE" == "$SECOND_DATE" ]]; then
            pass "Sorts by published_desc correctly"
        else
            fail "Items not sorted by published_desc"
        fi
    else
        pass "Sorting test (insufficient items to verify order)"
    fi
else
    fail "Expected 200, got $HTTP_CODE"
fi

# Test 9: Invalid parameter handling
echo ""
echo "Test 9: GET /api/v1/items?limit=invalid"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/items?limit=invalid")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "400" ]; then
    pass "Returns 400 for invalid limit parameter"
else
    fail "Expected 400, got $HTTP_CODE"
fi

# Test 10: Response structure
echo ""
echo "Test 10: Verify response structure has all required fields"
RESPONSE=$(curl -s "$BASE_URL/items")
if echo "$RESPONSE" | jq -e '.items, .total, .limit, .offset' > /dev/null 2>&1; then
    pass "Response has all required fields (items, total, limit, offset)"
else
    fail "Response missing required fields"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
