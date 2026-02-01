#!/bin/bash
# Setup script for RFC 4 black box tests
# Creates test data in the .news directory

set -e

BASE_URL="http://localhost:8080/api/v1"
NEWS_DIR=".news"

echo "Setting up test data..."

# Create .news directory if it doesn't exist
mkdir -p "$NEWS_DIR"

# Create test news items with known UUIDs and data
cat > "$NEWS_DIR/550e8400-e29b-41d4-a716-446655440000.json" <<'EOF'
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "Test Article 1",
  "summary": "This is a test article for API testing",
  "url": "https://example.com/article1",
  "publisher": "Test Publisher",
  "authors": ["Alice"],
  "published_at": "2026-01-01T10:00:00Z",
  "discovered_at": "2026-01-15T12:00:00Z",
  "pinned_at": null
}
EOF

cat > "$NEWS_DIR/550e8400-e29b-41d4-a716-446655440001.json" <<'EOF'
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "title": "Test Article 2",
  "summary": "Another test article",
  "url": "https://example.com/article2",
  "publisher": "Test Publisher",
  "authors": ["Bob", "Carol"],
  "published_at": "2026-01-02T10:00:00Z",
  "discovered_at": "2026-01-16T12:00:00Z",
  "pinned_at": "2026-01-20T14:00:00Z"
}
EOF

cat > "$NEWS_DIR/550e8400-e29b-41d4-a716-446655440002.json" <<'EOF'
{
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "title": "Test Article 3",
  "summary": "Third test article",
  "url": "https://example.com/article3",
  "publisher": "Another Publisher",
  "authors": ["Alice"],
  "published_at": "2026-01-03T10:00:00Z",
  "discovered_at": "2026-01-17T12:00:00Z",
  "pinned_at": null
}
EOF

echo "✓ Created 3 test items in $NEWS_DIR"
echo ""
echo "Test data ready!"
echo "Known UUIDs:"
echo "  - 550e8400-e29b-41d4-a716-446655440000 (unpinned)"
echo "  - 550e8400-e29b-41d4-a716-446655440001 (pinned)"
echo "  - 550e8400-e29b-41d4-a716-446655440002 (unpinned)"
