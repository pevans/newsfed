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

@test "tui: sources frame border shows Feeds label" {
    tui_start
    tui_wait_for "No sources." 5
    tui_assert_contains "Feeds"
}

@test "tui: news items frame border shows Feed Items label" {
    tui_start
    tui_wait_for "No sources." 5
    tui_assert_contains "Feed Items"
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

@test "tui: source entry shows (never) when never fetched" {
    newsfed sources add -type=rss -url=https://never.example.com/feed -name="Never Fetched"

    tui_start
    tui_wait_for "Never Fetched" 5

    tui_assert_contains "(never)"
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

@test "tui: source management modal shows source type" {
    newsfed sources add -type=rss -url=https://type-modal.example.com/feed -name="Type Modal Src"

    tui_start
    tui_wait_for "Type Modal Src" 5

    tui_send_keys "Enter"
    sleep 0.3

    tui_assert_contains "rss"
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

@test "tui: item title with embedded whitespace is collapsed to single spaces" {
    mkdir -p "$TEST_DIR/www"

    cat > "$TEST_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Whitespace Test Feed</title>
    <link>http://example.com</link>
    <description>A test feed</description>
    <item>
      <title>Word1   Word2	Word3</title>
      <link>http://example.com/article1</link>
      <description>desc</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="WS Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "WS Source" 5

    tui_send_keys "Tab"
    sleep 0.3

    # The title should appear with single spaces between words, not multiple.
    tui_assert_contains "Word1 Word2 Word3"
    tui_assert_not_contains "Word1   Word2"
}

@test "tui: item title with embedded newline is collapsed to a single space" {
    mkdir -p "$TEST_DIR/www"

    # The title contains a literal newline between the two words.
    cat > "$TEST_DIR/www/feed.xml" <<'RSSEOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Newline Test Feed</title>
    <link>http://example.com</link>
    <description>A test feed</description>
    <item>
      <title>LineOne
LineTwo</title>
      <link>http://example.com/article1</link>
      <description>desc</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" -name="NL Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    tui_start
    tui_wait_for "NL Source" 5

    tui_send_keys "Tab"
    sleep 0.3

    # The newline should be collapsed to a single space.
    tui_assert_contains "LineOne LineTwo"
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

    # Name is kept short (<=14 chars) so it does not trigger truncation alongside
    # the date suffix at 80 columns; this test validates scroll behaviour only.
    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Scroll Source"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    # Use a short terminal so only a few items fit on screen at once.
    # Height 8 with borders gives ~4 inner rows; at 1 line per item only
    # 4 items fit, ensuring the 10-item list requires scrolling.
    tui_start 80 8
    tui_wait_for "Scroll Source" 5

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

@test "tui: item detail modal hard-wraps long token in description to fit terminal width" {
    mkdir -p "$TEST_DIR/www"

    # Modal content width on an 80-column terminal:
    #   modalWidth = floor(80*60/100) - 6 = 42
    #
    # We embed a space-free token (a long URL) in the description whose
    # length is 121 chars.  Without hard-breaking, the modal tries to render
    # a line 127 chars wide (121 + 6 border overhead).  In an 80-column
    # terminal lipgloss clips the line at col 80, placing "DescSentinel" at
    # col ~112 -- well past the clip point, not visible in the tmux capture.
    # With hard-breaking the token is split into chunks of 42 chars;
    # "DescSentinel" lands on the third chunk at col ~28 and is fully visible.
    local long_token
    long_token="http://example.com/$(printf 'a%.0s' $(seq 1 90))DescSentinel"

    cat > "$TEST_DIR/www/feed.xml" <<RSSEOF
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Desc Wrap Test Feed</title>
    <link>http://example.com</link>
    <description>Feed for description wrap testing</description>
    <item>
      <title>Desc Wrap Article</title>
      <link>http://example.com/article</link>
      <description>Reference: $long_token for details.</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    # Name is kept short (<=14 chars) so it does not trigger truncation
    # alongside the date suffix at 80 columns; this test validates modal
    # hard-wrapping behaviour only.
    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Desc Src"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    # 80-column terminal -- narrow enough to trigger the overflow bug.
    tui_start 80 24
    tui_wait_for "Desc Src" 5

    tui_send_keys "Tab"
    tui_wait_for "Desc Wrap Article" 5
    tui_send_keys "Enter"
    sleep 0.3

    tui_assert_contains "Title:"

    # Without hard-wrapping the long description token overflows the modal
    # past the terminal edge, clipping "DescSentinel".  With the fix it wraps
    # onto a continuation line within the modal width and is visible.
    tui_assert_contains "DescSentinel"
}

@test "tui: item detail modal hard-wraps long URL and title to fit terminal width" {
    mkdir -p "$TEST_DIR/www"

    # Modal content width on an 80-column terminal:
    #   modalWidth  = floor(80 * 60/100) - 6  = 48 - 6 = 42
    #   valueWidth  = 42 - 11 (label width)   = 31
    #
    # We build a URL whose tail "/WrapSentinel" falls beyond column 80 when
    # printed as a single unwrapped line, but lands on a new wrapped line --
    # and is therefore fully visible -- after the fix is applied.
    #
    # URL layout: 19-char prefix + 12 'a's fills the first 31-char chunk
    # exactly; 31 more 'a's fill the second chunk; "/WrapSentinel" is the
    # third chunk (13 chars).  Total URL = 75 chars.  Without wrapping the
    # line would start at approx column 30 and extend to column 104, placing
    # "WrapSentinel" well past the 80-column clip point.
    local long_url
    long_url="http://example.com/$(printf 'a%.0s' $(seq 1 43))/WrapSentinel"

    # Similarly, a title whose last word "TitleSentinel" ends up beyond
    # column 80 unless the title is word-wrapped.
    local long_title="Article with filler words to exceed the terminal width TitleSentinel"

    cat > "$TEST_DIR/www/feed.xml" <<RSSEOF
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Wrap Test Feed</title>
    <link>http://example.com</link>
    <description>Feed for wrap testing</description>
    <item>
      <title>$long_title</title>
      <link>$long_url</link>
      <description>Short summary.</description>
    </item>
  </channel>
</rss>
RSSEOF

    start_mock_server "$TEST_DIR/www"

    # Name is kept short (<=14 chars) so it does not trigger truncation
    # alongside the date suffix at 80 columns; this test validates modal
    # hard-wrapping behaviour only.
    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Wrap Src"
    src_id=$(extract_uuid "$output")
    newsfed sync "$src_id" >/dev/null 2>&1

    stop_mock_server

    # 80-column terminal -- narrow enough to trigger the overflow bug.
    tui_start 80 24
    tui_wait_for "Wrap Src" 5

    tui_send_keys "Tab"
    tui_wait_for "Article with" 5
    tui_send_keys "Enter"
    sleep 0.3

    tui_assert_contains "Title:"

    # Without wrapping, both sentinels are clipped past column 80 and are not
    # visible in the tmux capture.  With wrapping they land on continuation
    # lines that fit within the terminal width and are therefore visible.
    tui_assert_contains "WrapSentinel"
    tui_assert_contains "TitleSentinel"
}

# ---------------------------------------------------------------------------
# Add source modal (spec-9 section 10)
# ---------------------------------------------------------------------------

@test "tui: mode line shows [A]dd source when source frame is focused" {
    tui_start
    tui_wait_for "No sources." 5

    tui_assert_contains "[A]dd source"
}

@test "tui: mode line hides [A]dd source when items frame is focused" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "Tab"
    sleep 0.2

    tui_assert_not_contains "[A]dd source"
}

@test "tui: a opens add source modal" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "a"
    tui_wait_for "Add Source" 3

    tui_assert_contains "Add Source"
    tui_assert_contains "Name:"
    tui_assert_contains "URL:"
}

@test "tui: add source modal escape cancels without creating source" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "a"
    tui_wait_for "Add Source" 3

    tui_send_keys "Escape"
    sleep 0.2

    tui_assert_not_contains "Add Source"
    tui_assert_contains "No sources."
}

@test "tui: add source modal creates source and shows it in the list" {
    local www_dir="$TEST_DIR/www-tui-add"
    create_rss_feed "$www_dir/feed.xml" "TUI Test Feed"
    start_mock_server "$www_dir"
    local feed_url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml"

    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "a"
    tui_wait_for "Add Source" 3

    # Leave Name blank -- autodiscovery will use the feed title.
    tui_send_keys "Tab"
    tui_type "$feed_url"
    tui_send_keys "Enter"

    # Wait for autodiscovery and source creation.
    tui_wait_for "TUI Test Feed" 10

    tui_assert_contains "TUI Test Feed"
    tui_assert_not_contains "Add Source"

    stop_mock_server
}

@test "tui: a has no effect when items frame is focused" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "Tab"   # move focus to items frame
    sleep 0.2
    tui_send_keys "a"
    sleep 0.3

    tui_assert_not_contains "Add Source"
}

@test "tui: add source modal keeps modal open when autodiscovery fails" {
    local www_dir="$TEST_DIR/www-tui-no-feed"
    mkdir -p "$www_dir"
    # Serve plain HTML with no feed links and no feed at common probe paths.
    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html><html><head><title>No Feed</title></head><body></body></html>
EOF
    start_mock_server "$www_dir"
    local page_url="http://127.0.0.1:$MOCK_SERVER_PORT/"

    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "a"
    tui_wait_for "Add Source" 3

    tui_send_keys "Tab"
    tui_type "$page_url"
    tui_send_keys "Enter"

    # Wait for the discovery attempt to finish and the error message to appear.
    tui_wait_for "No feed found" 10

    # Modal should remain open so the user can correct the URL.
    tui_assert_contains "Add Source"

    stop_mock_server
}

@test "tui: add source modal keeps modal open when fields are empty" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "a"
    tui_wait_for "Add Source" 3

    # Press Enter without filling in any fields.
    tui_send_keys "Enter"
    sleep 0.3

    # Modal should remain open.
    tui_assert_contains "Add Source"
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

@test "tui: r key refreshes source and updates Last updated date" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Refresh Test Feed" 1
    start_mock_server "$TEST_DIR/www"

    run newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Refresh Test Source"

    tui_start
    tui_wait_for "Refresh Test Source" 5

    # Before any fetch, the source should display "(never)".
    tui_assert_contains "(never)"

    # Press r to trigger a refresh of the selected source.
    tui_send_keys "r"

    # Wait for the fetch to complete -- the mode line shows "Fetched: N new item(s)".
    tui_wait_for "Fetched:" 10

    stop_mock_server

    # After a successful fetch the date should show "(today)".
    tui_wait_for "(today)" 5
}

# ---------------------------------------------------------------------------
# Refresh All (Spec 11)
# ---------------------------------------------------------------------------

@test "tui: R key with no enabled sources shows No enabled sources" {
    tui_start
    tui_wait_for "No sources." 5

    tui_send_keys "R"
    sleep 0.3

    tui_assert_contains "No enabled sources"
}

@test "tui: R key opens Refresh All modal with title in border" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Refresh All Feed" 2
    start_mock_server "$TEST_DIR/www"

    newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Refresh All Source"

    tui_start
    tui_wait_for "Refresh All Source" 5

    tui_send_keys "R"

    tui_wait_for "Refresh All Feeds" 5
    tui_assert_contains "Refresh All Feeds"
    tui_assert_contains "Refresh All Source"

    tui_wait_for "Done:" 15
    stop_mock_server
}

@test "tui: Refresh All completes and Esc dismisses with summary" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Dismiss Test Feed" 1
    start_mock_server "$TEST_DIR/www"

    newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Dismiss Test Source"

    tui_start
    tui_wait_for "Dismiss Test Source" 5

    tui_send_keys "R"
    tui_wait_for "Done:" 15

    stop_mock_server

    # Esc should dismiss the modal and show a summary in the mode line.
    tui_send_keys "Escape"
    sleep 0.3

    tui_assert_contains "Refreshed all:"
    tui_assert_not_contains "Refresh All Feeds"
}

@test "tui: Refresh All modal shows done indicator after fetch" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Done Indicator Feed" 3
    start_mock_server "$TEST_DIR/www"

    newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Done Indicator Source"

    tui_start
    tui_wait_for "Done Indicator Source" 5

    tui_send_keys "R"
    tui_wait_for "Done:" 15

    stop_mock_server

    # The [✓] indicator should be visible for the completed source.
    tui_assert_contains "[✓]"
}


@test "tui: Refresh All dismissal shows new-item counts instead of dates" {
    mkdir -p "$TEST_DIR/www"
    create_rss_feed "$TEST_DIR/www/feed.xml" "Count Feed" 5
    start_mock_server "$TEST_DIR/www"

    newsfed sources add -type=rss \
        -url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" \
        -name="Count Source"

    tui_start
    tui_wait_for "Count Source" 5

    # Before refresh, the source shows (never) since it hasn't been fetched.
    tui_assert_contains "(never)"

    tui_send_keys "R"
    tui_wait_for "Done:" 15

    stop_mock_server

    # Dismiss the modal.
    tui_send_keys "Escape"
    sleep 0.3

    # The source had 5 new items -- should show (5) instead of a date.
    tui_assert_contains "(5)"

    # The old (never) label should be gone.
    tui_assert_not_contains "(never)"

    # Counts persist across key presses -- navigating should not clear them.
    tui_send_keys "Tab"
    sleep 0.3

    tui_assert_contains "(5)"
}

@test "tui: mode line shows R refresh all hint" {
    tui_start
    tui_wait_for "No sources." 5

    tui_assert_contains "[R]efresh all"
}

@test "tui: newsfed with no arguments launches the TUI" {
    TUI_SESSION="newsfed-tui-$$-$RANDOM"
    tmux new-session -d -s "$TUI_SESSION" -x 120 -y 30 \
        "NEWSFED_METADATA_DSN=$NEWSFED_METADATA_DSN NEWSFED_FEED_DSN=$NEWSFED_FEED_DSN $TEST_DIR/newsfed"
    tui_wait_for "No sources." 5
    tui_assert_contains "No sources."
}

# ---------------------------------------------------------------------------
# Pin / Unpin (Spec 9 section 11)
# ---------------------------------------------------------------------------

@test "tui: mode line shows [P]in when items frame is focused" {
    run newsfed sources add -type=rss -url=https://pin-modeline.example.com/feed -name="Pin Modeline Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    local item_id="aaaaaaaa-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${item_id}.json" <<EOF
{
  "id": "$item_id",
  "title": "Modeline Test Article",
  "summary": "Summary",
  "url": "https://example.com/modeline-test",
  "authors": [],
  "published_at": "2024-06-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Modeline Feed" 5

    # Source frame focused by default -- [P]in should not appear.
    tui_assert_not_contains "[P]in"

    # Tab to items frame -- [P]in should appear.
    tui_send_keys "Tab"
    tui_wait_for "Modeline Test Article" 5
    tui_assert_contains "[P]in"
}

@test "tui: P shows pin indicator on item" {
    run newsfed sources add -type=rss -url=https://pin-show.example.com/feed -name="Pin Show Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    local item_id="bbbbbbbb-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${item_id}.json" <<EOF
{
  "id": "$item_id",
  "title": "Pin Show Article",
  "summary": "Summary",
  "url": "https://example.com/pin-show",
  "authors": [],
  "published_at": "2024-06-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Show Feed" 5

    tui_send_keys "Tab"
    tui_wait_for "Pin Show Article" 5

    # Before pinning, no indicator should appear.
    tui_assert_not_contains "[📌]"

    # Press P to pin.
    tui_send_keys "P"
    tui_wait_for "[📌]" 3

    tui_assert_contains "[📌]"
}

@test "tui: P removes pin indicator when item is already pinned" {
    run newsfed sources add -type=rss -url=https://pin-unpin.example.com/feed -name="Pin Unpin Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    local item_id="cccccccc-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${item_id}.json" <<EOF
{
  "id": "$item_id",
  "title": "Unpin Test Article",
  "summary": "Summary",
  "url": "https://example.com/unpin-test",
  "authors": [],
  "published_at": "2024-06-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "pinned_at": "2024-06-02T00:00:00Z",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Unpin Feed" 5

    tui_send_keys "Tab"
    tui_wait_for "[📌]" 5

    # Item starts pinned -- indicator is visible.
    tui_assert_contains "[📌]"

    # Press P to unpin.
    tui_send_keys "P"
    sleep 0.5

    tui_assert_not_contains "[📌]"
}

@test "tui: pinned items appear before unpinned items" {
    run newsfed sources add -type=rss -url=https://pin-order.example.com/feed -name="Pin Order Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    # Older item (published first) will be pinned.
    local older_id="dddddddd-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${older_id}.json" <<EOF
{
  "id": "$older_id",
  "title": "Older Article",
  "summary": "Summary",
  "url": "https://example.com/older",
  "authors": [],
  "published_at": "2024-01-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "pinned_at": "2024-06-01T00:00:00Z",
  "source_id": "$src_id"
}
EOF

    # Newer item (published second) is unpinned.
    local newer_id="eeeeeeee-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${newer_id}.json" <<EOF
{
  "id": "$newer_id",
  "title": "Newer Article",
  "summary": "Summary",
  "url": "https://example.com/newer",
  "authors": [],
  "published_at": "2024-07-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Order Feed" 5

    tui_send_keys "Tab"
    tui_wait_for "Older Article" 5

    local screen
    screen=$(tui_capture)

    # Both items must appear.
    echo "$screen" | grep -qF "Older Article"
    echo "$screen" | grep -qF "Newer Article"

    # Pinned older item must appear above unpinned newer item.
    local pos_older pos_newer
    pos_older=$(echo "$screen" | grep -n "Older Article" | head -1 | cut -d: -f1)
    pos_newer=$(echo "$screen" | grep -n "Newer Article" | head -1 | cut -d: -f1)
    [ "$pos_older" -lt "$pos_newer" ]
}

@test "tui: P has no effect when source frame is focused" {
    run newsfed sources add -type=rss -url=https://pin-noop-src.example.com/feed -name="Pin Noop Src Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    local item_id="ffffffff-0000-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${item_id}.json" <<EOF
{
  "id": "$item_id",
  "title": "Noop Src Article",
  "summary": "Summary",
  "url": "https://example.com/noop-src",
  "authors": [],
  "published_at": "2024-06-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Noop Src Feed" 5

    # Source frame is focused by default. Press P -- should have no effect.
    tui_send_keys "P"
    sleep 0.3

    # Tab to items frame and verify no pin indicator appeared.
    tui_send_keys "Tab"
    tui_wait_for "Noop Src Article" 5

    tui_assert_not_contains "[📌]"
}

@test "tui: P has no effect when a modal is open" {
    run newsfed sources add -type=rss -url=https://pin-noop-modal.example.com/feed -name="Pin Noop Modal Feed"
    local src_id
    src_id=$(extract_uuid "$output")

    local item_id="11111111-1111-0000-0000-000000000001"
    cat > "$NEWSFED_FEED_DSN/${item_id}.json" <<EOF
{
  "id": "$item_id",
  "title": "Noop Modal Article",
  "summary": "Summary",
  "url": "https://example.com/noop-modal",
  "authors": [],
  "published_at": "2024-06-01T00:00:00Z",
  "discovered_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "source_id": "$src_id"
}
EOF

    tui_start
    tui_wait_for "Pin Noop Modal Feed" 5

    tui_send_keys "Tab"
    tui_wait_for "Noop Modal Article" 5

    # Open the item detail modal.
    tui_send_keys "Enter"
    tui_wait_for "Title:" 3

    # Press P while modal is open -- should have no effect.
    tui_send_keys "P"
    sleep 0.3

    # Close modal.
    tui_send_keys "Escape"
    sleep 0.3

    # No pin indicator should appear.
    tui_assert_not_contains "[📌]"
}
