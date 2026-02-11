#!/bin/bash
# Run all bats-core tests for newsfed
# This script discovers and runs all .bats files in the tests.v3 directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Check if bats is installed
if ! command -v bats &> /dev/null; then
    echo -e "${RED}Error: bats-core is not installed${NC}"
    echo ""
    echo "Please install bats-core:"
    echo "  macOS:  brew install bats-core"
    echo "  Linux:  apt-get install bats or see https://github.com/bats-core/bats-core"
    exit 1
fi

# Optional: Check for bats-support and bats-assert
if [ ! -f "/opt/homebrew/lib/bats-support/load.bash" ]; then
    echo "Warning: bats-support not found, some assertions may not work"
    echo "  Install with: brew install bats-support"
fi

if [ ! -f "/opt/homebrew/lib/bats-assert/load.bash" ]; then
    echo "Warning: bats-assert not found, some assertions may not work"
    echo "  Install with: brew install bats-assert"
fi

# Find all .bats files
BATS_FILES=$(find "$SCRIPT_DIR" -name "*.bats" -type f | sort)

if [ -z "$BATS_FILES" ]; then
    echo -e "${RED}No .bats files found in $SCRIPT_DIR${NC}"
    exit 1
fi

echo "Running newsfed bats-core tests..."
echo ""

# Run bats with all test files
# Options:
#   -r: recursive
#   --formatter: pretty (default), tap, or junit
#   --jobs: number of parallel jobs (default: 1)
if [ "$1" = "--parallel" ]; then
    # Run tests in parallel for speed
    bats --jobs 4 $BATS_FILES
elif [ "$1" = "--tap" ]; then
    # Output in TAP format for CI systems
    bats --formatter tap $BATS_FILES
elif [ "$1" = "--junit" ]; then
    # Output in JUnit XML format for CI systems
    bats --formatter junit $BATS_FILES
else
    # Default: pretty format
    bats $BATS_FILES
fi

exit_code=$?

if [ $exit_code -eq 0 ]; then
    echo ""
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo ""
    echo -e "${RED}Some tests failed${NC}"
fi

exit $exit_code
