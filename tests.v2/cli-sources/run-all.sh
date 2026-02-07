#!/bin/bash
# Run all CLI source management tests

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Setup test environment silently
source "$SCRIPT_DIR/setup.sh" > /dev/null 2>&1

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
run_test "$SCRIPT_DIR/test-add.sh"
run_test "$SCRIPT_DIR/test-list.sh"
run_test "$SCRIPT_DIR/test-show.sh"
run_test "$SCRIPT_DIR/test-update.sh"
run_test "$SCRIPT_DIR/test-enable-disable.sh"
run_test "$SCRIPT_DIR/test-delete.sh"

# Print single summary line
if [ $TOTAL_FAILED -eq 0 ]; then
    echo "cli-sources: OK"
else
    echo "cli-sources: FAIL"
    printf "%b" "$FAILED_OUTPUT"
fi

exit $TOTAL_FAILED
