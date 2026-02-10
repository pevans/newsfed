#!/bin/bash
# Run all CLI error handling tests

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/.test-data"

# Setup test environment silently (run as script, not sourced)
bash "$SCRIPT_DIR/setup.sh" > /dev/null 2>&1

# Export environment variables for child processes
export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
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

# Run all test files
for test_file in "$SCRIPT_DIR"/test-*.sh; do
    if [ -f "$test_file" ] && [ "$test_file" != "$SCRIPT_DIR/test-helpers.sh" ]; then
        run_test "$test_file"
    fi
done

# Print single summary line
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "cli-errors: OK"
else
    echo "cli-errors: FAIL"
    printf "%b" "$FAILED_OUTPUT"
fi

exit $TOTAL_FAILED
