#!/bin/bash
# Common test helper functions for metadata API tests

# Helper function to test HTTP responses
test_response() {
    local test_name="$1"
    local response="$2"
    local expected_status="$3"
    local check_func="$4"

    # Extract body and status from response
    # Response format: <json_body>\nHTTP_STATUS:<code>
    body=$(echo "$response" | sed '$d')
    status_line=$(echo "$response" | tail -n 1)
    status=$(echo "$status_line" | sed 's/HTTP_STATUS://')

    if [ "$status" = "$expected_status" ]; then
        if [ -z "$check_func" ] || eval "$check_func" <<< "$body"; then
            printf "\033[32m✓\033[0m %s\n" "$test_name"
            return 0
        fi
    fi

    printf "\033[31m✗\033[0m %s\n" "$test_name"
    echo "  Expected status: $expected_status, Got: $status"
    echo "  Body: $body"
    return 1
}
