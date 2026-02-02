#!/bin/bash
# Test RFC 6 Section 3.3 - Create Source

BASE_URL="http://localhost:8081/api/v1/meta"
PASSED=0
FAILED=0

echo "Testing RFC 6 Section 3.3 - Create Source"
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

# Test 1: POST /api/v1/meta/sources (RSS source)
echo "Test 1: POST /api/v1/meta/sources (RSS source)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/new-feed.xml",
        "name": "New RSS Feed",
        "polling_interval": "1h"
    }')
test_response "Returns 201 Created with source object" "$response" "201" \
    'jq -e ".source_id and .source_type == \"rss\" and .url == \"https://example.com/new-feed.xml\"" >/dev/null'

# Test 2: POST /api/v1/meta/sources (Atom source)
echo "Test 2: POST /api/v1/meta/sources (Atom source)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "atom",
        "url": "https://example.com/create-test-atom.xml",
        "name": "Atom Feed"
    }')
test_response "Returns 201 Created for Atom source" "$response" "201" \
    'jq -e ".source_type == \"atom\"" >/dev/null'

# Test 3: POST /api/v1/meta/sources (Website source with scraper_config)
echo "Test 3: POST /api/v1/meta/sources (Website source with scraper_config)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "website",
        "url": "https://example.com/create-test-website",
        "name": "Website Scraper",
        "polling_interval": "2h",
        "scraper_config": {
            "discovery_mode": "direct",
            "article_config": {
                "title_selector": "h1.title",
                "content_selector": ".content"
            }
        }
    }')
test_response "Returns 201 Created for website source" "$response" "201" \
    'jq -e ".source_type == \"website\" and .scraper_config" >/dev/null'

# Test 4: Verify created source is enabled by default
echo "Test 4: Verify created source is enabled by default"
body=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/enabled-by-default.xml",
        "name": "Enabled Feed"
    }')
if echo "$body" | jq -e '.enabled_at != null' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Source is enabled by default (enabled_at is not null)"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Source is not enabled by default"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

# Test 5: POST /api/v1/meta/sources (disabled source)
echo "Test 5: POST /api/v1/meta/sources (disabled source with enabled_at: null)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/create-test-disabled.xml",
        "name": "Disabled Feed",
        "enabled_at": null
    }')
test_response "Creates disabled source correctly" "$response" "201" \
    'jq -e ".enabled_at == null" >/dev/null'

# Test 6: POST /api/v1/meta/sources (missing required fields)
echo "Test 6: POST /api/v1/meta/sources (missing required fields)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss"
    }')
test_response "Returns 400 for missing required fields" "$response" "400"

# Test 7: POST /api/v1/meta/sources (invalid source_type)
echo "Test 7: POST /api/v1/meta/sources (invalid source_type)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "invalid",
        "url": "https://example.com/feed.xml",
        "name": "Invalid Type"
    }')
test_response "Returns 400 for invalid source_type" "$response" "400"

# Test 8: POST /api/v1/meta/sources (duplicate URL)
echo "Test 8: POST /api/v1/meta/sources (duplicate URL)"
# Create first source
curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/unique-url.xml",
        "name": "First Feed"
    }' >/dev/null
# Try to create duplicate
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/unique-url.xml",
        "name": "Duplicate Feed"
    }')
test_response "Returns 409 Conflict for duplicate URL" "$response" "409"

# Test 9: Verify created source has all required fields
echo "Test 9: Verify created source has all required fields"
body=$(curl -s -X POST "$BASE_URL/sources" \
    -H "Content-Type: application/json" \
    -d '{
        "source_type": "rss",
        "url": "https://example.com/verify-fields.xml",
        "name": "Verify Fields Feed"
    }')
if echo "$body" | jq -e '.source_id and .source_type and .url and .name and .created_at and .updated_at' >/dev/null 2>&1; then
    printf "\033[32m✓\033[0m %s\n" "Created source has all required fields"
    PASSED=$((PASSED + 1))
else
    printf "\033[31m✗\033[0m %s\n" "Created source missing required fields"
    echo "  Body: $body"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

exit $FAILED
