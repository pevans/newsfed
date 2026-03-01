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

@test "tui: items frame scrolls viewport to follow cursor" {
    mkdir -p "$TEST_DIR/www"

    # Create a feed with 10 items (distinct dates) so the list exceeds the
    # visible height of a small terminal and requires scrolling.  Items are
    # sorted newest-first, so Juliet (Jan 10) appears at the top and Alpha
    # (Jan 01) at the bottom.
    cat > "$TEST_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Scroll Test Feed</title>
    <link>http://example.com</link>
    <description>Feed for scroll testing</description>
    <item><title>Story Alpha</title><link>http://example.com/alpha</link><pubDate>Thu, 01 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Bravo</title><link>http://example.com/bravo</link><pubDate>Fri, 02 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Charlie</title><link>http://example.com/charlie</link><pubDate>Sat, 03 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Delta</title><link>http://example.com/delta</link><pubDate>Sun, 04 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Echo</title><link>http://example.com/echo</link><pubDate>Mon, 05 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Foxtrot</title><link>http://example.com/foxtrot</link><pubDate>Tue, 06 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Golf</title><link>http://example.com/golf</link><pubDate>Wed, 07 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Hotel</title><link>http://example.com/hotel</link><pubDate>Thu, 08 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story India</title><link>http://example.com/india</link><pubDate>Fri, 09 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
    <item><title>Story Juliet</title><link>http://example.com/juliet</link><pubDate>Sat, 10 Jan 2026 00:00:00 GMT</pubDate><description>desc</description></item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Scroll Test Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    # Use a short terminal so only a few items fit on screen at once.
    # Height 8 with borders gives ~4 inner rows; at 1 line per item only
    # 4 items fit, ensuring the 10-item list requires scrolling.
    tui_start 80 8
    tui_wait_for "Scroll Test Source" 5

    tui_send_keys "Tab"
    sleep 0.3

    # Newest items (Juliet, India, ...) appear at the top; oldest (Alpha) is
    # off-screen at the bottom.
    tui_assert_contains "Story Juliet"
    tui_assert_not_contains "Story Alpha"

    # Press j enough times to move the cursor past the bottom of the
    # viewport.  The list should scroll so older items become visible.
    for i in $(seq 1 9); do
        tui_send_keys "j"
        sleep 0.05
    done
    sleep 0.3

    tui_assert_contains "Story Alpha"
    tui_assert_not_contains "Story Juliet"

    # Scroll back up to the top.
    for i in $(seq 1 9); do
        tui_send_keys "k"
        sleep 0.05
    done
    sleep 0.3

    tui_assert_contains "Story Juliet"
    tui_assert_not_contains "Story Alpha"
}

# ---------------------------------------------------------------------------
# Mode line
# ---------------------------------------------------------------------------

@test "tui: mode line shows keyboard shortcut summary on startup" {
    tui_start
    tui_wait_for "No sources." 5

    local screen
    screen=$(tui_capture)

    echo "$screen" | grep -qF "[Q]uit"
    echo "$screen" | grep -qF "[R]efresh"
    echo "$screen" | grep -qF "[Tab]"
    echo "$screen" | grep -qF "[Enter]"
}

@test "tui: item detail modal scrolls long content with j and k" {
    mkdir -p "$TEST_DIR/www"

    # Create an item with a long description so the modal content exceeds the
    # visible height and scrolling is required.  Two unique sentinel words --
    # ScrollTestTop (at the start) and ScrollTestBottom (at the end) -- let
    # us confirm that the viewport moves when j/k are pressed.
    cat > "$TEST_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Scroll Test Feed</title>
    <link>http://example.com</link>
    <description>Feed for scroll testing</description>
    <item>
      <title>Long Article</title>
      <link>http://example.com/long-article</link>
      <description>ScrollTestTop begins the article body with words that push content across several wrapped lines when displayed inside the narrow modal window. The second sentence adds more prose so the summary occupies additional rows after word-wrapping. A third sentence further pads the content ensuring the opening paragraph alone spans several visible lines. The fourth sentence continues adding filler text that is long enough to wrap onto its own line within the modal. The fifth sentence pushes the bottom sentinel well off the bottom edge of the initial viewport. The sixth sentence ensures a comfortable buffer of lines separates the two markers. The seventh sentence makes certain at least a dozen lines lie between the sentinel words. The eighth sentence adds even more padding so that scrolling a handful of times is definitely required to reach the end. ScrollTestBottom</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Scroll Test Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    # Use a short terminal (height=10) so the modal viewport is only 6 lines
    # tall (height minus 4 lines of border/padding overhead), leaving very
    # little room for the summary after the 4-line header.
    tui_start 120 10
    tui_wait_for "Scroll Test Source" 5

    tui_send_keys "Tab"
    tui_wait_for "Long Article" 5
    tui_send_keys "Enter"
    sleep 0.3

    # Initially, the top of the content is visible; the bottom is not.
    tui_assert_contains "ScrollTestTop"
    tui_assert_not_contains "ScrollTestBottom"

    # Scroll down until ScrollTestBottom comes into view.
    for i in $(seq 1 30); do
        tui_send_keys "j"
        sleep 0.03
    done
    sleep 0.3

    tui_assert_contains "ScrollTestBottom"

    # Scroll back up; ScrollTestTop should reappear and ScrollTestBottom
    # should leave the viewport again.
    for i in $(seq 1 30); do
        tui_send_keys "k"
        sleep 0.03
    done
    sleep 0.3

    tui_assert_contains "ScrollTestTop"
    tui_assert_not_contains "ScrollTestBottom"
}

@test "tui: item detail modal scroll-up works after scrolling past the end" {
    mkdir -p "$TEST_DIR/www"

    cat > "$TEST_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Scroll Test Feed</title>
    <link>http://example.com</link>
    <description>Feed for scroll testing</description>
    <item>
      <title>Long Article</title>
      <link>http://example.com/long-article</link>
      <description>ScrollTestTop begins the article body with words that push content across several wrapped lines when displayed inside the narrow modal window. The second sentence adds more prose so the summary occupies additional rows after word-wrapping. A third sentence further pads the content ensuring the opening paragraph alone spans several visible lines. The fourth sentence continues adding filler text that is long enough to wrap onto its own line within the modal. The fifth sentence pushes the bottom sentinel well off the bottom edge of the initial viewport. The sixth sentence ensures a comfortable buffer of lines separates the two markers. The seventh sentence makes certain at least a dozen lines lie between the sentinel words. The eighth sentence adds even more padding so that scrolling a handful of times is definitely required to reach the end. ScrollTestBottom</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Scroll Test Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start 120 10
    tui_wait_for "Scroll Test Source" 5

    tui_send_keys "Tab"
    tui_wait_for "Long Article" 5
    tui_send_keys "Enter"
    sleep 0.3

    # Scroll down many more times than necessary to reach the bottom. This
    # overshoots the true maximum scroll offset, which is the condition that
    # triggered the bug where subsequent k presses had no visible effect.
    for i in $(seq 1 50); do
        tui_send_keys "j"
        sleep 0.03
    done
    sleep 0.3

    # Confirm we are sitting at the bottom.
    tui_assert_contains "ScrollTestBottom"

    # A single k press must move the viewport up by one line. ScrollTestBottom
    # lives on the very last line of content, so moving up by one should push
    # it out of the visible area.
    tui_send_keys "k"
    sleep 0.3

    tui_assert_not_contains "ScrollTestBottom"
}
