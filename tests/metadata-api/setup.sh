#!/bin/bash
# Setup script for RFC 6 black box tests
# Clears test data from the metadata database via API

set -e

BASE_URL="http://localhost:8081/api/v1/meta"

echo "Setting up metadata API test data..."

# Delete all existing sources via API
echo "Clearing existing sources..."
sources=$(curl -s "$BASE_URL/sources" | jq -r '.sources[]?.source_id // empty')
count=0
for source_id in $sources; do
    curl -s -X DELETE "$BASE_URL/sources/$source_id" > /dev/null
    count=$((count + 1))
done

echo "✓ Cleared $count existing sources"
echo ""
echo "Test data ready!"
