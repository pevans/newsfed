#!/bin/bash
# Test RFC 6 Section 6 - CORS Support

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 6 - CORS Support"
echo "======================================="
echo ""

# Test 1: GET /api/v1/meta/sources includes CORS headers
echo "Test 1: GET /api/v1/meta/sources includes CORS headers"
headers=$(curl -s -D - "$BASE_URL/sources" -o /dev/null)
if echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    printf "\033[32m✓\033[0m %s\n" "Access-Control-Allow-Origin header present"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Access-Control-Allow-Origin header missing"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

# Test 2: POST /api/v1/meta/sources includes CORS headers
echo "Test 2: POST /api/v1/meta/sources includes CORS headers"
headers=$(curl -s -D - -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/cors-test.xml",
        "name": "CORS Test Feed"
    }' -o /dev/null)
if echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    printf "\033[32m✓\033[0m %s\n" "Access-Control-Allow-Origin header present on POST"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Access-Control-Allow-Origin header missing on POST"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

# Test 3: OPTIONS /api/v1/meta/sources (preflight request)
echo "Test 3: OPTIONS /api/v1/meta/sources (preflight request)"
status=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE_URL/sources")
if [ "$status" -eq 200 ] || [ "$status" -eq 204 ]; then
    printf "\033[32m✓\033[0m %s\n" "OPTIONS request returns 200/204 OK"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "OPTIONS request failed with status $status"
    FAILED=$((FAILED + 1))
fi

# Test 4: OPTIONS response includes required CORS headers
echo "Test 4: OPTIONS response includes required CORS headers"
headers=$(curl -s -D - -X OPTIONS "$BASE_URL/sources" -o /dev/null)
missing_headers=()

if ! echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    missing_headers+=("Access-Control-Allow-Origin")
fi
if ! echo "$headers" | grep -qi "Access-Control-Allow-Methods"; then
    missing_headers+=("Access-Control-Allow-Methods")
fi
if ! echo "$headers" | grep -qi "Access-Control-Allow-Headers"; then
    missing_headers+=("Access-Control-Allow-Headers")
fi

if [ ${#missing_headers[@]} -eq 0 ]; then
    printf "\033[32m✓\033[0m %s\n" "All required CORS headers present in OPTIONS response"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Missing CORS headers: ${missing_headers[*]}"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

# Test 5: OPTIONS response includes appropriate methods
echo "Test 5: OPTIONS response includes appropriate methods"
headers=$(curl -s -D - -X OPTIONS "$BASE_URL/sources" -o /dev/null)
methods_header=$(echo "$headers" | grep -i "Access-Control-Allow-Methods" || echo "")

if echo "$methods_header" | grep -qi "GET" && \
   echo "$methods_header" | grep -qi "POST" && \
   echo "$methods_header" | grep -qi "PUT" && \
   echo "$methods_header" | grep -qi "DELETE"; then
    printf "\033[32m✓\033[0m %s\n" "All HTTP methods (GET, POST, PUT, DELETE) included in Access-Control-Allow-Methods"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Not all required methods present in Access-Control-Allow-Methods"
    echo "  Methods header: $methods_header"
    FAILED=$((FAILED + 1))
fi

# Test 6: PUT /api/v1/meta/sources/{id} includes CORS headers
echo "Test 6: PUT /api/v1/meta/sources/{id} includes CORS headers"
# Create a source first
source_id=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/cors-put-test.xml",
        "name": "CORS PUT Test"
    }' | jq -r '.source_id')

headers=$(curl -s -D - -X PUT "$BASE_URL/sources/$source_id" \
    -H "Content-Type: application/json" \
    -d '{"name": "Updated Name"}' -o /dev/null)
if echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    printf "\033[32m✓\033[0m %s\n" "Access-Control-Allow-Origin header present on PUT"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Access-Control-Allow-Origin header missing on PUT"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

# Test 7: DELETE /api/v1/meta/sources/{id} includes CORS headers
echo "Test 7: DELETE /api/v1/meta/sources/{id} includes CORS headers"
headers=$(curl -s -D - -X DELETE "$BASE_URL/sources/$source_id" -o /dev/null)
if echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    printf "\033[32m✓\033[0m %s\n" "Access-Control-Allow-Origin header present on DELETE"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Access-Control-Allow-Origin header missing on DELETE"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

# Test 8: GET /api/v1/meta/config includes CORS headers
echo "Test 8: GET /api/v1/meta/config includes CORS headers"
headers=$(curl -s -D - "$BASE_URL/config" -o /dev/null)
if echo "$headers" | grep -qi "Access-Control-Allow-Origin"; then
    printf "\033[32m✓\033[0m %s\n" "Access-Control-Allow-Origin header present on config endpoint"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Access-Control-Allow-Origin header missing on config endpoint"
    echo "  Headers: $headers"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "======================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "======================================="

exit $FAILED
