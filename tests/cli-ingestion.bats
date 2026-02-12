#!/usr/bin/env bats
# Test CLI: spec-2 external news feed ingestion
# Verifies feed fetching, parsing, field mapping, deduplication, and item
# limiting as defined in spec-2.

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
}

teardown_file() {
    cleanup_test_env
}

setup() {
    # Each ingestion test gets a fresh isolated environment
    ISOLATION_DIR="$TEST_DIR/iso-${BATS_TEST_NUMBER}"
    mkdir -p "$ISOLATION_DIR/.news"
    mkdir -p "$ISOLATION_DIR/www"

    ORIG_METADATA="$NEWSFED_METADATA_DSN"
    ORIG_FEED="$NEWSFED_FEED_DSN"
    export NEWSFED_METADATA_DSN="$ISOLATION_DIR/metadata.db"
    export NEWSFED_FEED_DSN="$ISOLATION_DIR/.news"

    newsfed init > /dev/null
}

teardown() {
    # Some tests don't start a mock server, so ignore errors here
    stop_mock_server 2>/dev/null || true
    export NEWSFED_METADATA_DSN="$ORIG_METADATA"
    export NEWSFED_FEED_DSN="$ORIG_FEED"
}

# ── Helpers ──────────────────────────────────────────────────────────────────

# Count items in the local news feed via JSON output
count_feed_items() {
    local out
    out=$(newsfed list --all --format=json --limit=1000)
    printf '%s' "$out" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(len(data['items']))
"
}

# Create RSS feed with N dated items for testing ordering/limiting.
# Items are dated 1..N days ago so most recent is item 1.
# Usage: create_dated_rss_feed "$path" "Title" 25 ["url-prefix"]
create_dated_rss_feed() {
    local file="$1"
    local title="${2:-Test Feed}"
    local item_count="${3:-2}"
    local url_prefix="${4:-http://example.com/dated-article}"

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
        local pub_date
        pub_date=$(date -u -v-${i}d +"%a, %d %b %Y %H:%M:%S +0000" 2>/dev/null || \
            date -u --date="${i} days ago" +"%a, %d %b %Y %H:%M:%S +0000" 2>/dev/null || \
            echo "Wed, 01 Jan 2025 12:00:00 +0000")
        cat >> "$file" <<ITEMEOF
    <item>
      <title>Dated Article $i</title>
      <link>${url_prefix}-${i}</link>
      <description>Description of dated article $i</description>
      <pubDate>$pub_date</pubDate>
    </item>
ITEMEOF
    done

    cat >> "$file" <<RSSEOF
  </channel>
</rss>
RSSEOF
}

# ── Section 2.2: Feed Ingestion Process ──────────────────────────────────────

@test "ingestion: synced RSS items appear in news feed" {
    create_rss_feed "$ISOLATION_DIR/www/feed.xml" "Test Feed" 3
    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Test Source" > /dev/null

    run newsfed sync
    assert_success
    assert_output_contains "Items discovered: 3"

    local count
    count=$(count_feed_items)
    if [ "$count" != "3" ]; then
        echo "Expected 3 items in feed, got: $count"
        return 1
    fi
}

# ── Section 2.2.1: Feed Fetching ────────────────────────────────────────────

@test "ingestion: handles unreachable feed URL gracefully" {
    start_mock_server "$ISOLATION_DIR/www"

    # Point to a nonexistent file on the server (triggers 404 / parse error)
    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/nonexistent.xml" \
        --name="Bad Source" > /dev/null

    run newsfed sync
    assert_failure
    assert_output_contains "Sources failed: 1"
    assert_output_contains "Sources synced: 0"
}

@test "ingestion: handles invalid feed content gracefully" {
    cat > "$ISOLATION_DIR/www/invalid.xml" <<'EOF'
This is not valid XML or RSS content at all.
Just plain text that should fail to parse.
EOF
    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/invalid.xml" \
        --name="Invalid Source" > /dev/null

    run newsfed sync
    assert_failure
    assert_output_contains "Sources failed: 1"
}

# ── Section 2.2.3: Item Limiting ─────────────────────────────────────────────

@test "ingestion: first sync limits to 20 most recent items" {
    create_dated_rss_feed "$ISOLATION_DIR/www/feed.xml" "Big Feed" 25
    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Big Source" > /dev/null

    run newsfed sync
    assert_success

    local count
    count=$(count_feed_items)
    if [ "$count" != "20" ]; then
        echo "Expected 20 items (limited on first sync), got: $count"
        return 1
    fi
}

@test "ingestion: regular polling catches items missed by first sync limit" {
    create_dated_rss_feed "$ISOLATION_DIR/www/feed.xml" "Big Feed" 25
    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Big Source" > /dev/null

    # First sync -- limited to 20
    run newsfed sync
    assert_success

    local count
    count=$(count_feed_items)
    if [ "$count" != "20" ]; then
        echo "Expected 20 items after first sync, got: $count"
        return 1
    fi

    # Second sync -- regular polling, no limit applied.
    # The 5 oldest items that were skipped should now be ingested.
    run newsfed sync
    assert_success

    count=$(count_feed_items)
    if [ "$count" != "25" ]; then
        echo "Expected 25 items after regular polling (no limit), got: $count"
        return 1
    fi
}

@test "ingestion: stale source re-applies 20 item limit" {
    # Start with a small feed
    create_dated_rss_feed "$ISOLATION_DIR/www/feed.xml" "Feed" 5
    start_mock_server "$ISOLATION_DIR/www"

    local add_output
    add_output=$(newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Stale Source")
    local source_id
    source_id=$(extract_uuid "$add_output")

    run newsfed sync
    assert_success

    local count
    count=$(count_feed_items)
    if [ "$count" != "5" ]; then
        echo "Expected 5 items after initial sync, got: $count"
        return 1
    fi

    # Make source stale: set last_fetched_at to 16 days ago
    local stale_time
    stale_time=$(timestamp_days_ago 16)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$stale_time' WHERE source_id = '$source_id'"

    # Replace feed with 25 new items (different URL prefix -- no dedup overlap)
    create_dated_rss_feed "$ISOLATION_DIR/www/feed.xml" "Feed" 25 "http://example.com/new-article"

    # Sync stale source -- limit re-applies, only 20 of 25 new items ingested
    run newsfed sync
    assert_success

    count=$(count_feed_items)
    if [ "$count" != "25" ]; then
        echo "Expected 25 items (5 original + 20 new, limited), got: $count"
        return 1
    fi
}

@test "ingestion: source fetched 14 days ago is not considered stale" {
    # Section 2.2.3 of this spec: limit applies when "more than 15 days"
    # since last fetch. At 14 days, the source is clearly within the
    # non-stale window. (Combined with the 16-day stale test, this brackets
    # the 15-day threshold.)
    create_dated_rss_feed "$ISOLATION_DIR/www/feed.xml" "Feed" 25
    start_mock_server "$ISOLATION_DIR/www"

    local add_output
    add_output=$(newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Recent Source")
    local source_id
    source_id=$(extract_uuid "$add_output")

    # Simulate a previous fetch 14 days ago (not stale)
    local recent_time
    recent_time=$(timestamp_days_ago 14)
    exec_sqlite "UPDATE sources SET last_fetched_at = '$recent_time' WHERE source_id = '$source_id'"

    # Sync -- no limit should apply since 14 < 15 days
    run newsfed sync
    assert_success

    local count
    count=$(count_feed_items)
    if [ "$count" != "25" ]; then
        echo "Expected 25 items (no limit at 14 days), got: $count"
        return 1
    fi
}

# ── Section 2.3.1: RSS to NewsItem Mapping ──────────────────────────────────

@test "ingestion: RSS fields map correctly to NewsItem" {
    # Create RSS feed with specific known field values
    cat > "$ISOLATION_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Test Publisher Name</title>
    <link>http://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>Known RSS Article</title>
      <link>http://example.com/known-rss</link>
      <description>Known RSS summary text</description>
      <dc:creator>Jane Doe</dc:creator>
      <pubDate>Wed, 01 Jan 2025 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Test RSS" > /dev/null

    run newsfed sync
    assert_success

    run newsfed list --all --format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys, re

data = json.load(sys.stdin)
if len(data['items']) != 1:
    print('WRONG_COUNT: expected 1, got ' + str(len(data['items'])))
    sys.exit(0)

item = data['items'][0]
errors = []

# title -- from <title>
if item['title'] != 'Known RSS Article':
    errors.append('title: ' + repr(item['title']))

# summary -- from <description>
if item['summary'] != 'Known RSS summary text':
    errors.append('summary: ' + repr(item['summary']))

# url -- from <link>
if item['url'] != 'http://example.com/known-rss':
    errors.append('url: ' + repr(item['url']))

# publisher -- from channel <title>
if item.get('publisher') != 'Test Publisher Name':
    errors.append('publisher: ' + repr(item.get('publisher')))

# authors -- from <dc:creator>
if item.get('authors') != ['Jane Doe']:
    errors.append('authors: ' + repr(item.get('authors')))

# published_at -- from <pubDate>
if '2025-01-01' not in item.get('published_at', ''):
    errors.append('published_at: ' + repr(item.get('published_at')))

# discovered_at -- set at ingestion time, must be valid RFC 3339
rfc3339 = re.compile(r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}')
if not rfc3339.match(item.get('discovered_at', '')):
    errors.append('discovered_at missing or invalid: ' + repr(item.get('discovered_at')))

# pinned_at -- should be absent (not yet pinned)
if 'pinned_at' in item:
    errors.append('pinned_at should be absent: ' + repr(item['pinned_at']))

# id -- should be a generated UUID
uuid_re = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
if not uuid_re.match(item.get('id', '')):
    errors.append('id not UUID: ' + repr(item.get('id')))

if errors:
    print('ERRORS: ' + '; '.join(errors))
else:
    print('ALL_CORRECT')
")

    if [ "$result" != "ALL_CORRECT" ]; then
        echo "RSS field mapping errors: $result"
        return 1
    fi
}

# ── Section 2.3.2: RSS Deduplication Strategy ───────────────────────────────

@test "ingestion: RSS deduplication prevents duplicate items on re-sync" {
    create_rss_feed "$ISOLATION_DIR/www/feed.xml" "Dedup Feed" 3
    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=rss \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Dedup Source" > /dev/null

    # First sync
    run newsfed sync
    assert_success
    assert_output_contains "Items discovered: 3"

    # Second sync -- same feed, should discover 0 new items
    run newsfed sync
    assert_success
    assert_output_contains "Items discovered: 0"

    # Item count should remain 3
    local count
    count=$(count_feed_items)
    if [ "$count" != "3" ]; then
        echo "Expected 3 items after dedup, got: $count"
        return 1
    fi
}

# ── Section 2.4.1: Atom to NewsItem Mapping ─────────────────────────────────

@test "ingestion: Atom fields map correctly to NewsItem" {
    cat > "$ISOLATION_DIR/www/feed.xml" <<'ATOMEOF'
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test Publisher</title>
  <link href="http://example.com" rel="alternate"/>
  <id>urn:uuid:test-feed</id>
  <updated>2025-06-15T10:00:00Z</updated>
  <entry>
    <title>Known Atom Article</title>
    <link href="http://example.com/atom-known" rel="alternate" type="text/html"/>
    <id>urn:uuid:atom-known-1</id>
    <published>2025-06-01T10:00:00Z</published>
    <updated>2025-06-15T10:00:00Z</updated>
    <summary>Known Atom summary text</summary>
    <author><name>Atom Author Name</name></author>
  </entry>
</feed>
ATOMEOF

    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=atom \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Test Atom" > /dev/null

    run newsfed sync
    assert_success

    run newsfed list --all --format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys, re

data = json.load(sys.stdin)
if len(data['items']) != 1:
    print('WRONG_COUNT: expected 1, got ' + str(len(data['items'])))
    sys.exit(0)

item = data['items'][0]
errors = []

# title -- from <title>
if item['title'] != 'Known Atom Article':
    errors.append('title: ' + repr(item['title']))

# summary -- from <summary>
if item['summary'] != 'Known Atom summary text':
    errors.append('summary: ' + repr(item['summary']))

# url -- from <link rel=\"alternate\">
if item['url'] != 'http://example.com/atom-known':
    errors.append('url: ' + repr(item['url']))

# publisher -- from feed-level <title>
if item.get('publisher') != 'Atom Test Publisher':
    errors.append('publisher: ' + repr(item.get('publisher')))

# authors -- from <author><name>
if item.get('authors') != ['Atom Author Name']:
    errors.append('authors: ' + repr(item.get('authors')))

# published_at -- should prefer <updated> (2025-06-15) over <published> (2025-06-01)
if '2025-06-15' not in item.get('published_at', ''):
    errors.append('published_at should prefer updated: ' + repr(item.get('published_at')))

# pinned_at -- should be absent
if 'pinned_at' in item:
    errors.append('pinned_at should be absent')

# id -- should be a generated UUID (not the Atom <id> element)
uuid_re = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
if not uuid_re.match(item.get('id', '')):
    errors.append('id not UUID: ' + repr(item.get('id')))

if errors:
    print('ERRORS: ' + '; '.join(errors))
else:
    print('ALL_CORRECT')
")

    if [ "$result" != "ALL_CORRECT" ]; then
        echo "Atom field mapping errors: $result"
        return 1
    fi
}

# ── Section 2.4.2: Atom Deduplication Strategy ──────────────────────────────

@test "ingestion: Atom deduplication prevents duplicate items on re-sync" {
    cat > "$ISOLATION_DIR/www/feed.xml" <<'ATOMEOF'
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Dedup Feed</title>
  <link href="http://example.com" rel="alternate"/>
  <id>urn:uuid:dedup-feed</id>
  <updated>2025-06-15T10:00:00Z</updated>
  <entry>
    <title>Atom Entry 1</title>
    <link href="http://example.com/atom-1" rel="alternate"/>
    <id>urn:uuid:atom-dedup-1</id>
    <published>2025-06-14T10:00:00Z</published>
    <summary>First entry</summary>
  </entry>
  <entry>
    <title>Atom Entry 2</title>
    <link href="http://example.com/atom-2" rel="alternate"/>
    <id>urn:uuid:atom-dedup-2</id>
    <published>2025-06-13T10:00:00Z</published>
    <summary>Second entry</summary>
  </entry>
</feed>
ATOMEOF

    start_mock_server "$ISOLATION_DIR/www"

    newsfed sources add --type=atom \
        --url="http://127.0.0.1:${MOCK_SERVER_PORT}/feed.xml" \
        --name="Atom Dedup Source" > /dev/null

    # First sync
    run newsfed sync
    assert_success
    assert_output_contains "Items discovered: 2"

    # Second sync -- same feed
    run newsfed sync
    assert_success
    assert_output_contains "Items discovered: 0"

    # Item count should remain 2
    local count
    count=$(count_feed_items)
    if [ "$count" != "2" ]; then
        echo "Expected 2 items after Atom dedup, got: $count"
        return 1
    fi
}
