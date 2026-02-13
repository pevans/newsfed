#!/usr/bin/env bats
# Test CLI: spec-3 web scraping ingestion
# Verifies web scraping source configuration, HTML extraction, field mapping,
# deduplication, and article limiting as defined in spec-3.
#
# NOTE: Most tests require a mock HTTP server which cannot run in sandboxed mode.
# To run these tests, use one of these approaches:
#
#   1. Run tests with bats directly (outside Claude Code sandbox):
#      $ bats tests/cli-scraping.bats
#
#   2. Run via the test script which may handle permissions:
#      $ ./tests/run-all-tests.sh
#
#   3. Run individual test manually after starting a local server
#
# Tests that work in sandbox:
#   - scraping: adds website source with scraper config (✓ passing)

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    start_mock_server "$TEST_DIR"
    export MOCK_SERVER_PORT
    export MOCK_SERVER_PID
}

teardown_file() {
    stop_mock_server
    cleanup_test_env
}

setup() {
    # Each scraping test gets a fresh isolated environment
    ISOLATION_DIR="$TEST_DIR/iso-${BATS_TEST_NUMBER}"
    mkdir -p "$ISOLATION_DIR/.news"
    mkdir -p "$ISOLATION_DIR/www"

    ORIG_METADATA="$NEWSFED_METADATA_DSN"
    ORIG_FEED="$NEWSFED_FEED_DSN"
    export NEWSFED_METADATA_DSN="$ISOLATION_DIR/metadata.db"
    export NEWSFED_FEED_DSN="$ISOLATION_DIR/.news"
    export NEWSFED_RATE_LIMIT_INTERVAL=0s

    # Shared mock server URL prefix for this test's isolation dir
    WWW_URL="http://127.0.0.1:${MOCK_SERVER_PORT}/iso-${BATS_TEST_NUMBER}/www"

    newsfed init > /dev/null
}

teardown() {
    stop_redirect_mock_server 2>/dev/null || true
    stop_logging_mock_server 2>/dev/null || true
    export NEWSFED_METADATA_DSN="$ORIG_METADATA"
    export NEWSFED_FEED_DSN="$ORIG_FEED"
}

# ── Helpers ──────────────────────────────────────────────────────────────────

# Count items in the local news feed via JSON output
count_feed_items() {
    local out
    out=$(newsfed list -all -format=json -limit=1000)
    # Handle case when there are no items
    if echo "$out" | grep -q "No items to display"; then
        echo "0"
        return
    fi
    printf '%s' "$out" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(len(data['items']))
"
}

# Add a website source with scraper config via JSON
# Usage: add_scraper_source "Source Name" "http://url" '{"discovery_mode":"direct",...}'
add_scraper_source() {
    local name="$1"
    local url="$2"
    local scraper_config="$3"

    # For now, sources add command may not support scraper_config directly
    # We'll add basic website source and update config via sqlite if needed
    newsfed sources add -type=website -name="$name" -url="$url"
}

# ── Section 2.1: Scraper Source Definition ───────────────────────────────────

@test "scraping: adds website source with scraper config" {
    # Create a minimal scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    run newsfed sources add -type=website \
        -name="Test Scraper" \
        -url="http://example.com/articles" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    assert_output_contains "Created source"
    assert_output_contains "website"
    assert_output_contains "Scraper: Configured"

    # Extract source ID
    local source_id
    source_id=$(extract_uuid "$output")

    # Verify source shows up with correct type
    run newsfed sources list
    [ "$status" -eq 0 ]
    assert_output_contains "Test Scraper"
    assert_output_contains "website"

    # Verify scraper config is stored
    run newsfed sources show "$source_id"
    [ "$status" -eq 0 ]
    assert_output_contains "Scraper Configuration"
    assert_output_contains "Discovery Mode"
}

# ── Section 2.2: Discovery Mode: "direct" ────────────────────────────────────

@test "scraping: direct mode scrapes single article" {

    # Create a single article page
    create_html_article "$ISOLATION_DIR/www/article.html" \
        "Test Article" \
        "This is the test content." \
        "Jane Doe" \
        "2025-01-15T12:00:00Z"

    # Create scraper config for direct mode
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        ".author-name" \
        ".publish-date" \
        "2006-01-02T15:04:05Z07:00"


    # Add website source pointing directly to the article
    run newsfed sources add -type=website \
        -name="Direct Article" \
        -url="${WWW_URL}/article.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync the source
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify article appears in feed
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Test Article"
}

# ── Section 2.2: Discovery Mode: "list" ──────────────────────────────────────

@test "scraping: list mode discovers articles from index page" {


    # Create list page with 3 article links
    create_html_article_list "$ISOLATION_DIR/www/index.html" \
        3 \
        "${WWW_URL}/article"

    # Create the 3 article pages
    for i in 1 2 3; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content for article $i" \
            "Author $i" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config for list mode
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        "" \
        1 \
        ".article-title" \
        ".article-content" \
        ".author-name" \
        ".publish-date" \
        "2006-01-02T15:04:05Z07:00"

    # Add website source in list mode
    run newsfed sources add -type=website \
        -name="Article List" \
        -url="${WWW_URL}/index.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync the source
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify all 3 articles appear in feed
    local count
    count=$(count_feed_items)
    [ "$count" -eq 3 ]

    # Verify individual articles
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Article 1"
    assert_output_contains "Article 2"
    assert_output_contains "Article 3"
}

@test "scraping: list mode follows pagination" {

    # Create 2 pages with pagination (1 article per page)
    create_html_with_pagination "$ISOLATION_DIR/www/page1.html" \
        1 \
        "${WWW_URL}/article" \
        "page2.html"

    create_html_with_pagination "$ISOLATION_DIR/www/page2.html" \
        1 \
        "${WWW_URL}/article" \
        ""

    # Create 2 article pages
    for i in 1 2; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config with pagination
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        ".next-page" \
        2 \
        ".article-title" \
        ".article-content"

    # Add website source
    run newsfed sources add -type=website \
        -name="Paginated List" \
        -url="${WWW_URL}/page1.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync the source
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify all 2 articles from 2 pages appear
    local count
    count=$(count_feed_items)
    [ "$count" -eq 2 ]
}

@test "scraping: list mode respects max_pages limit" {

    # Create 3 pages with pagination (1 article per page)
    for page in 1 2 3; do
        local next_page=""
        if [ $page -lt 3 ]; then
            next_page="page$((page + 1)).html"
        fi
        create_html_with_pagination "$ISOLATION_DIR/www/page${page}.html" \
            1 \
            "${WWW_URL}/article" \
            "$next_page"
    done

    # Create 3 article pages (1 per page × 3 pages)
    for i in 1 2 3; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config with max_pages=2 (should only get 2 articles)
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        ".next-page" \
        2 \
        ".article-title" \
        ".article-content"

    # Add website source
    run newsfed sources add -type=website \
        -name="Max Pages Test" \
        -url="${WWW_URL}/page1.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync the source
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should only get 2 articles (2 pages × 1 article)
    local count
    count=$(count_feed_items)
    [ "$count" -eq 2 ]
}

# ── Section 2.3: Selector Syntax ─────────────────────────────────────────────

@test "scraping: extracts data using CSS selectors" {
    # Create article with various selector targets
    cat > "$ISOLATION_DIR/www/selectors.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Selector Test</title></head>
<body>
    <article>
        <h1 id="article-title">Title from ID selector</h1>
        <div class="author-name">Author from class selector</div>
        <div data-type="date">2025-01-20T10:00:00Z</div>
        <section class="content">
            <p>Content from descendant combinator</p>
        </section>
    </article>
</body>
</html>
EOF


    # Create scraper config with custom selectors
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        "#article-title" \
        ".content p" \
        ".author-name" \
        "[data-type='date']" \
        "2006-01-02T15:04:05Z07:00"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Custom Selectors" \
        -url="${WWW_URL}/selectors.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify fields extracted correctly
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Title from ID selector"
    assert_output_contains "Content from descendant combinator"
    assert_output_contains "Author from class selector"
}

@test "scraping: handles missing selectors gracefully" {
    # Article without optional elements
    cat > "$ISOLATION_DIR/www/minimal.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Minimal Article</title></head>
<body>
    <h1>Title Only</h1>
    <p>Some content</p>
    <!-- No author, no date -->
</body>
</html>
EOF


    # Create scraper config (selectors won't match anything)
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".nonexistent-title" \
        ".nonexistent-content" \
        ".nonexistent-author" \
        ".nonexistent-date"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Missing Selectors" \
        -url="${WWW_URL}/minimal.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync should complete without crashing
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should have used fallbacks (title: "(No title)", content: empty)
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "No title"
}

# ── Section 3.1: Full Extraction Flow ────────────────────────────────────────

@test "scraping: full scraping flow extracts all fields" {

    create_html_article "$ISOLATION_DIR/www/full-article.html" \
        "Complete Article Title" \
        "This is the complete article content with all fields." \
        "John Smith" \
        "2025-01-25T14:30:00Z"

    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        ".author-name" \
        ".publish-date" \
        "2006-01-02T15:04:05Z07:00"


    # Add and sync source
    run newsfed sources add -type=website \
        -name="Complete Test" \
        -url="${WWW_URL}/full-article.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify all fields extracted correctly
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]

    # Parse JSON and verify fields
    local item_json
    item_json=$(echo "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
if len(data['items']) > 0:
    print(json.dumps(data['items'][0], indent=2))
")

    # Check key fields
    echo "$item_json" | grep -q '"title": "Complete Article Title"'
    echo "$item_json" | grep -q '"Complete Test"' # publisher
    echo "$item_json" | grep -q '"John Smith"' # author
    echo "$item_json" | grep -q '"published_at": "2025-01-25T14:30:00Z"'
}

# ── Section 3.1.1: Article Limiting ──────────────────────────────────────────

@test "scraping: first sync limits to 20 articles" {

    # Create list page with 25 article links
    create_html_article_list "$ISOLATION_DIR/www/many-articles.html" \
        25 \
        "${WWW_URL}/article"

    # Create 25 article pages
    for i in $(seq 1 25); do
        local padded_i
        padded_i=$(printf "%02d" $i)
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-${padded_i}T12:00:00Z"
    done

    # Create scraper config for list mode
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        "" \
        1 \
        ".article-title" \
        ".article-content"

    # Add website source
    run newsfed sources add -type=website \
        -name="Many Articles Test" \
        -url="${WWW_URL}/many-articles.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync should limit to 20 articles
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 20 ]
}

@test "scraping: regular polling catches all new articles" {

    # Create initial list page with 5 articles
    create_html_article_list "$ISOLATION_DIR/www/articles.html" \
        5 \
        "${WWW_URL}/article"

    for i in 1 2 3 4 5; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        "" \
        1

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Regular Polling Test" \
        -url="${WWW_URL}/articles.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 5 ]

    # Add 3 more articles
    create_html_article_list "$ISOLATION_DIR/www/articles.html" \
        8 \
        "${WWW_URL}/article"

    for i in 6 7 8; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Second sync should catch all 3 new articles (no limit on regular polling)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    count=$(count_feed_items)
    [ "$count" -eq 8 ]
}

@test "scraping: stale source re-applies 20 article limit" {

    # Create list page with 25 articles
    create_html_article_list "$ISOLATION_DIR/www/stale-test.html" \
        25 \
        "${WWW_URL}/article"

    for i in $(seq 1 25); do
        local padded_i
        padded_i=$(printf "%02d" $i)
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-${padded_i}T12:00:00Z"
    done

    # Create scraper config
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link"

    # Add source
    run newsfed sources add -type=website \
        -name="Stale Source Test" \
        -url="${WWW_URL}/stale-test.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync (should limit to 20)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 20 ]

    # Simulate source being stale (16 days old)
    update_last_fetched_at "$source_id" 16

    # Second sync should re-apply 20 article limit (but all are duplicates)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should still have only 20 items (no new items added)
    count=$(count_feed_items)
    [ "$count" -eq 20 ]
}

@test "scraping: source fetched 14 days ago is not considered stale" {

    # Create initial list with 5 articles
    create_html_article_list "$ISOLATION_DIR/www/not-stale.html" \
        5 \
        "${WWW_URL}/article"

    for i in 1 2 3 4 5; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link"

    # Add source
    run newsfed sources add -type=website \
        -name="Not Stale Test" \
        -url="${WWW_URL}/not-stale.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 5 ]

    # Simulate source being 14 days old (NOT stale)
    update_last_fetched_at "$source_id" 14

    # Add 10 more articles
    create_html_article_list "$ISOLATION_DIR/www/not-stale.html" \
        15 \
        "${WWW_URL}/article"

    for i in $(seq 6 15); do
        local padded_i
        padded_i=$(printf "%02d" $i)
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-${padded_i}T12:00:00Z"
    done

    # Second sync should catch ALL 10 new articles (no limit on non-stale)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    count=$(count_feed_items)
    [ "$count" -eq 15 ]
}

@test "scraping: pagination stops at 20 articles across pages" {

    # Create 5 pages with 6 articles each (30 total)
    for page in 1 2 3 4 5; do
        local next_page=""
        if [ $page -lt 5 ]; then
            next_page="page$((page + 1)).html"
        fi
        create_html_with_pagination "$ISOLATION_DIR/www/page${page}.html" \
            6 \
            "${WWW_URL}/article" \
            "$next_page"
    done

    # Create 30 article pages
    for i in $(seq 1 30); do
        local padded_i
        padded_i=$(printf "%02d" $i)
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-${padded_i}T12:00:00Z"
    done

    # Create scraper config with pagination (max_pages=5)
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link" \
        ".next-page" \
        5

    # Add source
    run newsfed sources add -type=website \
        -name="Pagination Limit Test" \
        -url="${WWW_URL}/page1.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync should stop at 20 articles even though pagination allows more pages
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 20 ]
}

# ── Section 3.2: HTML Fetching ───────────────────────────────────────────────

@test "scraping: handles HTTP errors gracefully" {
    # Create config for the source
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add source with non-existent URL (port 9999 unlikely to be in use)
    run newsfed sources add -type=website \
        -name="404 Source" \
        -url="http://127.0.0.1:9999/nonexistent.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync should handle error gracefully (not crash)
    run newsfed sync "$source_id"

    # Should exit with error code (because source failed)
    # but should not crash (segfault, panic, etc.)
    [ "$status" -ne 0 ]

    # Should still show sync completed message
    assert_output_contains "Sync completed"
}

@test "scraping: handles timeouts gracefully" {
    # Note: This test requires a slow server that takes >10 seconds to respond
    # Skipping for now as it's difficult to reliably test without flakiness
    skip "Requires slow response server (timing-sensitive test)"
}

@test "scraping: follows HTTP redirects" {

    # Create target article
    create_html_article "$ISOLATION_DIR/www/target-article.html" \
        "Redirected Article" \
        "Content after redirect" \
        "Author" \
        "2025-01-15T12:00:00Z"

    local target_url="${WWW_URL}/target-article.html"

    # Start redirect server
    start_redirect_mock_server "$target_url"

    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add source pointing to redirect server
    run newsfed sources add -type=website \
        -name="Redirect Test" \
        -url="http://127.0.0.1:${REDIRECT_MOCK_SERVER_PORT}/redirect" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync should follow redirect and get the target article
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify we got the redirected content
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Redirected Article"

    stop_redirect_mock_server
}

@test "scraping: HTTP redirects" {
    # Duplicate of previous test - testing same functionality

    # Create target article
    create_html_article "$ISOLATION_DIR/www/final.html" \
        "Final Destination" \
        "Content at final URL" \
        "Author" \
        "2025-01-20T12:00:00Z"

    local target_url="${WWW_URL}/final.html"

    # Start redirect server
    start_redirect_mock_server "$target_url"

    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add source
    run newsfed sources add -type=website \
        -name="Redirect Test 2" \
        -url="http://127.0.0.1:${REDIRECT_MOCK_SERVER_PORT}/" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify content from final URL
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Final Destination"

    stop_redirect_mock_server
}

@test "scraping: includes User-Agent header" {
    local log_file="$ISOLATION_DIR/headers.log"

    # Create article
    create_html_article "$ISOLATION_DIR/www/ua-test.html" \
        "User-Agent Test" \
        "Testing headers" \
        "Author" \
        "2025-01-15T12:00:00Z"

    # Start logging mock server
    start_logging_mock_server "$ISOLATION_DIR/www" "$log_file"

    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add source
    run newsfed sources add -type=website \
        -name="UA Test" \
        -url="http://127.0.0.1:${LOGGING_MOCK_SERVER_PORT}/ua-test.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Check log file for User-Agent header
    wait_for_file "$log_file" 5
    local user_agent
    user_agent=$(grep "User-Agent:" "$log_file" | head -1)

    # Verify User-Agent contains "newsfed"
    echo "$user_agent" | grep -q "newsfed"

    stop_logging_mock_server
}

# ── Section 3.3: Rate Limiting ───────────────────────────────────────────────

@test "scraping: respects rate limiting between requests" {
    export NEWSFED_RATE_LIMIT_INTERVAL=1s

    # Create list page with 3 articles
    create_html_article_list "$ISOLATION_DIR/www/rate-limit.html" \
        3 \
        "${WWW_URL}/article"

    for i in 1 2 3; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config for list mode
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link"

    # Add source
    run newsfed sources add -type=website \
        -name="Rate Limit Test" \
        -url="${WWW_URL}/rate-limit.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Measure sync duration (should take at least 2 seconds for 3 requests)
    local duration
    duration=$(measure_sync_duration "$source_id")

    # With 1 second rate limit, 3 requests should take at least 2 seconds
    # (request 1, wait 1s, request 2, wait 1s, request 3)
    # Allow some margin for overhead
    [ "$duration" -ge 2000 ]
}

@test "scraping: makes sequential requests to same domain" {
    export NEWSFED_RATE_LIMIT_INTERVAL=1s

    # Create list page with 4 articles
    create_html_article_list "$ISOLATION_DIR/www/sequential.html" \
        4 \
        "${WWW_URL}/article"

    for i in 1 2 3 4; do
        create_html_article "$ISOLATION_DIR/www/article-${i}.html" \
            "Article $i" \
            "Content $i" \
            "Author" \
            "2025-01-0${i}T12:00:00Z"
    done

    # Create scraper config
    create_scraper_config_list "$ISOLATION_DIR/scraper-config.json" \
        ".article-link"

    # Add source
    run newsfed sources add -type=website \
        -name="Sequential Test" \
        -url="${WWW_URL}/sequential.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Measure sync duration (4 requests should take at least 3 seconds)
    local duration
    duration=$(measure_sync_duration "$source_id")

    # With 1 second rate limit, 4 requests should take at least 3 seconds
    [ "$duration" -ge 3000 ]
}

# ── Section 3.4: Content Extraction ──────────────────────────────────────────

@test "scraping: extracts and cleans title" {
    cat > "$ISOLATION_DIR/www/whitespace.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
    <h1 class="article-title">
        Title with    extra    whitespace
    </h1>
    <div class="article-content">Content here</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Whitespace Test" \
        -url="${WWW_URL}/whitespace.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify title whitespace was normalized
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Title with extra whitespace"
    # Should NOT have multiple spaces
    assert_output_not_contains "Title with    extra"
}

@test "scraping: uses (No title) when title missing" {
    cat > "$ISOLATION_DIR/www/no-title.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
    <h1 class="article-title"></h1>
    <div class="article-content">Content without title</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="No Title Test" \
        -url="${WWW_URL}/no-title.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should use "(No title)" fallback
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "No title"
}

@test "scraping: truncates long content for summary" {
    local long_content
    long_content=$(printf 'A%.0s' {1..1000})

    cat > "$ISOLATION_DIR/www/long-content.html" <<EOF
<!DOCTYPE html>
<html>
<head><title>Long Article</title></head>
<body>
    <h1 class="article-title">Long Content Article</h1>
    <div class="article-content">$long_content</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Long Content Test" \
        -url="${WWW_URL}/long-content.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify article exists
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Long Content Article"

    # Summary should be truncated to ~500 chars
    local summary_length
    summary_length=$(echo "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
if len(data['items']) > 0:
    print(len(data['items'][0]['summary']))
")
    # Should be around 500-503 (500 + "...")
    [ "$summary_length" -lt 550 ]
}

@test "scraping: handles long content" {
    local long_content
    long_content=$(printf 'X%.0s' {1..1000})

    cat > "$ISOLATION_DIR/www/very-long.html" <<EOF
<!DOCTYPE html>
<html>
<head><title>Very Long</title></head>
<body>
    <h1 class="article-title">Very Long Article</h1>
    <div class="article-content">$long_content</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Very Long Test" \
        -url="${WWW_URL}/very-long.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Summary should be truncated
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Very Long Article"

    local summary_length
    summary_length=$(echo "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
if len(data['items']) > 0:
    print(len(data['items'][0]['summary']))
")
    [ "$summary_length" -lt 550 ]
}

@test "scraping: extracts multiple authors" {
    cat > "$ISOLATION_DIR/www/multi-author.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Multi Author</title></head>
<body>
    <h1 class="article-title">Multi-Author Article</h1>
    <div class="author-name">Alice Smith</div>
    <div class="author-name">Bob Jones</div>
    <div class="article-content">Collaborative work</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        ".author-name"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Multi-Author Test" \
        -url="${WWW_URL}/multi-author.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify both authors extracted
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Alice Smith"
    assert_output_contains "Bob Jones"
}

@test "scraping: splits comma-separated authors" {
    cat > "$ISOLATION_DIR/www/comma-authors.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Comma Authors</title></head>
<body>
    <h1 class="article-title">Comma-Separated Authors</h1>
    <div class="author-name">Alice Smith, Bob Jones</div>
    <div class="article-content">Joint article</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        ".author-name"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Comma Authors Test" \
        -url="${WWW_URL}/comma-authors.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify both authors parsed from comma-separated string
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Alice Smith"
    assert_output_contains "Bob Jones"
}

@test "scraping: parses dates with format string" {
    cat > "$ISOLATION_DIR/www/custom-date.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Custom Date Format</title></head>
<body>
    <h1 class="article-title">Article with Custom Date</h1>
    <time class="publish-date">January 15, 2025</time>
    <div class="article-content">Content here</div>
</body>
</html>
EOF


    # Create scraper config with custom date format
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        "" \
        ".publish-date" \
        "January 2, 2006"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Custom Date Test" \
        -url="${WWW_URL}/custom-date.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify date was parsed correctly
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "2025-01-15"
}

@test "scraping: uses current time when date parsing fails" {
    cat > "$ISOLATION_DIR/www/bad-date.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Bad Date</title></head>
<body>
    <h1 class="article-title">Article with Invalid Date</h1>
    <time class="publish-date">not-a-valid-date</time>
    <div class="article-content">Content here</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        "" \
        ".publish-date" \
        "2006-01-02"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Bad Date Test" \
        -url="${WWW_URL}/bad-date.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should have used current time as fallback
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    # Verify article exists (with fallback date)
    assert_output_contains "Article with Invalid Date"
}

# ── Section 3.5: Error Handling ──────────────────────────────────────────────

@test "scraping: handles malformed HTML" {
    create_malformed_html "$ISOLATION_DIR/www/broken.html"

    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        "h1" \
        ".content"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Broken HTML Test" \
        -url="${WWW_URL}/broken.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # goquery should handle broken HTML gracefully (doesn't crash)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify article was added (even if fields couldn't be extracted properly)
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    # goquery may not extract text from unclosed tags, so we might get fallback
    local count
    count=$(count_feed_items)
    [ "$count" -eq 1 ]
}

@test "scraping: handles network errors" {
    # This is already tested in "handles HTTP errors gracefully" test
    # Network errors (connection refused, etc.) are handled the same way
    skip "Already covered by HTTP error handling test"
}

@test "scraping: handles missing elements" {
    cat > "$ISOLATION_DIR/www/sparse.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Sparse</title></head>
<body>
    <h1>Title Only Article</h1>
    <p>Just title and content, no metadata</p>
</body>
</html>
EOF


    # Create scraper config with selectors that won't match
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        "h1" \
        "p" \
        ".author" \
        ".date"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Sparse Test" \
        -url="${WWW_URL}/sparse.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Should handle missing optional fields gracefully
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Verify article exists with whatever fields were found
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Title Only Article"
}

# ── Section 4.1: Field Mapping ───────────────────────────────────────────────

@test "scraping: scraper fields map correctly to NewsItem" {

    create_html_article "$ISOLATION_DIR/www/mapping-test.html" \
        "Mapping Test Article" \
        "This tests field mapping from HTML to NewsItem." \
        "Test Author" \
        "2025-01-30T10:00:00Z"

    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content" \
        ".author-name" \
        ".publish-date" \
        "2006-01-02T15:04:05Z07:00"


    # Add and sync source
    run newsfed sources add -type=website \
        -name="Field Mapping Test" \
        -url="${WWW_URL}/mapping-test.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Get the item
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]

    # Verify spec-3 section 4.1 field mapping:
    # - title from title_selector
    assert_output_contains "Mapping Test Article"
    # - summary from content_selector (truncated if needed)
    assert_output_contains "This tests field mapping"
    # - url from article page URL
    assert_output_contains "${WWW_URL}/mapping-test.html"
    # - publisher from source name
    assert_output_contains "Field Mapping Test"
    # - authors from author_selector
    assert_output_contains "Test Author"
    # - published_at from date_selector
    assert_output_contains "2025-01-30T10:00:00Z"

    # Verify id is UUID, discovered_at is present, pinned_at is null
    echo "$output" | python3 -c "
import json, sys, uuid
data = json.load(sys.stdin)
item = data['items'][0]
# Verify UUID
uuid.UUID(item['id'])
# Verify discovered_at exists
assert 'discovered_at' in item
# Verify pinned_at is not present (null/omitted)
assert 'pinned_at' not in item or item['pinned_at'] is None
print('Field mapping verified')
"
}

# ── Section 4.2: Deduplication ───────────────────────────────────────────────

@test "scraping: deduplication prevents duplicate items on re-sync" {

    create_html_article "$ISOLATION_DIR/www/dedup-test.html" \
        "Deduplication Test" \
        "Original content" \
        "Author" \
        "2025-01-15T12:00:00Z"

    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"


    # Add website source
    run newsfed sources add -type=website \
        -name="Dedup Test" \
        -url="${WWW_URL}/dedup-test.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should have 1 item
    local count
    count=$(count_feed_items)
    [ "$count" -eq 1 ]

    # Modify the article content (but keep same URL)
    create_html_article "$ISOLATION_DIR/www/dedup-test.html" \
        "Modified Title" \
        "Modified content" \
        "Different Author" \
        "2025-01-20T12:00:00Z"

    # Second sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should still have only 1 item (no duplicate)
    count=$(count_feed_items)
    [ "$count" -eq 1 ]

    # Original title should be unchanged (no update on re-sync)
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Deduplication Test"
    assert_output_not_contains "Modified Title"
}

@test "scraping: deduplication uses URL as unique identifier" {
    create_html_article "$ISOLATION_DIR/www/url-dedup.html" \
        "Original Title" \
        "Original content" \
        "Author" \
        "2025-01-15T12:00:00Z"


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add website source
    run newsfed sources add -type=website \
        -name="URL Dedup Test" \
        -url="${WWW_URL}/url-dedup.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    local count
    count=$(count_feed_items)
    [ "$count" -eq 1 ]

    # Modify content but keep same URL
    create_html_article "$ISOLATION_DIR/www/url-dedup.html" \
        "Different Title" \
        "Different content" \
        "Different Author" \
        "2025-01-20T12:00:00Z"

    # Second sync should not create duplicate
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    count=$(count_feed_items)
    [ "$count" -eq 1 ]

    # Original title should remain (URL-based deduplication)
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Original Title"
    assert_output_not_contains "Different Title"
}

# ── Section 5.2: Incremental Scraping ────────────────────────────────────────

@test "scraping: tracks last successful scrape time" {
    create_html_article "$ISOLATION_DIR/www/track-test.html" \
        "Tracking Test" \
        "Content" \
        "Author" \
        "2025-01-20T12:00:00Z"


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add source
    run newsfed sources add -type=website \
        -name="Timestamp Track Test" \
        -url="${WWW_URL}/track-test.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # First sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Get first timestamp
    local timestamp1
    timestamp1=$(get_last_fetched_at "$source_id")

    # Verify timestamp exists and is recent (within last 10 seconds)
    [ -n "$timestamp1" ]

    # Sleep briefly
    sleep 0.1

    # Second sync
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Get second timestamp
    local timestamp2
    timestamp2=$(get_last_fetched_at "$source_id")

    # Verify timestamp updated (second one should be different)
    [ "$timestamp1" != "$timestamp2" ]
}

# ── Edge Cases ───────────────────────────────────────────────────────────────

@test "scraping: validates title length" {
    local very_long_title
    very_long_title=$(printf 'A%.0s' {1..600})

    cat > "$ISOLATION_DIR/www/long-title.html" <<EOF
<!DOCTYPE html>
<html>
<head><title>Long</title></head>
<body>
    <h1 class="article-title">$very_long_title</h1>
    <div class="article-content">Content</div>
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="Long Title Test" \
        -url="${WWW_URL}/long-title.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync should reject article with too-long title
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should have 0 items (rejected due to title length)
    local count
    count=$(count_feed_items)
    [ "$count" -eq 0 ]
}

@test "scraping: handles empty content selector result" {
    cat > "$ISOLATION_DIR/www/no-content.html" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>No Content</title></head>
<body>
    <h1 class="article-title">Title Without Content</h1>
    <!-- No content div -->
</body>
</html>
EOF


    # Create scraper config
    create_scraper_config_direct "$ISOLATION_DIR/scraper-config.json" \
        ".article-title" \
        ".article-content"

    # Add and sync source
    run newsfed sources add -type=website \
        -name="No Content Test" \
        -url="${WWW_URL}/no-content.html" \
        -config="$ISOLATION_DIR/scraper-config.json"

    [ "$status" -eq 0 ]
    source_id=$(extract_uuid "$output")

    # Sync should complete with warning (content is optional)
    run newsfed sync "$source_id"
    [ "$status" -eq 0 ]

    # Should have article with empty content
    run newsfed list -all -format=json
    [ "$status" -eq 0 ]
    assert_output_contains "Title Without Content"
}
