#!/bin/bash
# Run all CLI item command tests

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Setup test environment silently (run as script, not sourced)
bash "$SCRIPT_DIR/setup.sh" > /dev/null 2>&1

# Export environment variables for child processes
export NEWSFED_FEED_DSN="$TEST_DIR/.news"
export PATH="$TEST_DIR:$PATH"

# Track overall results
TOTAL_FAILED=0
FAILED_OUTPUT=""

# Helper function to run a single test
run_test() {
    local test_file="$1"
    local test_name=$(basename "$test_file" .sh)
    local tmpfile="$SCRIPT_DIR/.test-data/test-output-$test_name.txt"

    bash "$test_file" > "$tmpfile" 2>&1
    local exit_code=$?

    if [ $exit_code -ne 0 ]; then
        FAILED_OUTPUT="${FAILED_OUTPUT}\n=== $test_name ===\n"
        FAILED_OUTPUT="${FAILED_OUTPUT}$(cat "$tmpfile")\n"
        TOTAL_FAILED=$((TOTAL_FAILED + 1))
    fi

    rm -f "$tmpfile"
}

# Run tests in logical order
run_test "$SCRIPT_DIR/test-show.sh"
run_test "$SCRIPT_DIR/test-pin-unpin.sh"
run_test "$SCRIPT_DIR/test-open.sh"

# Print single summary line
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "cli-items: OK"
else
    echo "cli-items: FAIL"
    printf "%b" "$FAILED_OUTPUT"
fi

exit $TOTAL_FAILED
