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

# Update the last_fetched_at timestamp for a source
# Usage: update_last_fetched_at SOURCE_ID DAYS_AGO
update_last_fetched_at() {
    local source_id="$1"
    local days_ago="$2"
    local timestamp
    timestamp=$(timestamp_days_ago "$days_ago")
    exec_sqlite "UPDATE sources SET last_fetched_at = '$timestamp' WHERE source_id = '$source_id'"
}

# Get the last_fetched_at timestamp for a source
# Usage: timestamp=$(get_last_fetched_at SOURCE_ID)
get_last_fetched_at() {
    local source_id="$1"
    exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'"
}

# Measure the time taken for a sync operation in milliseconds
# Usage: duration=$(measure_sync_duration SOURCE_ID)
measure_sync_duration() {
    local source_id="$1"
    local start_sec start_nano end_sec end_nano
    local start_ms end_ms

    if command -v gdate >/dev/null 2>&1; then
        # macOS with GNU coreutils
        start_sec=$(gdate +%s)
        start_nano=$(gdate +%N)
        newsfed sync "$source_id" >/dev/null 2>&1
        end_sec=$(gdate +%s)
        end_nano=$(gdate +%N)

        # Calculate milliseconds
        start_ms=$((start_sec * 1000 + start_nano / 1000000))
        end_ms=$((end_sec * 1000 + end_nano / 1000000))
        echo $((end_ms - start_ms))
    else
        # Fallback: use seconds only
        start_sec=$(date +%s)
        newsfed sync "$source_id" >/dev/null 2>&1
        end_sec=$(date +%s)
        echo $(((end_sec - start_sec) * 1000))
    fi
}

# Start mock server that logs headers to a file
# Usage: start_logging_mock_server WWW_DIR LOG_FILE
start_logging_mock_server() {
    local www_dir="$1"
    local log_file="$2"
    local port_file="$TEST_DIR/logging_mock_server_port"
    rm -f "$port_file" "$log_file"

    python3 -c "
import http.server
import socketserver
import sys
import os

os.chdir('$www_dir')

class LoggingHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        # Log User-Agent header
        user_agent = self.headers.get('User-Agent', '')
        with open('$log_file', 'a') as f:
            f.write(f'{self.path}|User-Agent: {user_agent}\n')
        return super().do_GET()

    def log_message(self, format, *args):
        pass  # Suppress request logs

with socketserver.TCPServer(('127.0.0.1', 0), LoggingHandler) as httpd:
    port = httpd.server_address[1]
    with open('$port_file', 'w') as f:
        f.write(str(port))
    httpd.serve_forever()
" >/dev/null 2>&1 &

    LOGGING_MOCK_SERVER_PID=$!
    wait_for_file "$port_file" 10
    LOGGING_MOCK_SERVER_PORT=$(cat "$port_file")
}

# Stop logging mock server
stop_logging_mock_server() {
    if [ -n "${LOGGING_MOCK_SERVER_PID:-}" ]; then
        kill "$LOGGING_MOCK_SERVER_PID" 2>/dev/null || true
        wait "$LOGGING_MOCK_SERVER_PID" 2>/dev/null || true
        unset LOGGING_MOCK_SERVER_PID
    fi
    rm -f "$TEST_DIR/logging_mock_server_port"
}

# Start mock server that responds with HTTP redirect
# Usage: start_redirect_mock_server TARGET_URL
start_redirect_mock_server() {
    local target_url="$1"
    local port_file="$TEST_DIR/redirect_mock_server_port"
    rm -f "$port_file"

    python3 -c "
import http.server
import socketserver

class RedirectHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(302)
        self.send_header('Location', '$target_url')
        self.end_headers()

    def log_message(self, format, *args):
        pass  # Suppress logs

with socketserver.TCPServer(('127.0.0.1', 0), RedirectHandler) as httpd:
    port = httpd.server_address[1]
    with open('$port_file', 'w') as f:
        f.write(str(port))
    httpd.serve_forever()
" >/dev/null 2>&1 &

    REDIRECT_MOCK_SERVER_PID=$!
    wait_for_file "$port_file" 10
    REDIRECT_MOCK_SERVER_PORT=$(cat "$port_file")
}

# Stop redirect mock server
stop_redirect_mock_server() {
    if [ -n "${REDIRECT_MOCK_SERVER_PID:-}" ]; then
        kill "$REDIRECT_MOCK_SERVER_PID" 2>/dev/null || true
        wait "$REDIRECT_MOCK_SERVER_PID" 2>/dev/null || true
        unset REDIRECT_MOCK_SERVER_PID
    fi
    rm -f "$TEST_DIR/redirect_mock_server_port"
}

# Start a mock HTTP server in the background serving files from a directory.
# Sets MOCK_SERVER_PID and MOCK_SERVER_PORT variables.
# Usage: start_mock_server "$TEST_DIR/www"
start_mock_server() {
    local serve_dir="$1"
    local port_file="$TEST_DIR/mock_server_port"
    rm -f "$port_file"

    python3 -c "
import http.server, socketserver, os, sys
os.chdir(sys.argv[1])
handler = http.server.SimpleHTTPRequestHandler
httpd = socketserver.TCPServer(('127.0.0.1', 0), handler)
port = httpd.server_address[1]
with open(sys.argv[2], 'w') as f:
    f.write(str(port))
httpd.serve_forever()
" "$serve_dir" "$port_file" 2>/dev/null &
    MOCK_SERVER_PID=$!

    wait_for_file "$port_file" 10
    MOCK_SERVER_PORT=$(cat "$port_file")
}

# Stop the mock HTTP server.
stop_mock_server() {
    if [ -n "${MOCK_SERVER_PID:-}" ]; then
        kill "$MOCK_SERVER_PID" 2>/dev/null || true
        wait "$MOCK_SERVER_PID" 2>/dev/null || true
        unset MOCK_SERVER_PID
    fi
    rm -f "$TEST_DIR/mock_server_port"
}

# Create a minimal valid RSS feed XML file.
# Usage: create_rss_feed "$path/feed.xml" "Feed Title" 2
create_rss_feed() {
    local file="$1"
    local title="${2:-Test Feed}"
    local item_count="${3:-2}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<RSSEOF
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>$title</title>
    <link>http://example.com</link>
    <description>A test RSS feed</description>
RSSEOF

    for i in $(seq 1 "$item_count"); do
        cat >> "$file" <<ITEMEOF
    <item>
      <title>Article $i</title>
      <link>http://example.com/article$i</link>
      <description>Test article $i</description>
    </item>
ITEMEOF
    done

    cat >> "$file" <<RSSEOF
  </channel>
</rss>
RSSEOF
}

# Create an HTML article page for scraping tests.
# Usage: create_html_article "$path/article.html" "Title" "Content" "Author Name" "2025-01-15"
create_html_article() {
    local file="$1"
    local title="${2:-Test Article}"
    local content="${3:-This is the article content.}"
    local author="${4:-}"
    local date="${5:-}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<'HTMLEOF'
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>TITLE_PLACEHOLDER</title>
</head>
<body>
    <article>
        <h1 class="article-title" id="main-title">TITLE_PLACEHOLDER</h1>
HTMLEOF

    if [ -n "$author" ]; then
        echo "        <div class=\"author-name\" data-type=\"author\">$author</div>" >> "$file"
    fi

    if [ -n "$date" ]; then
        echo "        <time class=\"publish-date\" datetime=\"$date\">$date</time>" >> "$file"
    fi

    cat >> "$file" <<'HTMLEOF'
        <div class="article-content">
            CONTENT_PLACEHOLDER
        </div>
    </article>
</body>
</html>
HTMLEOF

    # Replace placeholders
    sed -i.bak "s/TITLE_PLACEHOLDER/$title/g" "$file" && rm -f "${file}.bak"
    sed -i.bak "s/CONTENT_PLACEHOLDER/$content/g" "$file" && rm -f "${file}.bak"
}

# Create an HTML page with a list of article links.
# Usage: create_html_article_list "$path/index.html" 5 "http://127.0.0.1:8080/article"
create_html_article_list() {
    local file="$1"
    local article_count="${2:-3}"
    local article_url_prefix="${3:-http://example.com/article}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<'HTMLEOF'
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Article List</title>
</head>
<body>
    <div class="article-list">
HTMLEOF

    for i in $(seq 1 "$article_count"); do
        echo "        <article class=\"article-item\">" >> "$file"
        echo "            <a href=\"${article_url_prefix}-${i}.html\" class=\"article-link\">Article $i</a>" >> "$file"
        echo "        </article>" >> "$file"
    done

    cat >> "$file" <<'HTMLEOF'
    </div>
</body>
</html>
HTMLEOF
}

# Create an HTML page with pagination (next page link).
# Usage: create_html_with_pagination "$path/page1.html" 3 "http://127.0.0.1:8080/article" "page2.html"
create_html_with_pagination() {
    local file="$1"
    local article_count="${2:-3}"
    local article_url_prefix="${3:-http://example.com/article}"
    local next_page="${4:-}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<'HTMLEOF'
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Article List with Pagination</title>
</head>
<body>
    <div class="article-list">
HTMLEOF

    # Determine starting article number from filename (e.g., page2.html -> start at 4)
    local page_num
    page_num=$(basename "$file" | grep -o '[0-9]\+' | head -1)
    page_num=${page_num:-1}
    local start_num=$(( (page_num - 1) * article_count + 1 ))

    for i in $(seq "$start_num" $((start_num + article_count - 1))); do
        echo "        <article class=\"article-item\">" >> "$file"
        echo "            <a href=\"${article_url_prefix}-${i}.html\" class=\"article-link\">Article $i</a>" >> "$file"
        echo "        </article>" >> "$file"
    done

    if [ -n "$next_page" ]; then
        echo "        <div class=\"pagination\">" >> "$file"
        echo "            <a href=\"$next_page\" class=\"next-page\" rel=\"next\">Next Page</a>" >> "$file"
        echo "        </div>" >> "$file"
    fi

    cat >> "$file" <<'HTMLEOF'
    </div>
</body>
</html>
HTMLEOF
}

# Create malformed HTML for error handling tests.
# Usage: create_malformed_html "$path/broken.html"
create_malformed_html() {
    local file="$1"
    mkdir -p "$(dirname "$file")"

    cat > "$file" <<'HTMLEOF'
<!DOCTYPE html>
<html>
<head>
    <title>Broken Article
</head>
<body>
    <article>
        <h1>Unclosed Title
        <div class="content">
            <p>Paragraph without closing tag
        </div>
    <!-- Missing closing tags for article, body, html -->
HTMLEOF
}

# Create HTML with custom selectors for testing.
# Usage: create_html_with_custom_selectors "$path/custom.html" "Title" "Content" ".custom-author" "#custom-date"
create_html_with_custom_selectors() {
    local file="$1"
    local title="${2:-Custom Article}"
    local content="${3:-Custom content here.}"
    local author_class="${4:-.author}"
    local date_id="${5:-#date}"

    mkdir -p "$(dirname "$file")"

    # Convert class/id to actual HTML attributes
    local author_attr=""
    local date_attr=""

    if [[ "$author_class" == .* ]]; then
        author_attr="class=\"${author_class#.}\""
    elif [[ "$author_class" == \#* ]]; then
        author_attr="id=\"${author_class#\#}\""
    fi

    if [[ "$date_id" == .* ]]; then
        date_attr="class=\"${date_id#.}\""
    elif [[ "$date_id" == \#* ]]; then
        date_attr="id=\"${date_id#\#}\""
    fi

    cat > "$file" <<HTMLEOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>$title</title>
</head>
<body>
    <h1>$title</h1>
    <div $author_attr>John Doe</div>
    <time $date_attr>2025-01-15T12:00:00Z</time>
    <div class="main-content">$content</div>
</body>
</html>
HTMLEOF
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

# Create a scraper config JSON file for direct mode.
# Usage: create_scraper_config_direct "$path/config.json" ".title" ".content" ".author" ".date" "2006-01-02"
create_scraper_config_direct() {
    local file="$1"
    local title_selector="${2:-.article-title}"
    local content_selector="${3:-.article-content}"
    local author_selector="${4:-.author-name}"
    local date_selector="${5:-.publish-date}"
    local date_format="${6:-2006-01-02T15:04:05Z07:00}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<EOF
{
  "discovery_mode": "direct",
  "article_config": {
    "title_selector": "$title_selector",
    "content_selector": "$content_selector"
EOF

    if [ -n "$author_selector" ]; then
        cat >> "$file" <<EOF
,
    "author_selector": "$author_selector"
EOF
    fi

    if [ -n "$date_selector" ]; then
        cat >> "$file" <<EOF
,
    "date_selector": "$date_selector",
    "date_format": "$date_format"
EOF
    fi

    cat >> "$file" <<EOF

  }
}
EOF
}

# Create a scraper config JSON file for list mode.
# Usage: create_scraper_config_list "$path/config.json" ".article-link" ".next-page" 3 ".title" ".content"
create_scraper_config_list() {
    local file="$1"
    local article_selector="${2:-.article-link}"
    local pagination_selector="${3:-}"
    local max_pages="${4:-1}"
    local title_selector="${5:-.article-title}"
    local content_selector="${6:-.article-content}"
    local author_selector="${7:-}"
    local date_selector="${8:-}"
    local date_format="${9:-2006-01-02T15:04:05Z07:00}"

    mkdir -p "$(dirname "$file")"

    cat > "$file" <<EOF
{
  "discovery_mode": "list",
  "list_config": {
    "article_selector": "$article_selector"
EOF

    if [ -n "$pagination_selector" ]; then
        cat >> "$file" <<EOF
,
    "pagination_selector": "$pagination_selector"
EOF
    fi

    cat >> "$file" <<EOF
,
    "max_pages": $max_pages
  },
  "article_config": {
    "title_selector": "$title_selector",
    "content_selector": "$content_selector"
EOF

    if [ -n "$author_selector" ]; then
        cat >> "$file" <<EOF
,
    "author_selector": "$author_selector"
EOF
    fi

    if [ -n "$date_selector" ]; then
        cat >> "$file" <<EOF
,
    "date_selector": "$date_selector",
    "date_format": "$date_format"
EOF
    fi

    cat >> "$file" <<EOF

  }
}
EOF
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
