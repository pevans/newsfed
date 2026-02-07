#!/bin/bash
# Setup script for CLI source management tests
# Prepares isolated test environment with clean databases

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

echo "Setting up CLI test environment..."
echo ""

# Clean up any existing test data
if [ -d "$TEST_DIR" ]; then
    echo "Cleaning up existing test data..."
    rm -rf "$TEST_DIR"
fi

# Create fresh test directory
mkdir -p "$TEST_DIR"

# Build the CLI if not already built or if source has changed
echo "Building newsfed CLI..."
cd "$SCRIPT_DIR/../.."
go build -o "$TEST_DIR/newsfed" ./cmd/newsfed 2>&1 | grep -v "operation not permitted" || true

# Verify the binary was created
if [ ! -f "$TEST_DIR/newsfed" ]; then
    echo "Error: Failed to build newsfed CLI"
    exit 1
fi

# Export test environment variables
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

echo "✓ Test environment ready"
echo "  CLI binary: $TEST_DIR/newsfed"
echo "  Metadata DB: $NEWSFED_METADATA_DSN"
echo "  Feed storage: $NEWSFED_FEED_DSN"
echo ""
