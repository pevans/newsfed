#!/bin/bash
# Common test helpers for bats-core tests
# This file is loaded by bats tests using: load test_helper

# Load bats-support and bats-assert if available
if [ -f "/opt/homebrew/lib/bats-support/load.bash" ]; then
    load "/opt/homebrew/lib/bats-support/load.bash"
fi
if [ -f "/opt/homebrew/lib/bats-assert/load.bash" ]; then
    load "/opt/homebrew/lib/bats-assert/load.bash"
fi

# Setup test environment variables
setup_test_env() {
    # Use /tmp/claude/ which is allowed by sandbox, or BATS_TEST_TMPDIR if set
    if [ -n "$BATS_TEST_TMPDIR" ]; then
        export TEST_DIR="$BATS_TEST_TMPDIR"
    else
        export TEST_DIR="/tmp/claude/bats-test-$$-$RANDOM"
        mkdir -p "$TEST_DIR"
    fi
    export NEWSFED_METADATA_DSN="$TEST_DIR/metadata.db"
    export NEWSFED_FEED_DSN="$TEST_DIR/.news"
    export PATH="$TEST_DIR:$PATH"
}

# Build newsfed CLI binary
build_newsfed() {
    local test_dir="${1:-$TEST_DIR}"
    echo "Building newsfed CLI..." >&2
    (cd "${BATS_TEST_DIRNAME}/.." && \
        go build -o "$test_dir/newsfed" ./cmd/newsfed 2>&1 | \
        grep -v "operation not permitted" || true) >&2

    if [ ! -f "$test_dir/newsfed" ]; then
        echo "Error: Failed to build newsfed CLI" >&2
        return 1
    fi
}

# Clean up test environment
cleanup_test_env() {
    if [ -n "$TEST_DIR" ] && [ -d "$TEST_DIR" ]; then
        rm -rf "$TEST_DIR"
    fi
}

# Assert output contains a string
assert_output_contains() {
    local expected="$1"
    if ! echo "$output" | grep -q "$expected"; then
        echo "Expected output to contain: $expected"
        echo "Got: $output"
        return 1
    fi
}

# Assert output does not contain a string
assert_output_not_contains() {
    local unexpected="$1"
    if echo "$output" | grep -q "$unexpected"; then
        echo "Expected output NOT to contain: $unexpected"
        echo "Got: $output"
        return 1
    fi
}

# Extract UUID from output
extract_uuid() {
    local text="$1"
    echo "$text" | grep -o '[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}' | head -1
}

# Create timestamp helpers (macOS and Linux compatible)
timestamp_days_ago() {
    local days="$1"
    date -u -v-${days}d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
        date -u --date="${days} days ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
        date -u +%Y-%m-%dT%H:%M:%SZ
}

timestamp_hours_ago() {
    local hours="$1"
    date -u -v-${hours}H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
        date -u --date="${hours} hours ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
        date -u +%Y-%m-%dT%H:%M:%SZ
}

# Create a news item JSON file
create_news_item() {
    local id="$1"
    local title="$2"
    local publisher="${3:-}"
    local published_at="${4:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
    local pinned_at="${5:-}"

    local json_file="$NEWSFED_FEED_DSN/${id}.json"

    cat > "$json_file" <<EOF
{
  "id": "$id",
  "title": "$title",
  "summary": "Summary for $title",
  "url": "https://example.com/${id}",
  "publisher": "$publisher",
  "authors": [],
  "published_at": "$published_at",
  "discovered_at": "$published_at"$([ -n "$pinned_at" ] && echo ",
  \"pinned_at\": \"$pinned_at\"" || echo "")
}
EOF
}

# Wait for file to exist (with timeout)
wait_for_file() {
    local file="$1"
    local timeout="${2:-5}"
    local elapsed=0

    while [ ! -f "$file" ] && [ $elapsed -lt $timeout ]; do
        sleep 0.1
        elapsed=$((elapsed + 1))
    done

    [ -f "$file" ]
}

# Execute SQLite command on metadata database
exec_sqlite() {
    local sql="$1"
    sqlite3 "$NEWSFED_METADATA_DSN" "$sql" 2>/dev/null || true
}

# Get default browser command for current platform
get_default_browser() {
    case "$(uname -s)" in
        Darwin)
            echo "open"
            ;;
        Linux)
            echo "xdg-open"
            ;;
        CYGWIN*|MINGW*|MSYS*)
            echo "cmd /c start"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# Create temporary directory for permission tests
create_permission_test_dir() {
    local temp_dir=$(mktemp -d)
    echo "$temp_dir"
}

# Restore permissions on directory (for cleanup after permission tests)
restore_permissions() {
    local dir="$1"
    if [ -d "$dir" ]; then
        chmod -R 755 "$dir" 2>/dev/null || true
    fi
}
