#!/bin/bash
# Black box tests for RFC 4 section 6 - CORS Support

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

echo "Testing RFC 4 Section 6 - CORS Support"
echo "======================================"
echo ""

# Test 1: GET request has CORS headers
echo "Test 1: GET /api/v1/items includes CORS headers"
RESPONSE=$(curl -s -i "$BASE_URL/items")

if echo "$RESPONSE" | grep -i "Access-Control-Allow-Origin: \*" > /dev/null; then
    pass "Access-Control-Allow-Origin header present"
else
    fail "Missing Access-Control-Allow-Origin header"
fi

# Test 2: POST request has CORS headers
echo ""
echo "Test 2: POST /api/v1/items/{id}/pin includes CORS headers"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/items/550e8400-e29b-41d4-a716-446655440000/pin")

if echo "$RESPONSE" | grep -i "Access-Control-Allow-Origin: \*" > /dev/null; then
    pass "Access-Control-Allow-Origin header present on POST"
else
    fail "Missing Access-Control-Allow-Origin header on POST"
fi

# Test 3: OPTIONS preflight request on /items
echo ""
echo "Test 3: OPTIONS /api/v1/items (preflight request)"
RESPONSE=$(curl -s -i -X OPTIONS \
    -H "Origin: http://example.com" \
    -H "Access-Control-Request-Method: GET" \
    "$BASE_URL/items")

HTTP_LINE=$(echo "$RESPONSE" | head -n1)
if echo "$HTTP_LINE" | grep "200 OK" > /dev/null; then
    pass "OPTIONS request returns 200 OK"
else
    fail "OPTIONS request did not return 200 OK"
fi

# Test 4: OPTIONS preflight has required CORS headers
echo ""
echo "Test 4: OPTIONS response includes required CORS headers"
RESPONSE=$(curl -s -i -X OPTIONS \
    -H "Origin: http://example.com" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Content-Type" \
    "$BASE_URL/items/550e8400-e29b-41d4-a716-446655440000/pin")

HEADERS_OK=true
if ! echo "$RESPONSE" | grep -i "Access-Control-Allow-Origin: \*" > /dev/null; then
    HEADERS_OK=false
fi
if ! echo "$RESPONSE" | grep -i "Access-Control-Allow-Methods:" > /dev/null; then
    HEADERS_OK=false
fi
if ! echo "$RESPONSE" | grep -i "Access-Control-Allow-Headers:" > /dev/null; then
    HEADERS_OK=false
fi

if [ "$HEADERS_OK" = true ]; then
    pass "All required CORS headers present in OPTIONS response"
else
    fail "Missing one or more required CORS headers"
fi

# Test 5: OPTIONS allows POST method
echo ""
echo "Test 5: OPTIONS response includes POST in allowed methods"
RESPONSE=$(curl -s -i -X OPTIONS \
    -H "Access-Control-Request-Method: POST" \
    "$BASE_URL/items/550e8400-e29b-41d4-a716-446655440000/pin")

if echo "$RESPONSE" | grep -i "Access-Control-Allow-Methods:.*POST" > /dev/null; then
    pass "POST method included in Access-Control-Allow-Methods"
else
    fail "POST method not included in Access-Control-Allow-Methods"
fi

# Summary
echo ""
echo "======================================"
echo "Results: $PASSED passed, $FAILED failed"
echo "======================================"

exit $FAILED
