#!/bin/bash
# Test RFC 6 Section 3.4 - Update Source

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 3.4 - Update Source"
echo "=========================================="
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

# Create test sources
echo "Creating test sources..."
SOURCE1=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/feed1.xml",
        "name": "Original Name",
        "polling_interval": "1h"
    }')
SOURCE1_ID=$(echo "$SOURCE1" | jq -r '.source_id')

SOURCE2=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "website",
        "url": "https://example.com/update-test-website",
        "name": "Original Website",
        "scraper_config": {
            "discovery_mode": "direct",
            "article_config": {
                "title_selector": "h1",
                "content_selector": ".content"
            }
        }
    }')
SOURCE2_ID=$(echo "$SOURCE2" | jq -r '.source_id')

echo "Created sources: $SOURCE1_ID, $SOURCE2_ID"
echo ""

# Test 1: PUT /api/v1/meta/sources/{id} (update name)
echo "Test 1: PUT /api/v1/meta/sources/{id} (update name)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Updated Name"
    }')
test_response "Returns 200 OK with updated source" "$response" "200" \
    'jq -e ".name == \"Updated Name\"" >/dev/null'

# Test 2: Verify update persisted (GET after PUT)
echo "Test 2: Verify update persisted (GET after PUT)"
body=$(curl -s "$BASE_URL/sources/$SOURCE1_ID")
if echo "$body" | jq -e '.name == "Updated Name"' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Update persisted correctly"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Update did not persist"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

# Test 3: PUT /api/v1/meta/sources/{id} (update URL)
echo "Test 3: PUT /api/v1/meta/sources/{id} (update URL)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "url": "https://example.com/new-feed-url.xml"
    }')
test_response "Updates URL successfully" "$response" "200" \
    'jq -e ".url == \"https://example.com/new-feed-url.xml\"" >/dev/null'

# Test 4: PUT /api/v1/meta/sources/{id} (update polling_interval)
echo "Test 4: PUT /api/v1/meta/sources/{id} (update polling_interval)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "polling_interval": "30m"
    }')
test_response "Updates polling_interval successfully" "$response" "200" \
    'jq -e ".polling_interval == \"30m\"" >/dev/null'

# Test 5: PUT /api/v1/meta/sources/{id} (disable source)
echo "Test 5: PUT /api/v1/meta/sources/{id} (disable source by setting enabled_at to null)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "enabled_at": null
    }')
test_response "Disables source successfully" "$response" "200" \
    'jq -e ".enabled_at == null" >/dev/null'

# Test 6: PUT /api/v1/meta/sources/{id} (enable source)
echo "Test 6: PUT /api/v1/meta/sources/{id} (enable source by setting enabled_at)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d "{
        \"enabled_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
    }")
test_response "Enables source successfully" "$response" "200" \
    'jq -e ".enabled_at != null" >/dev/null'

# Test 7: PUT /api/v1/meta/sources/{id} (update scraper_config for website)
echo "Test 7: PUT /api/v1/meta/sources/{id} (update scraper_config)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE2_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "scraper_config": {
            "discovery_mode": "list",
            "list_config": {
                "article_selector": ".article",
                "max_pages": 5
            },
            "article_config": {
                "title_selector": "h1.new-title",
                "content_selector": ".new-content"
            }
        }
    }')
test_response "Updates scraper_config successfully" "$response" "200" \
    'jq -e ".scraper_config.discovery_mode == \"list\"" >/dev/null'

# Test 8: PUT /api/v1/meta/sources/{non_existent_id}
echo "Test 8: PUT /api/v1/meta/sources/{non_existent_id}"
NON_EXISTENT_ID="550e8400-e29b-41d4-a716-999999999999"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$NON_EXISTENT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Updated Name"
    }')
test_response "Returns 404 for non-existent source" "$response" "404"

# Test 9: PUT /api/v1/meta/sources/{id} (invalid data)
echo "Test 9: PUT /api/v1/meta/sources/{id} (invalid polling_interval)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "polling_interval": "invalid"
    }')
test_response "Returns 400 for invalid data" "$response" "400"

# Test 10: Verify updated_at changes after update
echo "Test 10: Verify updated_at changes after update"
original_updated_at=$(curl -s "$BASE_URL/sources/$SOURCE1_ID" | jq -r '.updated_at')
sleep 1
curl -s -X PUT "$BASE_URL/sources/$SOURCE1_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Another Update"
    }' >/dev/null
new_updated_at=$(curl -s "$BASE_URL/sources/$SOURCE1_ID" | jq -r '.updated_at')

if [ "$original_updated_at" != "$new_updated_at" ]; then
    printf "\033[32m✓\033[0m %s\n" "updated_at timestamp changes after update"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "updated_at timestamp did not change"
    echo "  Original: $original_updated_at"
    echo "  New: $new_updated_at"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
