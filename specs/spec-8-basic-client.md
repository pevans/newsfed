---
Specification: 8
Title: Basic News Feed Client
Drafted At: 2026-02-04
Authors:
  - Peter Evans
---

# 1. Overview

A news feed client provides users with an interface for reading news and
managing sources. This specification describes the essential functionality and
implementation patterns for building a CLI-based client.

The CLI client provides direct access to storage layers (metadata and news feed
stores) using Go packages from Spec 1 and Spec 5.

The client serves two primary purposes:

1. **News consumption** -- Browse, read, filter, and pin news items
2. **Source administration** -- Configure and manage RSS/Atom feeds and
   website scrapers

# 2. Client Architecture

## 2.1. CLI Client

A command-line interface client provides direct access to newsfed
functionality by using the storage layer Go packages directly (metadata store
from Spec 5, news feed from Spec 1). The CLI client runs on the same machine
as the data stores.

**Advantages:**
- Scriptable and automatable
- No browser or GUI dependencies
- Easy to integrate with shell workflows
- Direct storage access
- Simple deployment -- single binary with storage configuration

**Disadvantages:**
- Limited visual formatting
- Less intuitive for non-technical users
- Cannot display rich media without external tools
- Must run on same machine as data stores (or have network access to database)

## 2.2. Shared Command Library

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

## 2.3. Implementation Approach

The CLI client directly accesses the storage layers using the Go packages from
Spec 1 (news feed storage) and Spec 5 (metadata storage). This approach provides:

- No server dependencies for basic usage
- Single binary deployment
- Lower latency (direct storage access)
- Fewer moving parts

# 3. Core Functionality

## 3.1. News Reading

### 3.1.1. List News Items

The client should allow users to list news items with various filters:

- Filter by pinned status (pinned only, unpinned only, or all)
- Filter by publisher or author
- Filter by date range (items discovered within a time window)
- Sort by published date, discovered date, or pinned date
- Paginate through large result sets

**Default Behavior:**

By default (when no filters are specified), the list command shows:
- Items discovered in the past 3 days, OR
- Items that are pinned (regardless of discovery date)

This keeps the default view focused on recent and important items. To see all
items regardless of age, use `--all`.

**Example CLI commands:**

```bash
# List recent items (past 3 days) and pinned items
newsfed list

# List all items regardless of age
newsfed list --all

# List pinned items only
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

# Echo the command that would be executed (for testing/debugging)
newsfed open --echo 550e8400-e29b-41d4-a716-446655440000
```

**Flags:**

- `--echo` -- Instead of executing the browser command, print the command that
  would be executed (including the platform-specific browser command and the
  URL). Useful for testing and debugging.

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

The scraper configuration file follows Spec 3 format.

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

### 3.2.7. Sync Sources

Users should be able to manually trigger a fetch from all enabled sources
to check for new content. This is useful for:

- Immediately checking for new content after adding sources
- Testing source configurations
- Refreshing content on demand

```bash
# Sync all enabled sources
newsfed sync

# Sync with verbose output
newsfed sync --verbose

# Sync specific source only
newsfed sync 550e8400...
```

The sync command:
- Fetches from all enabled sources (or a specific source if ID provided)
- Respects HTTP caching headers (If-Modified-Since, ETag)
- Updates operational metadata (last fetched time, error counts)
- Adds newly discovered items to the news feed
- Displays progress and summary of results
- Runs synchronously (blocks until complete)

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
storage.

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

Storage configuration (type and DSN) is loaded in the following order:

1. Environment variables
2. Configuration file (`~/.newsfed/config.yaml`)
3. Default values

**Note:** Storage DSN configuration is **not** exposed as command-line flags.
This keeps the CLI interface clean and encourages using configuration files or
environment variables for deployment-specific settings. Other command-specific
options (filters, output formats, etc.) are available as CLI flags.

# 5. Output Formatting

## 5.1. CLI Output Formats

The CLI client should support multiple output formats:

### 5.1.1. Table Format (Default)

Human-readable table display showing both published and discovered times:

```
📌 Example Article
   TechCrunch | Published: 2026-02-01 10:00 | Discovered: 2026-02-01 12:30
   This is a summary of the article...
   URL: https://example.com/article1
   ID: 550e8400-e29b-41d4-a716-446655440000

  Another Article
   Ars Technica | Published: 2026-02-01 09:30 | Discovered: 2026-02-01 11:45
   Another article summary...
   URL: https://example.com/article2
   ID: 550e8400-e29b-41d4-a716-446655440001
```

The list format includes:
- Pin indicator (📌) for pinned items
- Title
- Publisher, published time, and discovered time
- Summary (truncated)
- URL
- Item ID

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
- For file-based storage, handle individual file read errors

# 7. Security Considerations

## 7.1. File Permissions

For file-based storage, the CLI client should:

- Create storage directories with appropriate permissions (0700)
- Create database files with restricted permissions (0600)
- Warn if storage files have overly permissive permissions
- Never create world-readable storage files

## 7.2. Multi-User Environments

In multi-user systems:

- Default to per-user storage locations (`~/.newsfed/`)
- Support system-wide storage with appropriate permissions
- Document shared database setup for teams

# 8. Implementation Considerations

## 8.1. Storage Initialization

The CLI client should handle storage initialization:

- **Lazy initialization** -- Initialize storage only when needed
- **Auto-migration** -- Automatically upgrade database schemas
- **Health checks** -- Verify storage is accessible on startup
- **Connection pooling** -- Reuse database connections for performance

**Initialization flow:**

1. Load configuration (file, environment, flags)
2. Validate configuration (check DSN format, types)
3. Initialize metadata store using factory pattern (Spec 5)
4. Initialize news feed using factory pattern (Spec 1)
5. Verify connectivity (test query or file access)
6. Handle initialization errors gracefully

## 8.2. Querying and Filtering

The CLI client uses the storage layer directly:

- **News items** -- Call `NewsFeed.List()` with filter criteria (Spec 1)
- **Sources** -- Call `MetadataStore.ListSources()` with filters (Spec 5)
- **Pagination** -- Implement limit/offset in queries
- **Sorting** -- Leverage storage layer sorting capabilities

For large result sets:
- Stream results instead of loading all into memory
- Display results progressively as they're fetched
- Provide progress indicators for slow queries

## 8.3. Transaction Handling

For operations that modify data:

- Use database transactions where supported
- Ensure atomic updates (e.g., pin/unpin operations)
- Roll back on errors
- Provide clear feedback on success/failure

## 8.4. Storage Backend Selection

The client should support multiple storage backends via factory pattern:

```go
// Metadata store factory (from Spec 5)
metadataStore, err := metadata.NewStore(metadataType, metadataDSN)

// News feed factory (from Spec 1)
newsFeed, err := newsfeed.NewFeed(feedType, feedDSN)
```

Initial implementation should support:
- Metadata: SQLite
- Feed: File-based or SQLite

Additional backends can be added later without changing client logic.

# 9. Example Workflows

## 9.1. Daily News Reading Workflow

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

## 9.2. Source Management Workflow

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

## 9.3. Troubleshooting Workflow

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

# 10. Testing

## 10.1. Unit Tests

Test client functionality independently:

- Storage initialization and factory functions
- Command parsing and validation
- Output formatting (table, JSON, compact)
- Error handling and user messages
- Configuration loading

## 10.2. Integration Tests

Test against real or in-memory storage backends:

- Full workflows (list, show, pin, unpin)
- Source CRUD operations
- Error scenarios (storage errors, not found, validation failures)
- Pagination and filtering
- Concurrent access (if applicable)

Use in-memory SQLite (`:memory:`) for fast, isolated integration tests.

## 10.3. End-to-End Tests

Test complete user workflows with real storage:

- CLI command execution against test database
- Configuration loading from files and environment
- Multiple operations in sequence
- State persistence across commands
- Storage migration and initialization

# 11. Implementation Phases

## 11.1. Phase 1: Core CLI Client

Minimal viable product:

- List news items with basic filters
- View individual items
- Pin and unpin items
- Add/remove RSS/Atom sources
- List sources

## 11.2. Phase 2: Enhanced Source Management

Full source administration:

- Website scraper support
- Enable/disable sources
- Update source configuration
- Source health monitoring
- Error reporting
- Manual sync command

## 11.3. Phase 3: Improved UX

Better user experience:

- Configuration file support

# 12. Relationship to Other Specifications

## 12.1. News Feed Storage (Spec 1)

The CLI client directly uses the `NewsFeed` interface from Spec 1:

- `List(filters)` -- Retrieve news items with filtering and sorting
- `Get(id)` -- Get individual item by ID
- `Add(item)` -- Add new items (not typically used by client)
- `Pin(id)` -- Pin an item for later reference
- `Unpin(id)` -- Unpin an item

The client uses the factory function to initialize the appropriate storage
backend (file-based, SQLite, etc.).

## 12.2. Metadata Storage (Spec 5)

The CLI client directly uses the `MetadataStore` interface from Spec 5:

- `ListSources(filters)` -- Retrieve source list with filtering
- `GetSource(id)` -- Get source details including operational metadata
- `CreateSource(source)` -- Add new RSS/Atom feed or website scraper
- `UpdateSource(id, updates)` -- Modify source configuration
- `DeleteSource(id)` -- Remove a source
- `GetConfig()` / `UpdateConfig(config)` -- Manage user preferences

The client uses the factory function to initialize the appropriate storage
backend (SQLite, PostgreSQL, MySQL).

## 12.3. Web Scraping (Spec 3)

The client needs to understand scraper configuration format from Spec 3 when:

- Creating website sources (validate `scraper_config` JSON)
- Updating scraper configuration
- Displaying scraper settings to users
- Providing examples and documentation

The client validates scraper configuration structure but does not perform
actual scraping.
