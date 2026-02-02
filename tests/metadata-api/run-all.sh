#!/bin/bash
# Run metadata API black box tests

BASE_URL="http://localhost:8081/api/v1/meta"

echo "RFC 6 Metadata API Black Box Tests"
echo "==================================="
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Setup test data
echo "Setting up test data..."
"$SCRIPT_DIR/setup.sh"
echo ""

# Run all tests
TOTAL_FAILED=0

echo "Running test suites..."
echo ""

# Test List Sources (Section 3.1)
if "$SCRIPT_DIR/test-list-sources.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Get Source (Section 3.2)
if "$SCRIPT_DIR/test-get-source.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Create Source (Section 3.3)
if "$SCRIPT_DIR/test-create-source.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Update Source (Section 3.4)
if "$SCRIPT_DIR/test-update-source.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Delete Source (Section 3.5)
if "$SCRIPT_DIR/test-delete-source.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test Configuration (Sections 3.6 and 3.7)
if "$SCRIPT_DIR/test-config.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Test CORS Support (Section 6)
if "$SCRIPT_DIR/test-cors.sh"; then
    echo ""
else
    TOTAL_FAILED=$((TOTAL_FAILED + $?))
fi

# Final summary
echo ""
echo "==================================="
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "All metadata API tests passed! ✓"
    exit 0
else
    echo "Some metadata API tests failed ($TOTAL_FAILED total failures)"
    exit 1
fi
