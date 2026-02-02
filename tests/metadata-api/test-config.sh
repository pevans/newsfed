#!/bin/bash
# Test RFC 6 Sections 3.6 and 3.7 - Configuration Management

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Sections 3.6 & 3.7 - Configuration"
echo "================================================="
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

# Test 1: GET /api/v1/meta/config
echo "Test 1: GET /api/v1/meta/config"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/config")
test_response "Returns 200 OK with config object" "$response" "200"

# Test 2: Verify config has default_polling_interval field
echo "Test 2: Verify config has default_polling_interval field"
body=$(curl -s "$BASE_URL/config")
if echo "$body" | jq -e '.default_polling_interval' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Config has default_polling_interval field"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Config missing default_polling_interval field"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

# Test 3: PUT /api/v1/meta/config (update default_polling_interval)
echo "Test 3: PUT /api/v1/meta/config (update default_polling_interval)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/config" \
    -H "Content-Type: application/json" \
    -d '{
        "default_polling_interval": "2h"
    }')
test_response "Returns 200 OK with updated config" "$response" "200" \
    'jq -e ".default_polling_interval == \"2h\"" >/dev/null'

# Test 4: Verify config update persisted (GET after PUT)
echo "Test 4: Verify config update persisted (GET after PUT)"
body=$(curl -s "$BASE_URL/config")
if echo "$body" | jq -e '.default_polling_interval == "2h"' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Config update persisted"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Config update did not persist"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

# Test 5: PUT /api/v1/meta/config (update to different value)
echo "Test 5: PUT /api/v1/meta/config (update to different value)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/config" \
    -H "Content-Type: application/json" \
    -d '{
        "default_polling_interval": "30m"
    }')
test_response "Updates to new value successfully" "$response" "200" \
    'jq -e ".default_polling_interval == \"30m\"" >/dev/null'

# Test 6: PUT /api/v1/meta/config (invalid value)
echo "Test 6: PUT /api/v1/meta/config (invalid polling interval)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/config" \
    -H "Content-Type: application/json" \
    -d '{
        "default_polling_interval": "invalid"
    }')
test_response "Returns 400 for invalid value" "$response" "400"

# Test 7: PUT /api/v1/meta/config (empty body)
echo "Test 7: PUT /api/v1/meta/config (empty body)"
original_value=$(curl -s "$BASE_URL/config" | jq -r '.default_polling_interval')
curl -s -X PUT "$BASE_URL/config" \
    -H "Content-Type: application/json" \
    -d '{}' >/dev/null
new_value=$(curl -s "$BASE_URL/config" | jq -r '.default_polling_interval')

if [ "$original_value" == "$new_value" ]; then
    printf "\033[32m✓\033[0m %s\n" "Empty body does not change config"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Empty body changed config unexpectedly"
    echo "  Original: $original_value"
    echo "  New: $new_value"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "================================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "================================================="

exit $FAILED
