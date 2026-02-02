---
Request For Comments: 5
Title: Metadata Storage for Sources and Configuration
Drafted At: 2026-02-01
Authors:
  - Peter Evans
---

# 1. Metadata Storage

newsfed requires persistent storage of configuration metadata to manage its
news aggregation sources. This includes RSS/Atom feed sources, web scraper
configurations, and potentially other metadata needed for operation.

This RFC defines the structure and storage mechanism for this metadata. The
metadata storage system is separate from news item storage (RFC 1) -- metadata
describes *where* to find news, while news items are the actual content.

# 2. Metadata Types

## 2.1. Source Metadata

Source metadata defines external sources from which newsfed ingests news
items. Each source represents either:

- An RSS/Atom feed (as defined in RFC 2)
- A web scraper configuration (as defined in RFC 3)

All source types share common metadata fields:

- `source_id` -- UUID uniquely identifying this source
- `source_type` -- String indicating type: "rss", "atom", "website"
- `url` -- URL where the source can be accessed
- `name` -- Human-readable name for the source
- `enabled_at` -- Timestamp when the source was enabled; null if disabled
- `created_at` -- Timestamp when the source was added to the system
- `updated_at` -- Timestamp when the source configuration was last modified

## 2.2. Feed Source Metadata

For sources with `source_type` of "rss" or "atom", additional metadata may
include:

- `polling_interval` -- Duration between fetch attempts (e.g., "15m", "1h")
- `last_fetched_at` -- Timestamp of the most recent successful fetch
- `last_modified` -- HTTP Last-Modified header from last fetch (for caching)
- `etag` -- HTTP ETag header from last fetch (for caching)
- `fetch_error_count` -- Number of consecutive fetch failures
- `last_error` -- Description of the most recent fetch error

## 2.3. Website Source Metadata

For sources with `source_type` of "website", the configuration includes:

- `scraper_config` -- Object containing scraper-specific configuration as
  defined in RFC 3, section 2.2

Additionally, website sources may include the same operational metadata as
feed sources (polling_interval, last_fetched_at, etc.).

## 2.4. User Preferences

User-level configuration metadata includes:

- `default_polling_interval` -- Default time between source fetches

# 3. Storage Mechanism

## 3.1. SQLite-Based Storage

The initial implementation uses SQLite for metadata storage. SQLite provides
structured querying, transactions, and concurrent access while remaining
lightweight and requiring no separate server process.

Metadata is stored in a SQLite database file separate from news items.

Recommended location:

```
.newsfed/metadata.db      # SQLite database file
```

Future implementations may use other storage backends (PostgreSQL, MySQL, etc.),
but SQLite is sufficient for single-user deployments.

### 3.1.1. Database Schema

The database contains two main tables: `sources` and `config`.

**Sources Table:**

```sql
CREATE TABLE sources (
    source_id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    url TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    polling_interval TEXT,
    last_fetched_at TEXT,
    last_modified TEXT,
    etag TEXT,
    fetch_error_count INTEGER DEFAULT 0,
    last_error TEXT,
    scraper_config TEXT  -- JSON blob for website sources
);
```

Notes:
- `source_id` is stored as TEXT (UUID string representation)
- Timestamps are stored as TEXT in RFC 3339 format
- `enabled_at` is NULL when source is disabled
- `scraper_config` stores the entire scraper configuration as JSON for website sources

**Config Table:**

```sql
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

The config table stores key-value pairs for user preferences:
- `default_polling_interval` -- Default polling interval (e.g., "1h")

### 3.1.2. Example Data

**RSS Source:**

```sql
INSERT INTO sources (
    source_id, source_type, url, name, enabled_at,
    created_at, updated_at, polling_interval, last_fetched_at, etag
) VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'rss',
    'https://example.com/feed.xml',
    'Example Blog',
    '2026-01-15T10:00:00Z',
    '2026-01-15T10:00:00Z',
    '2026-01-20T14:30:00Z',
    '1h',
    '2026-01-20T14:30:00Z',
    '"abc123"'
);
```

**Website Source:**

```sql
INSERT INTO sources (
    source_id, source_type, url, name, enabled_at,
    created_at, updated_at, polling_interval, scraper_config
) VALUES (
    '550e8400-e29b-41d4-a716-446655440001',
    'website',
    'https://example.com/articles',
    'Example News Site',
    '2026-01-15T10:00:00Z',
    '2026-01-15T10:00:00Z',
    '2026-01-20T14:30:00Z',
    '2h',
    '{
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
    }'
);
```

## 3.2. Alternative Storage Options

Future implementations may support alternative storage backends:

- **PostgreSQL/MySQL** -- For multi-user or networked deployments
- **Key-Value Store** -- For distributed or cloud-based deployments

The metadata storage interface should be designed to allow swapping storage
backends without changing the API.

# 4. Operations

## 4.1. Source Management

The metadata storage system must support standard CRUD operations:

- **Create** -- Add a new source with configuration
- **Read** -- Retrieve source configuration by ID or list all sources
- **Update** -- Modify source configuration or operational metadata
- **Delete** -- Remove a source from the system

### 4.1.1. Create Source

```
CreateSource(source_type, url, name, config) -> source_id
```

Creates a new source with:
- Generated UUID as source_id
- enabled_at set to current time (source is enabled by default)
- created_at and updated_at set to current time
- Type-specific config as provided

### 4.1.2. Read Source

```
GetSource(source_id) -> source or error
ListSources() -> []source
ListSourcesByType(source_type) -> []source
ListEnabledSources() -> []source
```

### 4.1.3. Update Source

```
UpdateSource(source_id, updates) -> error
```

Updates can modify any mutable field:
- name, url, enabled_at, polling_interval
- scraper_config (for website sources)
- Automatically updates updated_at timestamp

Operational metadata (last_fetched_at, etag, etc.) is updated separately by the
fetching system, not by user actions.

### 4.1.4. Delete Source

```
DeleteSource(source_id) -> error
```

Removes the source configuration. This does NOT delete news items that were
previously ingested from this source -- they remain in the news feed.

## 4.2. Configuration Management

User configuration operations:

```
GetConfig() -> config
UpdateConfig(updates) -> error
```

Configuration changes take effect immediately.

# 5. Relationship to Other Components

## 5.1. News Feed Aggregator

A news aggregator component (not yet defined in an RFC) would:

1. Use `ListEnabledSources()` to find active sources
2. For each source, check its polling_interval and last_fetched_at to determine
   if it should be fetched
3. Fetch and parse the source according to its type (RFC 2 or RFC 3)
4. Update operational metadata (last_fetched_at, etag, etc.) after fetch
   attempts
5. Store discovered news items in the news feed (RFC 1)

## 5.2. CLI Tools

A command-line interface for managing sources would be useful:

```bash
newsfed source add --type=rss --url=https://... --name="..."
newsfed source list
newsfed source enable {source_id}
newsfed source disable {source_id}
newsfed source remove {source_id}
newsfed source show {source_id}
```

# 6. Implementation Considerations

## 6.1. Validation

Source configurations should be validated before saving:

- URL format validation
- Required fields present
- scraper_config structure matches expected schema
- polling_interval is a valid duration

## 6.2. Migration and Backup

Metadata should be easy to backup and migrate:

- SQLite database can be backed up by copying the database file
- Export/import functions for migrating between storage backends
- Version the metadata schema to support future changes

## 6.3. Defaults and Templates

The system could provide:

- Default scraper configurations for popular websites
- Templates for common feed types
- Auto-discovery of feed URLs from website URLs

## 6.4. Source Groups and Tags

Future enhancement: organize sources into groups or apply tags for easier
management:

```json
{
  "source_id": "...",
  "tags": ["tech", "news", "daily"],
  "group": "technology-feeds"
}
```

This would enable operations like "fetch all sources in the 'breaking-news'
group" or "show news from sources tagged 'weekly-digest'".

# 7. Security Considerations

## 7.1. URL Validation

Source URLs should be validated to prevent:

- Local file access (`file://` URLs)
- Access to internal network resources (unless explicitly allowed)
- Malicious redirects

## 7.2. Credential Storage

If sources require authentication (future enhancement), credentials must be
stored securely:

- Encrypt sensitive fields in source metadata
- Use system keychain/credential manager where available
- Never log or expose credentials in error messages

## 7.3. Rate Limiting

Metadata about fetch attempts (last_fetched_at, fetch_error_count) helps
implement rate limiting to avoid overwhelming source servers or being blocked.

# 8. Testing Considerations

Metadata storage should be testable:

- Use in-memory or temporary storage for tests
- Provide factory functions for creating test sources
- Validate that CRUD operations work correctly
- Test concurrent access scenarios
- Test migration between storage backends
