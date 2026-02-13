#!/usr/bin/env bats
# Test CLI: spec-1 data model validation
# Verifies news item structure, field types, and optional field behavior
# as defined in spec-1 sections 2.1 and 2.2.

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    mkdir -p "$NEWSFED_FEED_DSN"

    ONE_DAY_AGO=$(timestamp_days_ago 1)
    TWO_DAYS_AGO=$(timestamp_days_ago 2)
    THIRTY_DAYS_AGO=$(timestamp_days_ago 30)

    # Item 1: Complete item with all fields populated (including pinned_at)
    cat > "$NEWSFED_FEED_DSN/11111111-1111-1111-1111-111111111111.json" <<EOF
{
  "id": "11111111-1111-1111-1111-111111111111",
  "title": "Complete Article",
  "summary": "A complete article with all fields populated.",
  "url": "https://example.com/complete",
  "publisher": "Test Publisher",
  "authors": ["Alice Smith", "Bob Jones"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$TWO_DAYS_AGO",
  "pinned_at": "$ONE_DAY_AGO"
}
EOF

    # Item 2: Item without publisher (optional field omitted)
    cat > "$NEWSFED_FEED_DSN/22222222-2222-2222-2222-222222222222.json" <<EOF
{
  "id": "22222222-2222-2222-2222-222222222222",
  "title": "No Publisher Article",
  "summary": "An article without a publisher field.",
  "url": "https://example.com/no-publisher",
  "authors": ["Charlie Brown"],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
EOF

    # Item 3: Item that is not pinned
    cat > "$NEWSFED_FEED_DSN/33333333-3333-3333-3333-333333333333.json" <<EOF
{
  "id": "33333333-3333-3333-3333-333333333333",
  "title": "Unpinned Article",
  "summary": "An article that is not pinned.",
  "url": "https://example.com/unpinned",
  "publisher": "Another Publisher",
  "authors": [],
  "published_at": "$ONE_DAY_AGO",
  "discovered_at": "$ONE_DAY_AGO"
}
EOF

    # Item 4: Old item to verify indefinite retention
    cat > "$NEWSFED_FEED_DSN/44444444-4444-4444-4444-444444444444.json" <<EOF
{
  "id": "44444444-4444-4444-4444-444444444444",
  "title": "Ancient Article",
  "summary": "An article from long ago.",
  "url": "https://example.com/ancient",
  "publisher": "Old Times Press",
  "authors": ["Historic Writer"],
  "published_at": "$THIRTY_DAYS_AGO",
  "discovered_at": "$THIRTY_DAYS_AGO"
}
EOF
}

teardown_file() {
    cleanup_test_env
}

# ── Section 2.1: Structure of a single news item ────────────────────────────

@test "spec-1 data model: JSON output contains all required fields" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
required = ['id', 'title', 'summary', 'url', 'authors', 'published_at', 'discovered_at']
for item in data['items']:
    if item['title'] == 'Complete Article':
        missing = [f for f in required if f not in item]
        if missing:
            print('MISSING: ' + ', '.join(missing))
        else:
            print('ALL_PRESENT')
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "ALL_PRESENT" ]; then
        echo "Expected all required fields present, got: $result"
        return 1
    fi
}

@test "spec-1 data model: id field is a valid UUID" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys, re
data = json.load(sys.stdin)
uuid_re = re.compile(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
for item in data['items']:
    if not uuid_re.match(item['id']):
        print('INVALID: ' + item['id'])
        sys.exit(0)
print('ALL_VALID')
")
    if [ "$result" != "ALL_VALID" ]; then
        echo "Expected all IDs to be valid UUIDs, got: $result"
        return 1
    fi
}

@test "spec-1 data model: publisher field is optional and omitted when absent" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'No Publisher Article':
        if 'publisher' in item:
            print('PRESENT: ' + repr(item['publisher']))
        else:
            print('OMITTED')
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "OMITTED" ]; then
        echo "Expected publisher field to be omitted, got: $result"
        return 1
    fi
}

@test "spec-1 data model: authors field is a list of strings" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    authors = item.get('authors')
    if not isinstance(authors, list):
        print('NOT_LIST: ' + item['title'] + ' -> ' + repr(authors))
        sys.exit(0)
    for a in authors:
        if not isinstance(a, str):
            print('NOT_STRING: ' + item['title'] + ' -> ' + repr(a))
            sys.exit(0)
print('ALL_VALID')
")
    if [ "$result" != "ALL_VALID" ]; then
        echo "Expected authors to be lists of strings, got: $result"
        return 1
    fi
}

@test "spec-1 data model: authors supports multiple entries" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'Complete Article':
        authors = item['authors']
        if len(authors) >= 2 and 'Alice Smith' in authors and 'Bob Jones' in authors:
            print('MULTIPLE_AUTHORS')
        else:
            print('UNEXPECTED: ' + repr(authors))
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "MULTIPLE_AUTHORS" ]; then
        echo "Expected multiple authors, got: $result"
        return 1
    fi
}

@test "spec-1 data model: authors can be an empty list" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'Unpinned Article':
        if item['authors'] == []:
            print('EMPTY_LIST')
        else:
            print('UNEXPECTED: ' + repr(item['authors']))
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "EMPTY_LIST" ]; then
        echo "Expected empty authors list, got: $result"
        return 1
    fi
}

@test "spec-1 data model: pinned_at present when item is pinned" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'Complete Article':
        if 'pinned_at' in item and item['pinned_at']:
            print('PRESENT')
        else:
            print('MISSING')
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "PRESENT" ]; then
        echo "Expected pinned_at to be present for pinned item, got: $result"
        return 1
    fi
}

@test "spec-1 data model: pinned_at omitted when item is not pinned" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'Unpinned Article':
        if 'pinned_at' in item:
            print('PRESENT: ' + repr(item['pinned_at']))
        else:
            print('OMITTED')
        break
else:
    print('ITEM_NOT_FOUND')
")
    if [ "$result" != "OMITTED" ]; then
        echo "Expected pinned_at to be omitted for unpinned item, got: $result"
        return 1
    fi
}

@test "spec-1 data model: timestamps use RFC 3339 format" {
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys, re
data = json.load(sys.stdin)
# RFC 3339: YYYY-MM-DDTHH:MM:SSZ or with fractional seconds and timezone offset
rfc3339 = re.compile(
    r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$'
)
for item in data['items']:
    for field in ['published_at', 'discovered_at']:
        val = item.get(field, '')
        if not rfc3339.match(val):
            print('INVALID: ' + field + '=' + val + ' in ' + item['title'])
            sys.exit(0)
    if 'pinned_at' in item:
        val = item['pinned_at']
        if not rfc3339.match(val):
            print('INVALID: pinned_at=' + val + ' in ' + item['title'])
            sys.exit(0)
print('ALL_VALID')
")
    if [ "$result" != "ALL_VALID" ]; then
        echo "Expected all timestamps in RFC 3339 format, got: $result"
        return 1
    fi
}

@test "spec-1 data model: published_at prefers updated date over published date" {
    # This test exercises the sync pipeline to verify that when a feed entry
    # has both <published> and <updated> dates, published_at uses the updated
    # (most current) date as specified in spec-1 section 2.1.

    # Use an isolated environment so we don't interfere with other tests
    local sync_dir="$TEST_DIR/sync-test"
    mkdir -p "$sync_dir/.news"
    local orig_meta="$NEWSFED_METADATA_DSN"
    local orig_feed="$NEWSFED_FEED_DSN"
    export NEWSFED_METADATA_DSN="$sync_dir/metadata.db"
    export NEWSFED_FEED_DSN="$sync_dir/.news"

    # Initialize storage
    run newsfed init
    assert_success

    # Create Atom feed with both published and updated dates
    local www_dir="$sync_dir/www"
    mkdir -p "$www_dir"
    cat > "$www_dir/feed.xml" <<'ATOMEOF'
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Feed</title>
  <link href="http://example.com"/>
  <updated>2025-06-15T10:00:00Z</updated>
  <entry>
    <title>Article With Both Dates</title>
    <link href="http://example.com/both-dates"/>
    <id>urn:uuid:article-both</id>
    <published>2025-06-01T10:00:00Z</published>
    <updated>2025-06-15T10:00:00Z</updated>
    <summary>Has both published and updated dates.</summary>
  </entry>
  <entry>
    <title>Article With Only Published</title>
    <link href="http://example.com/pub-only"/>
    <id>urn:uuid:article-pub</id>
    <published>2025-06-10T08:00:00Z</published>
    <summary>Has only a published date.</summary>
  </entry>
</feed>
ATOMEOF

    # Start mock server and sync
    start_mock_server "$www_dir"

    run newsfed sources add \
        -name="Test Atom" \
        -type=atom \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml"
    assert_success

    run newsfed sync
    assert_success

    # Verify published_at values
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
results = {}
for item in data['items']:
    if item['title'] == 'Article With Both Dates':
        # Should use updated date (2025-06-15), not published (2025-06-01)
        if '2025-06-15' in item['published_at']:
            results['both'] = 'OK'
        else:
            results['both'] = 'WRONG: ' + item['published_at']
    elif item['title'] == 'Article With Only Published':
        # Should use published date (2025-06-10)
        if '2025-06-10' in item['published_at']:
            results['pub'] = 'OK'
        else:
            results['pub'] = 'WRONG: ' + item['published_at']

if results.get('both') == 'OK' and results.get('pub') == 'OK':
    print('CORRECT')
else:
    print('FAILED: ' + repr(results))
")

    stop_mock_server
    export NEWSFED_METADATA_DSN="$orig_meta"
    export NEWSFED_FEED_DSN="$orig_feed"

    if [ "$result" != "CORRECT" ]; then
        echo "Expected published_at to prefer updated date, got: $result"
        return 1
    fi
}

# ── Section 2.2: Structure of a news feed ────────────────────────────────────

@test "spec-1 data model: items remain in feed indefinitely" {
    # An item from 30 days ago should still be retrievable via -all
    run newsfed list -all -format=json
    assert_success

    local result
    result=$(printf '%s' "$output" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for item in data['items']:
    if item['title'] == 'Ancient Article':
        print('FOUND')
        break
else:
    print('NOT_FOUND')
")
    if [ "$result" != "FOUND" ]; then
        echo "Expected old item to remain in feed, got: $result"
        return 1
    fi
}

@test "spec-1 data model: client filters items not the feed" {
    # Default list (without -all) filters by recency, but -all reveals
    # the feed itself retains everything. This verifies that filtering is
    # a client concern, not a feed concern (spec-1 section 2.2).
    run newsfed list
    assert_success
    assert_output_not_contains "Ancient Article"

    run newsfed list -all
    assert_success
    assert_output_contains "Ancient Article"
}
