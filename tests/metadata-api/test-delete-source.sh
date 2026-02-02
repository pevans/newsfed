#!/bin/bash
# Test RFC 6 Section 3.5 - Delete Source

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 3.5 - Delete Source"
echo "=========================================="
echo ""

# Helper function to test HTTP responses
test_response() {
    local test_name="$1"
    local response="$2"
    local expected_status="$3"

    status_full=$(echo "$response" | tail -n 1)

    status=$(echo "$status_full" | sed "s/HTTP_STATUS://")
    if [ "$status" = "$expected_status" ]; then
        printf "\033[32m✓\033[0m %s\n" "$test_name"
        PASSED=$((PASSED + 1))
        return 0
    fi

    printf "\033[31m✗\033[0m %s\n" "$test_name"
    echo "  Status: $status"
    FAILED=$((FAILED + 1))
    return 1
}

# Create test sources
echo "Creating test sources..."
SOURCE1=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/delete-test1.xml",
        "name": "Delete Test 1"
    }')
SOURCE1_ID=$(echo "$SOURCE1" | jq -r '.source_id')

SOURCE2=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/delete-test2.xml",
        "name": "Delete Test 2"
    }')
SOURCE2_ID=$(echo "$SOURCE2" | jq -r '.source_id')

echo "Created sources: $SOURCE1_ID, $SOURCE2_ID"
echo ""

# Test 1: DELETE /api/v1/meta/sources/{id}
echo "Test 1: DELETE /api/v1/meta/sources/{id}"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$BASE_URL/sources/$SOURCE1_ID")
test_response "Returns 204 No Content" "$response" "204"

# Test 2: Verify source was deleted (GET after DELETE should return 404)
echo "Test 2: Verify source was deleted (GET after DELETE)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources/$SOURCE1_ID")
test_response "GET returns 404 for deleted source" "$response" "404"

# Test 3: Verify deleted source not in list
echo "Test 3: Verify deleted source not in list"
body=$(curl -s "$BASE_URL/sources")
if echo "$body" | jq -e ".sources | any(.source_id == \"$SOURCE1_ID\")" >/dev/null 2>&1; then
    printf "\033[31m✗\033[0m %s\n" "Deleted source still appears in list"
    FAILED=$((FAILED + 1))
else
    printf "\033[32m✓\033[0m %s\n" "Deleted source not in list"
    PASSED=$((PASSED + 1))
fi

# Test 4: DELETE /api/v1/meta/sources/{non_existent_id}
echo "Test 4: DELETE /api/v1/meta/sources/{non_existent_id}"
NON_EXISTENT_ID="550e8400-e29b-41d4-a716-999999999999"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$BASE_URL/sources/$NON_EXISTENT_ID")
test_response "Returns 404 for non-existent source" "$response" "404"

# Test 5: DELETE is idempotent (deleting already deleted source)
echo "Test 5: DELETE is idempotent (deleting already deleted source)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$BASE_URL/sources/$SOURCE1_ID")
test_response "Returns 404 for already deleted source" "$response" "404"

# Test 6: DELETE /api/v1/meta/sources/{invalid_uuid}
echo "Test 6: DELETE /api/v1/meta/sources/{invalid_uuid}"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$BASE_URL/sources/invalid-uuid")
test_response "Returns 400 for invalid UUID" "$response" "400"

# Test 7: Verify other sources still exist after deletion
echo "Test 7: Verify other sources still exist after deletion"
body=$(curl -s "$BASE_URL/sources")
if echo "$body" | jq -e ".sources | any(.source_id == \"$SOURCE2_ID\")" >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Other sources still exist"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Other sources were affected by deletion"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
