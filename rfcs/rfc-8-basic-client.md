---
Request For Comments: 8
Title: Basic News Feed Client
Drafted At: 2026-02-04
Authors:
  - Peter Evans
---

# 1. Overview

A news feed client provides users with an interface for reading news and
managing sources. This RFC describes the essential functionality, architecture
options, and implementation patterns for building a basic client.

There are two client architecture approaches:

1. **CLI client** -- Direct access to storage layers (metadata and news feed
   stores) using Go packages from RFC 1 and RFC 5
2. **Web client** -- Remote access via HTTP APIs (RFC 4 and RFC 6)

The client serves two primary purposes:

1. **News consumption** -- Browse, read, filter, and pin news items
2. **Source administration** -- Configure and manage RSS/Atom feeds and
   website scrapers

# 2. Client Architecture Options

## 2.1. CLI Client

A command-line interface client provides direct access to newsfed
functionality by using the storage layer Go packages directly (metadata store
from RFC 5, news feed from RFC 1). The CLI client runs on the same machine as
the data stores and does not require the HTTP API server to be running.

**Advantages:**
- Lightweight and fast -- no HTTP overhead
- Scriptable and automatable
- No browser or GUI dependencies
- Easy to integrate with shell workflows
- Direct storage access -- no API server required
- Simple deployment -- single binary with storage configuration

**Disadvantages:**
- Limited visual formatting
- Less intuitive for non-technical users
- Cannot display rich media without external tools
- Must run on same machine as data stores (or have network access to database)

## 2.2. Web Client

A web-based client provides a graphical interface through a browser. Unlike
the CLI client, the web client must use the HTTP APIs (RFC 4 and RFC 6) since
it runs in a browser and cannot directly access the storage layer.

**Advantages:**
- Rich, visual interface with formatting and layout
- Accessible from any device with a browser
- Can display images, videos, and rich content
- Familiar interaction patterns for most users
- Can run remotely (different machine than data stores)

**Disadvantages:**
- Requires web development (HTML/CSS/JavaScript)
- More complex to build and maintain
- Requires HTTP API server to be running (RFC 4, RFC 6)
- Additional network latency

## 2.3. Shared Command Library

For the CLI client, common functionality can be extracted into a shared Go
package that handles:

- Storage initialization (metadata store and news feed)
- Common operations (list items, manage sources)
- Output formatting (table, JSON, compact)
- Configuration loading

This library is used internally by the CLI commands but is not a public API
client library.

**Advantages:**
- Code reuse across CLI commands
- Consistent behavior and error handling
- Easier testing
- Clear separation of concerns

**Disadvantages:**
- Additional abstraction layer
- More code to maintain

## 2.4. Recommended Approach

The recommended approach is to build a **CLI client first** as a minimal
viable product. The CLI client directly accesses the storage layers using the
Go packages from RFC 1 (news feed storage) and RFC 5 (metadata storage). This
approach is simpler than building API-based clients because:

- No HTTP API server required for basic usage
- Single binary deployment
- Lower latency (direct storage access)
- Fewer moving parts

A web client can be added later, which would require the HTTP API servers
(RFC 4 and RFC 6) to be running.

# 3. Core Functionality

## 3.1. News Reading

### 3.1.1. List News Items

The client should allow users to list news items with various filters:

- Filter by pinned status (pinned only, unpinned only, or all)
- Filter by publisher or author
- Filter by date range (items discovered within a time window)
- Sort by published date, discovered date, or pinned date
- Paginate through large result sets

**Example CLI commands:**

```bash
# List recent items
newsfed list

# List pinned items
newsfed list --pinned

# List items from a specific publisher
newsfed list --publisher="TechCrunch"

# List items discovered in the last 24 hours
newsfed list --since=24h

# List with custom pagination
newsfed list --limit=10 --offset=20
```

### 3.1.2. View Individual Items

Users should be able to view the full details of a specific news item:

- Display all metadata (title, summary, URL, authors, dates)
- Show pinned status
- Provide easy access to the original URL

**Example CLI command:**

```bash
# View item by ID
newsfed show 550e8400-e29b-41d4-a716-446655440000
```

### 3.1.3. Pin and Unpin Items

Users should be able to pin items for later reference:

**Example CLI commands:**

```bash
# Pin an item
newsfed pin 550e8400-e29b-41d4-a716-446655440000

# Unpin an item
newsfed unpin 550e8400-e29b-41d4-a716-446655440000
```

### 3.1.4. Open Items in Browser

For CLI clients, provide a quick way to open the original URL in a browser:

**Example CLI command:**

```bash
# Open item URL in default browser
newsfed open 550e8400-e29b-41d4-a716-446655440000
```

## 3.2. Source Management

### 3.2.1. List Sources

Users should be able to view all configured sources:

- Display source ID, name, type, URL, and status
- Filter by type (RSS, Atom, website)
- Filter by enabled status
- Show health indicators (error counts, last fetch time)

**Example CLI commands:**

```bash
# List all sources
newsfed sources list

# List only RSS sources
newsfed sources list --type=rss

# List only enabled sources
newsfed sources list --enabled
```

### 3.2.2. View Source Details

Users should be able to inspect detailed source information:

- Full configuration including polling interval
- Operational metadata (last fetched, error count, last error)
- For website sources, show scraper configuration

**Example CLI command:**

```bash
# View source details
newsfed sources show 550e8400-e29b-41d4-a716-446655440000
```

### 3.2.3. Add Sources

Users should be able to add new sources:

**For RSS/Atom feeds:**

```bash
# Add RSS feed
newsfed sources add \
  --type=rss \
  --url="https://blog.example.com/feed.xml" \
  --name="Example Blog" \
  --interval=30m
```

**For websites with scraper configuration:**

```bash
# Add website source (with inline scraper config)
newsfed sources add \
  --type=website \
  --url="https://blog.example.com" \
  --name="Example Blog" \
  --config=scraper-config.json
```

The scraper configuration file follows RFC 3 format.

### 3.2.4. Update Sources

Users should be able to modify existing sources:

```bash
# Change polling interval
newsfed sources update 550e8400... --interval=1h

# Rename source
newsfed sources update 550e8400... --name="New Name"

# Update scraper config
newsfed sources update 550e8400... --config=new-config.json
```

### 3.2.5. Enable and Disable Sources

Users should be able to enable or disable sources:

```bash
# Disable a source
newsfed sources disable 550e8400...

# Enable a source
newsfed sources enable 550e8400...
```

### 3.2.6. Delete Sources

Users should be able to remove sources:

```bash
# Delete a source (with confirmation)
newsfed sources delete 550e8400...

# Force delete without confirmation
newsfed sources delete 550e8400... --force
```

## 3.3. Source Health Monitoring

### 3.3.1. Check Source Status

Users should be able to check the health of sources:

```bash
# Show sources with errors
newsfed sources status

# Show detailed error information
newsfed sources status --verbose
```

Output should highlight:
- Sources with recent fetch errors
- Sources that haven't been fetched recently
- Sources that have been auto-disabled

### 3.3.2. View Error History

For troubleshooting, users should see error details:

```bash
# View errors for a specific source
newsfed sources errors 550e8400...
```

# 4. Configuration

## 4.1. Storage Configuration

The CLI client needs to know how to connect to the metadata and news feed
storage. This configuration is identical to what the discovery service uses
(RFC 7, section 9.1.1).

**Configuration file** (`~/.newsfed/config.yaml`):

```yaml
storage:
  # Metadata storage (sources, configuration)
  metadata:
    type: "sqlite"  # sqlite, postgres, mysql
    dsn: "file:/Users/username/.newsfed/metadata.db"

  # News feed storage (news items)
  feed:
    type: "file"  # file, sqlite, postgres
    dsn: "file:/Users/username/.newsfed/feed"
```

**Environment variables:**

```bash
# Metadata Storage
NEWSFED_METADATA_TYPE="sqlite"
NEWSFED_METADATA_DSN="file:/Users/username/.newsfed/metadata.db"

# News Feed Storage
NEWSFED_FEED_TYPE="file"
NEWSFED_FEED_DSN="file:/Users/username/.newsfed/feed"
```

**DSN Examples:**

```bash
# SQLite (file-based)
NEWSFED_METADATA_DSN="file:/path/to/metadata.db"
NEWSFED_FEED_DSN="file:/path/to/feed.db"

# PostgreSQL
NEWSFED_METADATA_DSN="postgres://user:pass@localhost:5432/newsfed_metadata"
NEWSFED_FEED_DSN="postgres://user:pass@localhost:5432/newsfed_feed"

# MySQL
NEWSFED_METADATA_DSN="user:pass@tcp(localhost:3306)/newsfed_metadata"

# File-based feed storage
NEWSFED_FEED_DSN="file:/Users/username/.newsfed/feed"
```

**Configuration Precedence:**

1. Command-line flags (if provided)
2. Environment variables
3. Configuration file
4. Default values

## 4.2. Display Preferences

For CLI clients, users may want to customize output:

```yaml
display:
  # Date format for CLI output
  date_format: "2006-01-02 15:04:05"

  # Truncate long summaries
  summary_length: 200

  # Items per page for list commands
  page_size: 20

  # Color output (auto, always, never)
  color: auto
```

## 4.3. Default Browser

For opening URLs from CLI:

```yaml
browser:
  # Command to open URLs (defaults to system default)
  command: "open"  # macOS
  # command: "xdg-open"  # Linux
  # command: "start"  # Windows
```

# 5. Output Formatting

## 5.1. CLI Output Formats

The CLI client should support multiple output formats:

### 5.1.1. Table Format (Default)

Human-readable table display:

```
ID                                    TITLE                 PUBLISHER    PUBLISHED
550e8400-e29b-41d4-a716-446655440000  Example Article       TechCrunch   2026-02-01 10:00
550e8400-e29b-41d4-a716-446655440001  Another Article       Ars Technica 2026-02-01 09:30
```

### 5.1.2. JSON Format

Machine-readable JSON output for scripting:

```bash
newsfed list --format=json
```

```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Example Article",
      "publisher": "TechCrunch",
      "published_at": "2026-02-01T10:00:00Z"
    }
  ],
  "total": 42
}
```

### 5.1.3. Compact Format

Minimal output for quick scanning:

```bash
newsfed list --format=compact
```

```
550e8400... Example Article (TechCrunch)
550e8401... Another Article (Ars Technica)
```

## 5.2. Web Client Display

Web clients should provide:

- Card-based or list-based layouts for news items
- Visual indicators for pinned items
- Responsive design for mobile and desktop
- Search and filter controls
- Rich text rendering for summaries

# 6. Error Handling

## 6.1. Storage Errors

The CLI client should handle storage errors gracefully:

- **Connection errors** -- Clear message if database/storage is unreachable
- **File not found** -- For file-based storage, explain path issues
- **Permission errors** -- Explain file/directory permission problems
- **Database errors** -- Syntax errors, constraint violations
- **Not found errors** -- Friendly message for invalid IDs (item/source not
  found)
- **Corruption errors** -- Suggest backup restoration or repair

**Example error messages:**

```
Error: Could not connect to metadata storage
  Type: sqlite
  DSN: file:/Users/username/.newsfed/metadata.db
  Cause: unable to open database file: no such file or directory

Hint: Initialize storage with 'newsfed init' or check configuration
```

## 6.2. User Input Validation

Validate user input before accessing storage:

- Check UUID format for item/source IDs
- Validate URL format for source URLs
- Validate duration strings for polling intervals
- Check file existence for config files
- Validate scraper configuration JSON structure

## 6.3. Storage Initialization

The client should detect uninitialized storage and help users:

```bash
# Initialize storage (create databases/directories)
newsfed init

# Check storage health
newsfed doctor
```

## 6.4. Partial Failures

For batch operations (e.g., listing many items), handle partial failures:

- Display what succeeded before the failure
- Provide clear error message about what failed
- Allow retry or continuation
- For file-based storage, handle individual file read errors

# 7. Interactive Features

## 7.1. Interactive Mode (CLI)

For CLI clients, consider an interactive mode:

```bash
# Start interactive shell
newsfed interactive
```

Interactive mode provides:
- Persistent storage connections (avoids repeated initialization)
- Command history and completion
- Faster operations (reuses database connections)
- Better user experience for exploratory workflows

## 7.2. Fuzzy Search (CLI)

For CLI clients, provide fuzzy finding:

```bash
# Select an item interactively
newsfed list | fzf | newsfed show

# Or built-in selection
newsfed select
```

## 7.3. Watch Mode

Monitor for new items in real-time:

```bash
# Poll for new items every 30 seconds
newsfed watch --interval=30s
```

# 8. Security Considerations

## 8.1. File Permissions

For file-based storage, the CLI client should:

- Create storage directories with appropriate permissions (0700)
- Create database files with restricted permissions (0600)
- Warn if storage files have overly permissive permissions
- Never create world-readable storage files

## 8.2. Database Authentication

For database-based storage (PostgreSQL, MySQL):

- Support password authentication via DSN
- Support reading credentials from environment variables
- Warn about credentials in command history
- Consider using connection string files with restricted permissions

## 8.3. Multi-User Environments

In multi-user systems:

- Default to per-user storage locations (`~/.newsfed/`)
- Support system-wide storage with appropriate permissions
- Document shared database setup for teams

# 9. Implementation Considerations

## 9.1. Storage Initialization

The CLI client should handle storage initialization:

- **Lazy initialization** -- Initialize storage only when needed
- **Auto-migration** -- Automatically upgrade database schemas
- **Health checks** -- Verify storage is accessible on startup
- **Connection pooling** -- Reuse database connections for performance

**Initialization flow:**

1. Load configuration (file, environment, flags)
2. Validate configuration (check DSN format, types)
3. Initialize metadata store using factory pattern (RFC 5)
4. Initialize news feed using factory pattern (RFC 1)
5. Verify connectivity (test query or file access)
6. Handle initialization errors gracefully

## 9.2. Querying and Filtering

The CLI client uses the storage layer directly:

- **News items** -- Call `NewsFeed.List()` with filter criteria (RFC 1)
- **Sources** -- Call `MetadataStore.ListSources()` with filters (RFC 5)
- **Pagination** -- Implement limit/offset in queries
- **Sorting** -- Leverage storage layer sorting capabilities

For large result sets:
- Stream results instead of loading all into memory
- Display results progressively as they're fetched
- Provide progress indicators for slow queries

## 9.3. Transaction Handling

For operations that modify data:

- Use database transactions where supported
- Ensure atomic updates (e.g., pin/unpin operations)
- Roll back on errors
- Provide clear feedback on success/failure

## 9.4. Storage Backend Selection

The client should support multiple storage backends via factory pattern:

```go
// Metadata store factory (from RFC 5)
metadataStore, err := metadata.NewStore(metadataType, metadataDSN)

// News feed factory (from RFC 1)
newsFeed, err := newsfeed.NewFeed(feedType, feedDSN)
```

Initial implementation should support:
- Metadata: SQLite
- Feed: File-based or SQLite

Additional backends can be added later without changing client logic.

# 10. Example Workflows

## 10.1. Daily News Reading Workflow

```bash
# Check for new items today
newsfed list --since=24h

# Read interesting item
newsfed show 550e8400...

# Pin for later
newsfed pin 550e8400...

# Open in browser for full article
newsfed open 550e8400...

# Review pinned items later
newsfed list --pinned
```

## 10.2. Source Management Workflow

```bash
# Add a new RSS feed
newsfed sources add \
  --type=rss \
  --url="https://blog.rust-lang.org/feed.xml" \
  --name="Rust Blog"

# Check if it's working
newsfed sources status

# Adjust polling interval
newsfed sources update 550e8400... --interval=2h

# Check for items from new source
newsfed list --publisher="Rust Blog"
```

## 10.3. Troubleshooting Workflow

```bash
# Check source health
newsfed sources status

# View specific source errors
newsfed sources show 550e8400...

# Temporarily disable problematic source
newsfed sources disable 550e8400...

# Fix configuration (e.g., update scraper config)
newsfed sources update 550e8400... --config=fixed-config.json

# Re-enable
newsfed sources enable 550e8400...
```

# 11. Testing

## 11.1. Unit Tests

Test client functionality independently:

- Storage initialization and factory functions
- Command parsing and validation
- Output formatting (table, JSON, compact)
- Error handling and user messages
- Configuration loading

## 11.2. Integration Tests

Test against real or in-memory storage backends:

- Full workflows (list, show, pin, unpin)
- Source CRUD operations
- Error scenarios (storage errors, not found, validation failures)
- Pagination and filtering
- Concurrent access (if applicable)

Use in-memory SQLite (`:memory:`) for fast, isolated integration tests.

## 11.3. End-to-End Tests

Test complete user workflows with real storage:

- CLI command execution against test database
- Configuration loading from files and environment
- Multiple operations in sequence
- State persistence across commands
- Storage migration and initialization

# 12. Documentation

## 12.1. User Documentation

Provide comprehensive user documentation:

- **Installation guide** -- How to install and configure
- **Quick start** -- Basic usage examples
- **Command reference** -- All commands and flags
- **Configuration guide** -- All settings and options
- **Troubleshooting** -- Common problems and solutions

## 12.2. Developer Documentation

For those extending or contributing:

- **Architecture overview** -- How the client is structured
- **Storage layer** -- How to use NewsFeed and MetadataStore interfaces
- **Adding commands** -- How to add new CLI commands
- **Testing guide** -- How to run and write tests
- **Factory patterns** -- How storage backends are initialized

# 13. Future Enhancements

## 13.1. Full-Text Search

Search across all news items:

```bash
# Search in titles and summaries
newsfed search "rust programming"
```

Requires backend support for full-text indexing.

## 13.2. Feed Import/Export

Import and export source configurations:

```bash
# Export all sources to OPML
newsfed sources export --format=opml > sources.opml

# Import from OPML
newsfed sources import sources.opml
```

## 13.3. Notification System

Notify users of new items:

- Desktop notifications (CLI client)
- Push notifications (web client)
- Email digests
- Webhook integrations

## 13.4. Read/Unread Tracking

Track which items the user has read:

- Mark items as read when viewed
- Filter to show only unread items
- Read counts per source

Requires backend support for read state storage.

## 13.5. Collections and Tags

Organize items into collections:

```bash
# Create collection
newsfed collections create "Rust Articles"

# Add item to collection
newsfed collections add "Rust Articles" 550e8400...

# View collection
newsfed collections show "Rust Articles"
```

Requires backend API for collection management.

## 13.6. Sync Across Devices

Synchronize state across multiple devices:

- Pinned items
- Read status
- Collections
- Preferences

Requires backend support and authentication.

## 13.7. Discovery Service Status

Show discovery service health:

```bash
# Check if discovery service is running
newsfed status

# View recent discovery activity
newsfed activity

# Force fetch from a specific source
newsfed sources fetch 550e8400...
```

Requires discovery service to expose status API.

# 14. Implementation Phases

## 14.1. Phase 1: Core CLI Client

Minimal viable product:

- List news items with basic filters
- View individual items
- Pin and unpin items
- Add/remove RSS/Atom sources
- List sources

## 14.2. Phase 2: Enhanced Source Management

Full source administration:

- Website scraper support
- Enable/disable sources
- Update source configuration
- Source health monitoring
- Error reporting

## 14.3. Phase 3: Improved UX

Better user experience:

- Interactive mode
- Multiple output formats (JSON, table, compact)
- Configuration file support
- Better error messages
- Command completion

## 14.4. Phase 4: Web Client

Graphical interface:

- Web-based news reader
- Visual source management
- Responsive design
- Rich media display

## 14.5. Phase 5: Advanced Features

Optional enhancements:

- Full-text search
- Collections and tagging
- Read/unread tracking
- Notifications
- Feed import/export (OPML)

# 15. Relationship to Other RFCs

## 15.1. News Feed Storage (RFC 1)

The CLI client directly uses the `NewsFeed` interface from RFC 1:

- `List(filters)` -- Retrieve news items with filtering and sorting
- `Get(id)` -- Get individual item by ID
- `Add(item)` -- Add new items (not typically used by client)
- `Pin(id)` -- Pin an item for later reference
- `Unpin(id)` -- Unpin an item

The client uses the factory function to initialize the appropriate storage
backend (file-based, SQLite, etc.).

## 15.2. Metadata Storage (RFC 5)

The CLI client directly uses the `MetadataStore` interface from RFC 5:

- `ListSources(filters)` -- Retrieve source list with filtering
- `GetSource(id)` -- Get source details including operational metadata
- `CreateSource(source)` -- Add new RSS/Atom feed or website scraper
- `UpdateSource(id, updates)` -- Modify source configuration
- `DeleteSource(id)` -- Remove a source
- `GetConfig()` / `UpdateConfig(config)` -- Manage user preferences

The client uses the factory function to initialize the appropriate storage
backend (SQLite, PostgreSQL, MySQL).

## 15.3. Discovery Service (RFC 7)

The CLI client does not directly interact with the discovery service, but:

- Shares the same storage configuration format (section 9.1.1 of RFC 7)
- Displays operational metadata updated by the service (fetch errors, last
  fetch time, error counts)
- Source configuration changes made via the client affect the discovery service
- Both the CLI client and discovery service can run concurrently, accessing the
  same storage

**Concurrent access considerations:**
- Both use the same storage backend
- Database backends handle concurrent access via transactions
- File-based storage may need locking for safe concurrent updates

## 15.4. Web Scraping (RFC 3)

The client needs to understand scraper configuration format from RFC 3 when:

- Creating website sources (validate `scraper_config` JSON)
- Updating scraper configuration
- Displaying scraper settings to users
- Providing examples and documentation

The client validates scraper configuration structure but does not perform
actual scraping.

## 15.5. News Feed API (RFC 4)

The CLI client does **not** use the News Feed API. Web clients would use
RFC 4 for remote access, but the CLI client accesses storage directly.

If both CLI and web clients are used in the same deployment:
- CLI client accesses storage directly (RFC 1)
- Web client accesses via HTTP API (RFC 4)
- Both operate on the same underlying storage

## 15.6. Metadata Management API (RFC 6)

The CLI client does **not** use the Metadata Management API. Web clients would
use RFC 6 for remote access, but the CLI client accesses storage directly.

If both CLI and web clients are used in the same deployment:
- CLI client accesses storage directly (RFC 5)
- Web client accesses via HTTP API (RFC 6)
- Both operate on the same underlying storage
