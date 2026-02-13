#!/usr/bin/env bats
# Test CLI: spec 5 -- metadata storage for sources and configuration
#
# These tests verify the metadata storage layer (SQLite schema, field
# persistence, timestamps, operational metadata updates) as specified
# in spec 5. Many tests use exec_sqlite to inspect the database directly
# because spec 5 focuses on the storage contract, not just CLI output.

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    mkdir -p "$NEWSFED_FEED_DSN"
    mkdir -p "$TEST_DIR/metadata"
    run newsfed init
}

teardown_file() {
    cleanup_test_env
}

# ---------------------------------------------------------------------------
# Section 3.1 -- SQLite-Based Storage
# ---------------------------------------------------------------------------

@test "spec-5 storage: metadata database is a valid SQLite file" {
    [ -f "$NEWSFED_METADATA_DSN" ]
    file_type=$(file "$NEWSFED_METADATA_DSN")
    echo "$file_type" | grep -qi "sqlite"
}

@test "spec-5 storage: metadata is separate from news item storage" {
    # Metadata DB and feed directory are distinct paths
    [ -f "$NEWSFED_METADATA_DSN" ]
    [ -d "$NEWSFED_FEED_DSN" ]
    [ "$NEWSFED_METADATA_DSN" != "$NEWSFED_FEED_DSN" ]

    # Feed directory should not contain the metadata database
    db_basename=$(basename "$NEWSFED_METADATA_DSN")
    [ ! -f "$NEWSFED_FEED_DSN/$db_basename" ]
}

# ---------------------------------------------------------------------------
# Section 3.1.1 -- Database Schema
# ---------------------------------------------------------------------------

@test "spec-5 schema: sources table has all expected columns" {
    columns=$(sqlite3 "$NEWSFED_METADATA_DSN" "PRAGMA table_info(sources);" 2>/dev/null)

    # Verify each column from the spec exists
    echo "$columns" | grep -q "source_id"
    echo "$columns" | grep -q "source_type"
    echo "$columns" | grep -q "url"
    echo "$columns" | grep -q "name"
    echo "$columns" | grep -q "enabled_at"
    echo "$columns" | grep -q "created_at"
    echo "$columns" | grep -q "updated_at"
    echo "$columns" | grep -q "polling_interval"
    echo "$columns" | grep -q "last_fetched_at"
    echo "$columns" | grep -q "last_modified"
    echo "$columns" | grep -q "etag"
    echo "$columns" | grep -q "fetch_error_count"
    echo "$columns" | grep -q "last_error"
    echo "$columns" | grep -q "scraper_config"
}

@test "spec-5 schema: source_errors table exists with expected columns" {
    columns=$(sqlite3 "$NEWSFED_METADATA_DSN" "PRAGMA table_info(source_errors);" 2>/dev/null)

    echo "$columns" | grep -q "source_id"
    echo "$columns" | grep -q "error"
    echo "$columns" | grep -q "occurred_at"
}

@test "spec-5 schema: source_id column is TEXT type" {
    col_info=$(sqlite3 "$NEWSFED_METADATA_DSN" "PRAGMA table_info(sources);" 2>/dev/null | grep "source_id")
    echo "$col_info" | grep -qi "text"
}

@test "spec-5 schema: source_id is the primary key" {
    col_info=$(sqlite3 "$NEWSFED_METADATA_DSN" "PRAGMA table_info(sources);" 2>/dev/null | grep "source_id")
    # PRAGMA table_info format: cid|name|type|notnull|dflt_value|pk
    # pk=1 means primary key
    pk_val=$(echo "$col_info" | awk -F'|' '{print $6}')
    [ "$pk_val" = "1" ]
}

@test "spec-5 schema: fetch_error_count defaults to 0" {
    col_info=$(sqlite3 "$NEWSFED_METADATA_DSN" "PRAGMA table_info(sources);" 2>/dev/null | grep "fetch_error_count")
    # Check default value field
    echo "$col_info" | grep -q "0"
}

# ---------------------------------------------------------------------------
# Section 2.1 -- Source Metadata (common fields)
# ---------------------------------------------------------------------------

@test "spec-5 source metadata: source_id is a valid UUID" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-uuid.xml -name="UUID Test")
    source_id=$(extract_uuid "$output_add")

    # Verify UUID format (8-4-4-4-12 hex pattern)
    echo "$source_id" | grep -qE '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
}

@test "spec-5 source metadata: created_at is stored in RFC 3339 format" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-created.xml -name="Created Test")
    source_id=$(extract_uuid "$output_add")

    created_at=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")
    # RFC 3339 pattern: YYYY-MM-DDTHH:MM:SSZ or with timezone offset
    echo "$created_at" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}'
}

@test "spec-5 source metadata: updated_at equals created_at on initial creation" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-timestamps.xml -name="Timestamps Test")
    source_id=$(extract_uuid "$output_add")

    created_at=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")
    updated_at=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")

    [ "$created_at" = "$updated_at" ]
}

@test "spec-5 source metadata: all common fields are stored" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-allfields.xml -name="All Fields Test")
    source_id=$(extract_uuid "$output_add")

    # Required fields should not be NULL
    source_type=$(exec_sqlite "SELECT source_type FROM sources WHERE source_id = '$source_id'")
    url=$(exec_sqlite "SELECT url FROM sources WHERE source_id = '$source_id'")
    name=$(exec_sqlite "SELECT name FROM sources WHERE source_id = '$source_id'")
    created_at=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")
    updated_at=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")

    [ -n "$source_type" ]
    [ -n "$url" ]
    [ -n "$name" ]
    [ -n "$created_at" ]
    [ -n "$updated_at" ]

    # Verify correct values
    [ "$source_type" = "rss" ]
    [ "$url" = "https://example.com/meta-allfields.xml" ]
    [ "$name" = "All Fields Test" ]
}

@test "spec-5 source metadata: enabled_at is set on creation (source enabled by default)" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-enabled.xml -name="Enabled Test")
    source_id=$(extract_uuid "$output_add")

    enabled_at=$(exec_sqlite "SELECT enabled_at FROM sources WHERE source_id = '$source_id'")
    [ -n "$enabled_at" ]

    # Verify it's a valid timestamp
    echo "$enabled_at" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}'
}

@test "spec-5 source metadata: enabled_at is NULL when source is disabled" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-disabled.xml -name="Disabled Test")
    source_id=$(extract_uuid "$output_add")

    newsfed sources disable "$source_id" > /dev/null

    enabled_at=$(exec_sqlite "SELECT enabled_at FROM sources WHERE source_id = '$source_id'")
    [ -z "$enabled_at" ]
}

# ---------------------------------------------------------------------------
# Section 2.2 -- Feed Source Metadata
# ---------------------------------------------------------------------------

@test "spec-5 feed metadata: last_fetched_at updates after successful sync" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/sync-meta.xml" "Sync Meta Feed" 1
    start_mock_server "$TEST_DIR/www"

    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/sync-meta.xml" \
        -name="Sync Meta Test")
    source_id=$(extract_uuid "$output_add")

    # Verify last_fetched_at is NULL before sync
    before=$(exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'")
    [ -z "$before" ]

    # Sync
    newsfed sync "$source_id" > /dev/null 2>&1

    stop_mock_server

    # Verify last_fetched_at is now set
    after=$(exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'")
    [ -n "$after" ]
    echo "$after" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}'
}

@test "spec-5 feed metadata: fetch_error_count increments on sync failure" {
    # Fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Add source with unreachable URL
    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:1/nonexistent.xml" \
        -name="Error Count Test")
    source_id=$(extract_uuid "$output_add")

    # Verify initial error count is 0
    before=$(exec_sqlite "SELECT fetch_error_count FROM sources WHERE source_id = '$source_id'")
    [ "$before" = "0" ]

    # Attempt sync (should fail)
    newsfed sync "$source_id" > /dev/null 2>&1 || true

    # Verify error count increased
    after=$(exec_sqlite "SELECT fetch_error_count FROM sources WHERE source_id = '$source_id'")
    [ "$after" -gt 0 ]
}

@test "spec-5 feed metadata: last_error records failure message on sync failure" {
    # Fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Add source with unreachable URL
    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:1/unreachable.xml" \
        -name="Last Error Test")
    source_id=$(extract_uuid "$output_add")

    # Verify last_error is NULL initially
    before=$(exec_sqlite "SELECT last_error FROM sources WHERE source_id = '$source_id'")
    [ -z "$before" ]

    # Attempt sync (should fail)
    newsfed sync "$source_id" > /dev/null 2>&1 || true

    # Verify last_error is now set
    after=$(exec_sqlite "SELECT last_error FROM sources WHERE source_id = '$source_id'")
    [ -n "$after" ]
}

@test "spec-5 feed metadata: fetch_error_count resets after successful sync" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/reset-errors.xml" "Reset Errors Feed" 1
    start_mock_server "$TEST_DIR/www"

    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/reset-errors.xml" \
        -name="Reset Errors Test")
    source_id=$(extract_uuid "$output_add")

    # Manually set error count to simulate previous failures
    exec_sqlite "UPDATE sources SET fetch_error_count = 5, last_error = 'previous error' WHERE source_id = '$source_id'"

    # Sync successfully
    newsfed sync "$source_id" > /dev/null 2>&1

    stop_mock_server

    # Verify error count is reset
    error_count=$(exec_sqlite "SELECT fetch_error_count FROM sources WHERE source_id = '$source_id'")
    [ "$error_count" = "0" ]
}

# ---------------------------------------------------------------------------
# Section 2.3 -- Website Source Metadata
# ---------------------------------------------------------------------------

@test "spec-5 website metadata: scraper_config is stored as valid JSON" {
    # Fresh database
    rm -f "$NEWSFED_METADATA_DSN"
    newsfed init > /dev/null

    cat > "$TEST_DIR/meta-scraper.json" <<EOF
{
  "discovery_mode": "list",
  "list_config": {
    "article_selector": ".article-link",
    "max_pages": 3
  },
  "article_config": {
    "title_selector": "h1.title",
    "content_selector": ".article-body",
    "author_selector": ".author-name",
    "date_selector": "time.published",
    "date_format": "2006-01-02"
  }
}
EOF

    output_add=$(newsfed sources add -type=website \
        -url=https://example.com/meta-scraper \
        -name="Scraper Config Test" \
        -config="$TEST_DIR/meta-scraper.json")
    source_id=$(extract_uuid "$output_add")

    # Read scraper_config from DB
    config_json=$(exec_sqlite "SELECT scraper_config FROM sources WHERE source_id = '$source_id'")
    [ -n "$config_json" ]

    # Verify it's valid JSON that contains expected fields
    echo "$config_json" | python3 -c "import sys, json; d = json.load(sys.stdin); assert d['discovery_mode'] == 'list'"
    echo "$config_json" | python3 -c "import sys, json; d = json.load(sys.stdin); assert d['article_config']['title_selector'] == 'h1.title'"
}

@test "spec-5 website metadata: scraper_config is NULL for non-website sources" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-no-scraper.xml -name="No Scraper Test")
    source_id=$(extract_uuid "$output_add")

    config=$(exec_sqlite "SELECT scraper_config FROM sources WHERE source_id = '$source_id'")
    [ -z "$config" ]
}

# ---------------------------------------------------------------------------
# Section 4.1.1 -- Create Source
# ---------------------------------------------------------------------------

@test "spec-5 create: generates unique UUIDs for different sources" {
    output1=$(newsfed sources add -type=rss -url=https://example.com/meta-unique1.xml -name="Unique 1")
    output2=$(newsfed sources add -type=rss -url=https://example.com/meta-unique2.xml -name="Unique 2")

    id1=$(extract_uuid "$output1")
    id2=$(extract_uuid "$output2")

    [ -n "$id1" ]
    [ -n "$id2" ]
    [ "$id1" != "$id2" ]
}

@test "spec-5 create: timestamps are set to approximately current time" {
    before_epoch=$(date +%s 2>/dev/null)

    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-time.xml -name="Time Test")
    source_id=$(extract_uuid "$output_add")

    after_epoch=$(date +%s 2>/dev/null)

    created_at=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")
    # Verify it's today's date (same YYYY-MM-DD)
    today=$(date -u +%Y-%m-%d 2>/dev/null)
    echo "$created_at" | grep -q "$today"
}

@test "spec-5 create: stores correct source_type for each type" {
    out_rss=$(newsfed sources add -type=rss -url=https://example.com/meta-type-rss.xml -name="RSS Type")
    out_atom=$(newsfed sources add -type=atom -url=https://example.com/meta-type-atom.xml -name="Atom Type")

    id_rss=$(extract_uuid "$out_rss")
    id_atom=$(extract_uuid "$out_atom")

    type_rss=$(exec_sqlite "SELECT source_type FROM sources WHERE source_id = '$id_rss'")
    type_atom=$(exec_sqlite "SELECT source_type FROM sources WHERE source_id = '$id_atom'")

    [ "$type_rss" = "rss" ]
    [ "$type_atom" = "atom" ]
}

# ---------------------------------------------------------------------------
# Section 4.1.3 -- Update Source
# ---------------------------------------------------------------------------

@test "spec-5 update: automatically updates updated_at timestamp" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-update-ts.xml -name="Update TS Test")
    source_id=$(extract_uuid "$output_add")

    original_updated=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")

    # Small delay to ensure timestamp differs
    sleep 1

    newsfed sources update "$source_id" -name="Updated TS Name" > /dev/null

    new_updated=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")

    # updated_at should have changed
    [ "$original_updated" != "$new_updated" ]
}

@test "spec-5 update: does not modify created_at" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-keep-created.xml -name="Keep Created Test")
    source_id=$(extract_uuid "$output_add")

    original_created=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")

    sleep 1

    newsfed sources update "$source_id" -name="Keep Created Updated" > /dev/null

    new_created=$(exec_sqlite "SELECT created_at FROM sources WHERE source_id = '$source_id'")

    [ "$original_created" = "$new_created" ]
}

@test "spec-5 update: polling_interval is stored in database" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-interval-store.xml -name="Interval Store Test")
    source_id=$(extract_uuid "$output_add")

    newsfed sources update "$source_id" -interval=30m > /dev/null

    interval=$(exec_sqlite "SELECT polling_interval FROM sources WHERE source_id = '$source_id'")
    [ "$interval" = "30m" ]
}

# ---------------------------------------------------------------------------
# Section 4.1.4 -- Delete Source
# ---------------------------------------------------------------------------

@test "spec-5 delete: news items persist after source deletion" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/delete-persist.xml" "Delete Persist Feed" 2
    start_mock_server "$TEST_DIR/www"

    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/delete-persist.xml" \
        -name="Delete Persist Test")
    source_id=$(extract_uuid "$output_add")

    # Sync to ingest items
    newsfed sync "$source_id" > /dev/null 2>&1

    stop_mock_server

    # Count items before deletion
    items_before=$(ls "$NEWSFED_FEED_DSN"/*.json 2>/dev/null | wc -l | tr -d ' ')
    [ "$items_before" -gt 0 ]

    # Delete the source
    newsfed sources delete "$source_id" > /dev/null

    # Verify source is gone
    run newsfed sources show "$source_id"
    assert_failure

    # Verify items still exist
    items_after=$(ls "$NEWSFED_FEED_DSN"/*.json 2>/dev/null | wc -l | tr -d ' ')
    [ "$items_after" = "$items_before" ]
}

@test "spec-5 delete: source row is removed from database" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-delete-row.xml -name="Delete Row Test")
    source_id=$(extract_uuid "$output_add")

    # Verify source exists in DB
    count_before=$(exec_sqlite "SELECT count(*) FROM sources WHERE source_id = '$source_id'")
    [ "$count_before" = "1" ]

    newsfed sources delete "$source_id" > /dev/null

    # Verify source is removed from DB
    count_after=$(exec_sqlite "SELECT count(*) FROM sources WHERE source_id = '$source_id'")
    [ "$count_after" = "0" ]
}

# ---------------------------------------------------------------------------
# Section 5.1 -- News Feed Aggregator Integration
# ---------------------------------------------------------------------------

@test "spec-5 aggregator: sync skips disabled sources" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/skip-disabled.xml" "Skip Disabled Feed" 2
    start_mock_server "$TEST_DIR/www"

    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/skip-disabled.xml" \
        -name="Skip Disabled Test")
    source_id=$(extract_uuid "$output_add")

    # Disable the source
    newsfed sources disable "$source_id" > /dev/null

    # Sync all -- should skip disabled source
    run newsfed sync

    stop_mock_server

    assert_success

    # last_fetched_at should still be NULL (source was not fetched)
    last_fetched=$(exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'")
    [ -z "$last_fetched" ]
}

@test "spec-5 aggregator: sync updates operational metadata in database" {
    # Fresh database and feed directory
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init > /dev/null

    # Create RSS feed and start mock server
    create_rss_feed "$TEST_DIR/www/op-meta.xml" "Op Meta Feed" 1
    start_mock_server "$TEST_DIR/www"

    output_add=$(newsfed sources add -type=rss \
        -url="http://127.0.0.1:${MOCK_SERVER_PORT}/op-meta.xml" \
        -name="Op Meta Test")
    source_id=$(extract_uuid "$output_add")

    # Verify operational fields are empty before sync
    last_fetched_before=$(exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'")
    [ -z "$last_fetched_before" ]

    error_count_before=$(exec_sqlite "SELECT fetch_error_count FROM sources WHERE source_id = '$source_id'")
    [ "$error_count_before" = "0" ]

    # Sync
    newsfed sync "$source_id" > /dev/null 2>&1

    stop_mock_server

    # Verify last_fetched_at is updated
    last_fetched_after=$(exec_sqlite "SELECT last_fetched_at FROM sources WHERE source_id = '$source_id'")
    [ -n "$last_fetched_after" ]

    # Error count should remain 0 after successful sync
    error_count_after=$(exec_sqlite "SELECT fetch_error_count FROM sources WHERE source_id = '$source_id'")
    [ "$error_count_after" = "0" ]
}

# ---------------------------------------------------------------------------
# Section 6.1 -- Validation
# ---------------------------------------------------------------------------

@test "spec-5 validation: rejects malformed scraper config JSON" {
    echo "this is not json" > "$TEST_DIR/bad-config.json"

    run newsfed sources add -type=website \
        -url=https://example.com/meta-bad-config \
        -name="Bad Config Test" \
        -config="$TEST_DIR/bad-config.json"
    assert_failure
}

@test "spec-5 validation: rejects nonexistent config file" {
    run newsfed sources add -type=website \
        -url=https://example.com/meta-missing-config \
        -name="Missing Config Test" \
        -config="$TEST_DIR/nonexistent-config.json"
    assert_failure
}

# ---------------------------------------------------------------------------
# Section 6.2 -- Migration and Backup
# ---------------------------------------------------------------------------

@test "spec-5 backup: database file is copyable and backup is usable" {
    # Ensure there's at least one source in the DB
    newsfed sources add -type=rss -url=https://example.com/meta-backup.xml -name="Backup Test" > /dev/null

    # Copy the database file
    cp "$NEWSFED_METADATA_DSN" "$TEST_DIR/backup.db"

    # Verify the copy is a valid SQLite database with data
    count=$(sqlite3 "$TEST_DIR/backup.db" "SELECT count(*) FROM sources;" 2>/dev/null)
    [ "$count" -gt 0 ]

    # Verify we can read source data from the backup
    name=$(sqlite3 "$TEST_DIR/backup.db" "SELECT name FROM sources WHERE name = 'Backup Test';" 2>/dev/null)
    [ "$name" = "Backup Test" ]
}

# ---------------------------------------------------------------------------
# Section 2.1 + 4.1.3 -- Timestamp behaviour across enable/disable
# ---------------------------------------------------------------------------

@test "spec-5 enable/disable: enabled_at is restored when source is re-enabled" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-reenable.xml -name="Re-enable Test")
    source_id=$(extract_uuid "$output_add")

    # Disable
    newsfed sources disable "$source_id" > /dev/null
    enabled_disabled=$(exec_sqlite "SELECT enabled_at FROM sources WHERE source_id = '$source_id'")
    [ -z "$enabled_disabled" ]

    # Re-enable
    newsfed sources enable "$source_id" > /dev/null
    enabled_reenabled=$(exec_sqlite "SELECT enabled_at FROM sources WHERE source_id = '$source_id'")
    [ -n "$enabled_reenabled" ]
    echo "$enabled_reenabled" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}'
}

@test "spec-5 enable/disable: updated_at changes when enabling or disabling" {
    output_add=$(newsfed sources add -type=rss -url=https://example.com/meta-toggle-ts.xml -name="Toggle TS Test")
    source_id=$(extract_uuid "$output_add")

    original_updated=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")

    sleep 1

    newsfed sources disable "$source_id" > /dev/null
    disabled_updated=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")
    [ "$original_updated" != "$disabled_updated" ]

    sleep 1

    newsfed sources enable "$source_id" > /dev/null
    enabled_updated=$(exec_sqlite "SELECT updated_at FROM sources WHERE source_id = '$source_id'")
    [ "$disabled_updated" != "$enabled_updated" ]
}
