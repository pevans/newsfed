#!/bin/bash
# Setup script for CLI configuration tests
# Prepares isolated test environment for testing configuration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

echo "Setting up CLI configuration test environment..."
echo ""

# Clean up any existing test data
if [ -d "$TEST_DIR" ]; then
    echo "Cleaning up existing test data..."
    rm -rf "$TEST_DIR"
fi

# Create fresh test directories
mkdir -p "$TEST_DIR/config1"
mkdir -p "$TEST_DIR/config2"

# Build the CLI if not already built or if source has changed
echo "Building newsfed CLI..."
cd "$SCRIPT_DIR/../.."
go build -o "$TEST_DIR/newsfed" ./cmd/newsfed 2>&1 | grep -v "operation not permitted" || true

# Verify the binary was created
if [ ! -f "$TEST_DIR/newsfed" ]; then
    echo "Error: Failed to build newsfed CLI"
    exit 1
fi

# Export PATH to include test binary
export PATH="$TEST_DIR:$PATH"

echo "✓ Test environment ready"
echo "  CLI binary: $TEST_DIR/newsfed"
echo "  Test directory: $TEST_DIR"
echo ""
