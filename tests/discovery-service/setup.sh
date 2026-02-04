#!/bin/bash
# Setup script for RFC 7 discovery service black box tests
# Creates test metadata database and test RSS feed files

set -e

METADATA_DB="test-metadata.db"
FEED_DIR="test-feed"
TEST_FEEDS_DIR="test-fixtures"

echo "Setting up test environment for discovery service..."
echo ""

# Clean up any existing test data
rm -rf "$METADATA_DB" "$FEED_DIR" "$TEST_FEEDS_DIR"

# Create directories
mkdir -p "$FEED_DIR"
mkdir -p "$TEST_FEEDS_DIR"

echo "Creating test RSS feed fixtures..."

# Create a valid RSS feed file
cat > "$TEST_FEEDS_DIR/valid-feed.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>http://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>Test Article 1</title>
      <link>http://example.com/article1</link>
      <description>This is the first test article</description>
      <pubDate>Mon, 01 Jan 2026 10:00:00 GMT</pubDate>
      <author>Alice</author>
    </item>
    <item>
      <title>Test Article 2</title>
      <link>http://example.com/article2</link>
      <description>This is the second test article</description>
      <pubDate>Tue, 02 Jan 2026 10:00:00 GMT</pubDate>
      <author>Bob</author>
    </item>
  </channel>
</rss>
EOF

# Create another RSS feed for deduplication testing
cat > "$TEST_FEEDS_DIR/duplicate-feed.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Duplicate Feed</title>
    <link>http://example.com</link>
    <description>Feed with duplicate items</description>
    <item>
      <title>Test Article 1</title>
      <link>http://example.com/article1</link>
      <description>Same article from different feed</description>
      <pubDate>Mon, 01 Jan 2026 10:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Test Article 3</title>
      <link>http://example.com/article3</link>
      <description>This is a unique article</description>
      <pubDate>Wed, 03 Jan 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>
EOF

# Create an invalid RSS feed
cat > "$TEST_FEEDS_DIR/invalid-feed.xml" <<'EOF'
This is not valid XML
EOF

echo "✓ Created RSS feed fixtures in $TEST_FEEDS_DIR"
echo ""

# Start a simple HTTP server to serve test feeds
echo "Starting HTTP server for test feeds..."
python3 -m http.server 9876 --directory "$TEST_FEEDS_DIR" > /dev/null 2>&1 &
HTTP_SERVER_PID=$!
echo $HTTP_SERVER_PID > .http-server-pid

# Wait for server to start
sleep 1

echo "✓ HTTP server started on port 9876 (PID: $HTTP_SERVER_PID)"
echo ""

# Initialize metadata database and create test sources
echo "Initializing metadata database..."

# Build the discovery service to the test directory
echo "Building discovery service..."
(cd ../.. && go build -o tests/discovery-service/newsfed-discover ./cmd/newsfed-discover)
echo "✓ Built discovery service to test directory"

# Create a Go script to initialize the database with test sources
cat > init-db.go <<'GOSCRIPT'
package main

import (
	"log"
	"time"
	"github.com/pevans/newsfed"
)

func main() {
	// Create metadata store
	store, err := newsfed.NewMetadataStore("test-metadata.db")
	if err != nil {
		log.Fatalf("Failed to create metadata store: %v", err)
	}
	defer store.Close()

	now := time.Now()

	// Create source for valid RSS feed (served via HTTP)
	_, err = store.CreateSource(
		"rss",
		"http://localhost:9876/valid-feed.xml",
		"Valid Test Feed",
		nil,
		&now,
	)
	if err != nil {
		log.Fatalf("Failed to create valid feed source: %v", err)
	}

	// Create source for invalid RSS feed (for error testing)
	_, err = store.CreateSource(
		"rss",
		"http://localhost:9876/invalid-feed.xml",
		"Invalid Test Feed",
		nil,
		&now,
	)
	if err != nil {
		log.Fatalf("Failed to create invalid feed source: %v", err)
	}

	// Create source for non-existent feed (for error testing)
	_, err = store.CreateSource(
		"rss",
		"http://localhost:9876/nonexistent.xml",
		"Nonexistent Feed",
		nil,
		&now,
	)
	if err != nil {
		log.Fatalf("Failed to create nonexistent feed source: %v", err)
	}

	log.Println("✓ Created 3 test sources in database")
}
GOSCRIPT

# Run the initialization script
go run init-db.go
rm init-db.go

echo "✓ Initialized metadata database with test sources"
echo ""
echo "Test environment ready!"
echo ""
echo "Test data:"
echo "  - Metadata DB: $METADATA_DB"
echo "  - Feed storage: $FEED_DIR/"
echo "  - Test fixtures: $TEST_FEEDS_DIR/"
echo "  - 3 test sources configured (1 valid, 1 invalid, 1 nonexistent)"
