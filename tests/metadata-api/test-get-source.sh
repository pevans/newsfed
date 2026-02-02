#!/bin/bash
# Test RFC 6 Section 3.2 - Get Source by ID

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 3.2 - Get Source by ID"
echo "============================================="
echo ""

# Helper function to test HTTP responses
test_response() {
    local test_name="$1"
    local response="$2"
    local expected_status="$3"
    local check_func="$4"

    status_full=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | sed '$d')

    status=$(echo "$status_full" | sed "s/HTTP_STATUS://")
    if [ "$status" = "$expected_status" ]; then
        if [ -z "$check_func" ] || echo "$body" | eval "$check_func"; then
            printf "\033[32m✓\033[0m %s\n" "$test_name"
            PASSED=$((PASSED + 1))
            return 0
        fi
    fi

    printf "\033[31m✗\033[0m %s\n" "$test_name"
    echo "  Status: $status"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
    return 1
}

# Create a test source
echo "Creating test source..."
SOURCE_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/test-feed.xml",
        "name": "Test RSS Feed for Get",
        "polling_interval": "1h"
    }')
SOURCE_ID=$(echo "$SOURCE_RESPONSE" | jq -r '.source_id')
echo "Created source: $SOURCE_ID"
echo ""

# Test 1: GET /api/v1/meta/sources/{valid_id}
echo "Test 1: GET /api/v1/meta/sources/{valid_id}"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources/$SOURCE_ID")
test_response "Returns 200 OK with correct source" "$response" "200" \
    "jq -e '.source_id == \"$SOURCE_ID\"' >/dev/null"

# Test 2: Verify source has all required fields
echo "Test 2: Verify source has all required fields"
body=$(curl -s "$BASE_URL/sources/$SOURCE_ID")
if echo "$body" | jq -e '.source_id and .source_type and .url and .name and .enabled_at and .created_at and .updated_at and .polling_interval' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Source has all required fields"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Source missing required fields"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

# Test 3: GET /api/v1/meta/sources/{non_existent_id}
echo "Test 3: GET /api/v1/meta/sources/{non_existent_id}"
NON_EXISTENT_ID="550e8400-e29b-41d4-a716-999999999999"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources/$NON_EXISTENT_ID")
test_response "Returns 404 for non-existent source" "$response" "404"

# Test 4: GET /api/v1/meta/sources/{invalid_uuid}
echo "Test 4: GET /api/v1/meta/sources/{invalid_uuid}"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources/invalid-uuid")
test_response "Returns 400 for invalid UUID format" "$response" "400"

# Test 5: Verify error response format for 404
echo "Test 5: Verify error response format for 404"
body=$(curl -s "$BASE_URL/sources/$NON_EXISTENT_ID")
if echo "$body" | jq -e '.error.code and .error.message' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Error response has correct format (code, message)"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Error response has incorrect format"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "============================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "============================================="

exit $FAILED
