#!/bin/bash
# Test RFC 6 Section 3.1 - List Sources

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 3.1 - List Sources"
echo "========================================="
echo ""

# Helper function to test HTTP responses
test_response() {
    local test_name="$1"
    local response="$2"
    local expected_status="$3"
    local check_func="$4"

    status_full=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | head -n -1)

    status=$(echo "$status_full" | sed "s/HTTP_STATUS://")
    if [ "$status" = "$expected_status" ]; then
        if [ -z "$check_func" ] || $check_func "$body"; then
            echo -e "\e[32m✓\e[0m $test_name"
            PASSED=$((PASSED + 1))
            return 0
        fi
    fi

    echo -e "\e[31m✗\e[0m $test_name"
    echo "  Status: $status"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
    return 1
}

# Create test sources
echo "Setting up test sources..."

# Create RSS source
RSS_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/rss-feed.xml",
        "name": "Test RSS Feed",
        "polling_interval": "1h"
    }')
RSS_ID=$(echo "$RSS_RESPONSE" | jq -r '.source_id')

# Create Atom source
ATOM_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "atom",
        "url": "https://example.com/atom-feed.xml",
        "name": "Test Atom Feed",
        "polling_interval": "2h"
    }')
ATOM_ID=$(echo "$ATOM_RESPONSE" | jq -r '.source_id')

# Create website source (enabled)
WEBSITE_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "website",
        "url": "https://example.com/articles",
        "name": "Test Website",
        "polling_interval": "30m",
        "scraper_config": {
            "discovery_mode": "list",
            "list_config": {
                "article_selector": ".article-link",
                "max_pages": 3
            },
            "article_config": {
                "title_selector": "h1.title",
                "content_selector": ".content"
            }
        }
    }')
WEBSITE_ID=$(echo "$WEBSITE_RESPONSE" | jq -r '.source_id')

# Create disabled source
DISABLED_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/disabled-feed.xml",
        "name": "Disabled Feed",
        "enabled_at": null
    }')
DISABLED_ID=$(echo "$DISABLED_RESPONSE" | jq -r '.source_id')

echo "Created test sources: RSS=$RSS_ID, ATOM=$ATOM_ID, WEBSITE=$WEBSITE_ID, DISABLED=$DISABLED_ID"
echo ""

# Test 1: GET /api/v1/meta/sources (no parameters)
echo "Test 1: GET /api/v1/meta/sources (no parameters)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources")
test_response "Returns 200 OK with sources array" "$response" "200" \
    'jq -e ".sources | length == 4" >/dev/null'

# Test 2: Verify response structure
echo "Test 2: Verify response structure"
body=$(curl -s "$BASE_URL/sources")
if echo "$body" | jq -e '.sources and .total' >/dev/null 2>&1; then
    echo -e "\e[32m✓\e[0m Response has required fields (sources, total)"
    PASSED=$((PASSED + 1))
else
    echo -e "\e[31m✗\e[0m Response missing required fields"
    FAILED=$((FAILED + 1))
fi

# Test 3: GET /api/v1/meta/sources?type=rss
echo "Test 3: GET /api/v1/meta/sources?type=rss"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources?type=rss")
test_response "Filters RSS sources correctly" "$response" "200" \
    'jq -e ".sources | length == 2 and all(.source_type == \"rss\")" >/dev/null'

# Test 4: GET /api/v1/meta/sources?type=atom
echo "Test 4: GET /api/v1/meta/sources?type=atom"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources?type=atom")
test_response "Filters Atom sources correctly" "$response" "200" \
    'jq -e ".sources | length == 1 and all(.source_type == \"atom\")" >/dev/null'

# Test 5: GET /api/v1/meta/sources?type=website
echo "Test 5: GET /api/v1/meta/sources?type=website"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources?type=website")
test_response "Filters website sources correctly" "$response" "200" \
    'jq -e ".sources | length == 1 and all(.source_type == \"website\")" >/dev/null'

# Test 6: GET /api/v1/meta/sources?enabled=true
echo "Test 6: GET /api/v1/meta/sources?enabled=true"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources?enabled=true")
test_response "Filters enabled sources correctly" "$response" "200" \
    'jq -e ".sources | length == 3 and all(.enabled_at != null)" >/dev/null'

# Test 7: GET /api/v1/meta/sources?enabled=false
echo "Test 7: GET /api/v1/meta/sources?enabled=false"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/sources?enabled=false")
test_response "Filters disabled sources correctly" "$response" "200" \
    'jq -e ".sources | length == 1 and all(.enabled_at == null)" >/dev/null'

# Test 8: Verify source has all required fields
echo "Test 8: Verify source has all required fields"
body=$(curl -s "$BASE_URL/sources")
if echo "$body" | jq -e '.sources[0] | .source_id and .source_type and .url and .name and .created_at and .updated_at' >/dev/null 2>&1; then
    echo -e "\e[32m✓\e[0m Source has all required fields"
    PASSED=$((PASSED + 1))
else
    echo -e "\e[31m✗\e[0m Source missing required fields"
    FAILED=$((FAILED + 1))
fi

# Test 9: Verify total count is correct
echo "Test 9: Verify total count matches array length"
body=$(curl -s "$BASE_URL/sources")
sources_length=$(echo "$body" | jq '.sources | length')
total_value=$(echo "$body" | jq '.total')
if [ "$sources_length" -eq "$total_value" ]; then
    echo -e "\e[32m✓\e[0m Total count matches array length"
    PASSED=$((PASSED + 1))
else
    echo -e "\e[31m✗\e[0m Total count ($total_value) doesn't match array length ($sources_length)"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
