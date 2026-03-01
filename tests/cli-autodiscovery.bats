#!/usr/bin/env bats
# Test CLI: feed autodiscovery (Spec 10 sections 3.2, 3.3, 5.2, 6.3)

load test_helper

setup_file() {
    setup_test_env
    build_newsfed "$TEST_DIR"
    mkdir -p "$NEWSFED_FEED_DSN"
    run newsfed init
}

teardown_file() {
    cleanup_test_env
}

# ---------------------------------------------------------------------------
# Spec 10 section 3.1 -- direct feed detection
# ---------------------------------------------------------------------------

@test "autodiscovery: direct feed URL recognised without discovery notice (spec 3.1, 6.3)" {
    local www_dir="$TEST_DIR/www-direct"
    create_rss_feed "$www_dir/feed.xml" "Direct Feed"
    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml"
    stop_mock_server

    assert_success
    assert_output_not_contains "Discovered"
    assert_output_contains "Created source: Direct Feed (rss)"
}

@test "autodiscovery: --name overrides feed title on direct feed (spec 5.1)" {
    local www_dir="$TEST_DIR/www-direct-name"
    create_rss_feed "$www_dir/feed.xml" "Feed Title From Document"
    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/feed.xml" --name="My Custom Name"
    stop_mock_server

    assert_success
    assert_output_contains "My Custom Name"
    assert_output_not_contains "Feed Title From Document"
}

# ---------------------------------------------------------------------------
# Spec 10 section 3.2 -- HTML link tag detection
# ---------------------------------------------------------------------------

@test "autodiscovery: discovers RSS feed via HTML link tag (spec 3.2)" {
    local www_dir="$TEST_DIR/www-link-rss"
    mkdir -p "$www_dir"
    create_rss_feed "$www_dir/feed.xml" "Link Tag RSS Feed"

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html>
<head>
  <title>Test Site</title>
  <link rel="alternate" type="application/rss+xml" href="/feed.xml" title="Link Tag RSS Feed">
</head>
<body><p>Hello</p></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_success
    assert_output_contains "Discovered RSS feed"
    assert_output_contains "Created source: Link Tag RSS Feed (rss)"
}

@test "autodiscovery: discovers Atom feed via HTML link tag (spec 3.2)" {
    local www_dir="$TEST_DIR/www-link-atom"
    mkdir -p "$www_dir"

    cat > "$www_dir/atom.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Link Tag Atom Feed</title>
  <link href="http://example.com"/>
  <id>http://example.com</id>
</feed>
EOF

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html>
<head>
  <title>Test Site</title>
  <link rel="alternate" type="application/atom+xml" href="/atom.xml" title="Link Tag Atom Feed">
</head>
<body><p>Hello</p></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_success
    assert_output_contains "Discovered Atom feed"
    assert_output_contains "Created source: Link Tag Atom Feed (atom)"
}

@test "autodiscovery: uses feed title as source name when --name omitted (spec 5.1, 3.2)" {
    local www_dir="$TEST_DIR/www-link-title"
    mkdir -p "$www_dir"
    create_rss_feed "$www_dir/feed.xml" "Autodiscovered Title"

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html>
<head>
  <link rel="alternate" type="application/rss+xml" href="/feed.xml">
</head>
<body></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_success
    assert_output_contains "Autodiscovered Title"
}

# ---------------------------------------------------------------------------
# Spec 10 section 3.3 -- common path probing
# ---------------------------------------------------------------------------

@test "autodiscovery: discovers feed at root /index.xml when page has no link tags (spec 3.3)" {
    local www_dir="$TEST_DIR/www-probe-root"
    mkdir -p "$www_dir"
    create_rss_feed "$www_dir/index.xml" "Probe Feed"

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html><head><title>No Link Tags Here</title></head>
<body><p>Hello</p></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_success
    assert_output_contains "Discovered RSS feed"
    assert_output_contains "Created source: Probe Feed (rss)"
}

@test "autodiscovery: discovers feed at /feed.xml when page has no link tags (spec 3.3)" {
    local www_dir="$TEST_DIR/www-probe-feed"
    mkdir -p "$www_dir"
    create_rss_feed "$www_dir/feed.xml" "Feed XML Feed"

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html><head><title>No Link Tags</title></head>
<body></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_success
    assert_output_contains "Discovered RSS feed"
    assert_output_contains "Created source: Feed XML Feed (rss)"
}

# ---------------------------------------------------------------------------
# Spec 10 section 5.2 / 6.4 -- failure output
# ---------------------------------------------------------------------------

@test "autodiscovery: error lists tried URLs when no feed found (spec 5.2, 6.4)" {
    local www_dir="$TEST_DIR/www-no-feed"
    mkdir -p "$www_dir"

    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html><head><title>No Feed Here</title></head>
<body><p>No feeds on this site.</p></body>
</html>
EOF

    start_mock_server "$www_dir"
    local base_url="http://127.0.0.1:$MOCK_SERVER_PORT"

    run newsfed sources add --url="$base_url/"
    stop_mock_server

    assert_failure
    assert_output_contains "no feed found"
    assert_output_contains "Tried:"
    assert_output_contains "$base_url"
    assert_output_contains "To add this URL as a website source"
}

@test "autodiscovery: error output contains no feed links in page note (spec 6.4)" {
    local www_dir="$TEST_DIR/www-no-links"
    mkdir -p "$www_dir"

    # Serve a plain HTML page with no qualifying link tags at every path.
    cat > "$www_dir/index.html" <<'EOF'
<!DOCTYPE html>
<html><head><title>Plain Page</title></head>
<body></body>
</html>
EOF

    start_mock_server "$www_dir"

    run newsfed sources add --url="http://127.0.0.1:$MOCK_SERVER_PORT/"
    stop_mock_server

    assert_failure
    assert_output_contains "no feed links in page"
}
