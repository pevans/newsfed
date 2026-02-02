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
    body=$(echo "$response" | head -n -1)
    status_line=$(echo "$response" | tail -n 1)
    status=$(echo "$status_line" | sed 's/HTTP_STATUS://')

    if [ "$status" = "$expected_status" ]; then
        if [ -z "$check_func" ] || eval "$check_func" <<< "$body"; then
            echo -e "\e[32m✓\e[0m $test_name"
            return 0
        fi
    fi

    echo -e "\e[31m✗\e[0m $test_name"
    echo "  Expected status: $expected_status, Got: $status"
    echo "  Body: $body"
    return 1
}
