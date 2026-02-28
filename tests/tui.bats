#!/usr/bin/env bats
# Black box tests for the newsfed TUI (tmux-based)

load test_helper

setup_file() {
    if ! command -v tmux >/dev/null 2>&1; then
        echo "tui.bats: tmux not found -- cannot run TUI tests" >&2
        return 1
    fi
    setup_test_env
    build_newsfed "$TEST_DIR"
}

teardown_file() {
    cleanup_test_env
}

setup() {
    if ! command -v tmux >/dev/null 2>&1; then
        echo "tmux is not available" >&2
        return 1
    fi
    tui_stop  # ensure no leftover session from a previous test
    # Reinitialize storage so each test starts with a clean database and feed dir.
    rm -f "$NEWSFED_METADATA_DSN"
    rm -rf "$NEWSFED_FEED_DSN"
    mkdir -p "$NEWSFED_FEED_DSN"
    newsfed init >/dev/null 2>&1
}

teardown() {
    tui_stop
    stop_mock_server 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Startup
# ---------------------------------------------------------------------------

@test "tui: starts and displays source frame" {
    tui_start
    tui_wait_for "No sources." 5
    tui_assert_contains "No sources."
}

@test "tui: q quits the TUI" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "q"

    # After quitting, the tmux session should disappear within a few seconds.
    local i=0
    while [ $i -lt 30 ]; do
        if ! tmux has-session -t "$TUI_SESSION" 2>/dev/null; then
            return 0
        fi
        sleep 0.1
        i=$(( i + 1 ))
    done

    echo "TUI did not exit after pressing q"
    return 1
}

# ---------------------------------------------------------------------------
# Source frame content
# ---------------------------------------------------------------------------

@test "tui: shows sources sorted alphabetically" {
    newsfed sources add -type=rss -url=https://z.example.com/feed -name="Zebra News"
    newsfed sources add -type=rss -url=https://a.example.com/feed -name="Alpha News"
    newsfed sources add -type=rss -url=https://m.example.com/feed -name="Mid News"

    tui_start
    tui_wait_for "Alpha News" 5

    local screen
    screen=$(tui_capture)

    # All three sources must appear.
    echo "$screen" | grep -qF "Alpha News"
    echo "$screen" | grep -qF "Mid News"
    echo "$screen" | grep -qF "Zebra News"

    # Alpha must appear before Mid, and Mid before Zebra.
    local pos_alpha pos_mid pos_zebra
    pos_alpha=$(echo "$screen" | grep -n "Alpha News" | head -1 | cut -d: -f1)
    pos_mid=$(echo "$screen" | grep -n "Mid News" | head -1 | cut -d: -f1)
    pos_zebra=$(echo "$screen" | grep -n "Zebra News" | head -1 | cut -d: -f1)

    [ "$pos_alpha" -lt "$pos_mid" ]
    [ "$pos_mid" -lt "$pos_zebra" ]
}

@test "tui: source entry shows type in parentheses" {
    newsfed sources add -type=rss -url=https://type.example.com/feed -name="Type Test"

    tui_start
    tui_wait_for "Type Test" 5

    tui_assert_contains "(rss)"
}

@test "tui: source entry shows Last updated: Never when never fetched" {
    newsfed sources add -type=rss -url=https://never.example.com/feed -name="Never Fetched"

    tui_start
    tui_wait_for "Never Fetched" 5

    tui_assert_contains "Never"
}

# ---------------------------------------------------------------------------
# Frame focus and navigation
# ---------------------------------------------------------------------------

@test "tui: tab switches focus to items frame" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Tab Test Feed" 1
    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="Tab Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "Tab Source" 5

    tui_send_keys "Tab"  # tab to items frame
    tui_wait_for "Article 1" 5

    # With the items frame focused, Enter should open the item detail modal
    # (showing "Title:"), not the source management modal ("Edit"/"Delete").
    tui_send_keys "Enter"
    sleep 0.3

    tui_assert_contains "Title:"
    tui_assert_not_contains "Edit"
}

@test "tui: j and k move cursor through source list" {
    newsfed sources add -type=rss -url=https://first.example.com/feed -name="First Source"
    newsfed sources add -type=rss -url=https://second.example.com/feed -name="Second Source"

    tui_start
    tui_wait_for "First Source" 5

    # At startup, First Source is selected (alphabetical order). Open its modal
    # and confirm the first source's URL appears, then close it.
    tui_send_keys "Enter"
    tui_wait_for "first.example.com" 3
    tui_assert_contains "first.example.com"
    tui_assert_not_contains "second.example.com"
    tui_send_keys "Escape"
    sleep 0.1

    # Press j to move down to Second Source, then open its modal.
    tui_send_keys "j"
    sleep 0.2
    tui_send_keys "Enter"
    tui_wait_for "second.example.com" 3
    tui_assert_contains "second.example.com"
    tui_assert_not_contains "first.example.com"
    tui_send_keys "Escape"
    sleep 0.1

    # Press k to move back up to First Source and confirm again.
    tui_send_keys "k"
    sleep 0.2
    tui_send_keys "Enter"
    tui_wait_for "first.example.com" 3
    tui_assert_contains "first.example.com"
    tui_assert_not_contains "second.example.com"
}

# ---------------------------------------------------------------------------
# Source management modal
# ---------------------------------------------------------------------------

@test "tui: enter on source opens source management modal" {
    newsfed sources add -type=rss -url=https://modal.example.com/feed -name="Modal Source"

    tui_start
    tui_wait_for "Modal Source" 5

    tui_send_keys "Enter"
    sleep 0.3

    # The modal should show Edit and Delete options.
    tui_assert_contains "Edit"
    tui_assert_contains "Delete"
}

@test "tui: escape closes source management modal" {
    newsfed sources add -type=rss -url=https://esc.example.com/feed -name="Esc Source"

    tui_start
    tui_wait_for "Esc Source" 5

    tui_send_keys "Enter"
    sleep 0.2
    tui_wait_for "Edit" 3

    tui_send_keys "Escape"
    sleep 0.2

    tui_assert_not_contains "Edit"
    tui_assert_not_contains "Delete"
}

# ---------------------------------------------------------------------------
# News items frame: items from an ingested source appear
# ---------------------------------------------------------------------------

@test "tui: items from synced RSS source appear in items frame" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Items Test Feed" 3
    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="Items Test Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "Items Test Source" 5

    # Tab to the items frame.
    tui_send_keys "Tab"
    sleep 0.3

    # The items frame should show at least one article.
    tui_assert_contains "Article"
}

@test "tui: items frame shows item titles from selected source" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Title Test Feed" 2
    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="Title Test Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "Title Test Source" 5

    # Tab to the items frame.
    tui_send_keys "Tab"
    sleep 0.3

    # The RSS fixture creates items named "Article 1", "Article 2", etc.
    tui_assert_contains "Article 1"
}

@test "tui: selecting a different source updates the items frame" {
    mkdir -p "$TEST_DIR/www"

    # Each feed uses distinct URLs so deduplication does not suppress beta's items.
    cat > "$TEST_DIR/www/alpha.xml" << 'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Alpha Feed</title><link>http://alpha.example.com</link><description>Alpha</description>
<item><title>Alpha Story</title><link>http://alpha.example.com/story1</link><description>desc</description></item>
</channel></rss>
RSSEOF
    cat > "$TEST_DIR/www/beta.xml" << 'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Beta Feed</title><link>http://beta.example.com</link><description>Beta</description>
<item><title>Beta Story</title><link>http://beta.example.com/story1</link><description>desc</description></item>
</channel></rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/alpha.xml" -name="Alpha Source"
    alpha_id=$(extract_uuid "$output")
    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/beta.xml"  -name="Beta Source"
    beta_id=$(extract_uuid "$output")
    newsfed sync "$alpha_id" >/dev/null 2>&1
    newsfed sync "$beta_id"  >/dev/null 2>&1

    stop_mock_server

    tui_start
    # Sources are sorted alphabetically; Alpha Source is selected first.
    tui_wait_for "Alpha Source" 5

    # Tab to the items frame and confirm Alpha Source's item is visible.
    tui_send_keys "Tab"
    tui_wait_for "Alpha Story" 5
    tui_assert_contains "Alpha Story"

    # Tab back to the sources frame and move down to Beta Source.
    tui_send_keys "Tab"
    sleep 0.1
    tui_send_keys "j"

    # The items frame should now show Beta Source's item.
    tui_wait_for "Beta Story" 5
    tui_assert_contains "Beta Story"
}

@test "tui: enter on item opens item detail modal" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Modal Item Feed" 1
    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="Modal Item Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "Modal Item Source" 5

    tui_send_keys "	"   # switch to items frame
    sleep 0.2
    tui_wait_for "Article" 3

    tui_send_keys "Enter"
    sleep 0.3

    # The item detail modal shows labeled fields.
    tui_assert_contains "Title:"
    tui_assert_contains "Published:"
    tui_assert_contains "URL:"
}
