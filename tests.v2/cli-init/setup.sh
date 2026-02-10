#!/bin/bash
# Setup script for CLI init/doctor tests
# Prepares test environment for initialization tests
# Runs silently -- only prints errors

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Clean up any existing test data
if [ -d "$TEST_DIR" ]; then
    rm -rf "$TEST_DIR"
fi

# Create fresh test directory
mkdir -p "$TEST_DIR"

# Build the CLI if not already built or if source has changed
(cd "$SCRIPT_DIR/../.." && go build -o "$TEST_DIR/newsfed" ./cmd/newsfed 2>&1 | grep -v "operation not permitted" || true) > /dev/null 2>&1

# Verify the binary was created
if [ ! -f "$TEST_DIR/newsfed" ]; then
    echo "Error: Failed to build newsfed CLI" >&2
    exit 1
fi

# Export test environment variables
export PATH="$TEST_DIR:$PATH"
