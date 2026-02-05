---
Request For Comments: 7
Title: Newsfeed Discovery Service
Drafted At: 2026-02-02
Authors:
  - Peter Evans
---

# 1. Overview

The newsfeed discovery service is a background service that automatically
discovers and ingests news items from configured sources. It regularly polls
RSS/Atom feeds and scrapes websites according to the source configurations
stored in the metadata database (RFC 5), then adds newly discovered items to
the news feed storage (RFC 1).

This is a daemon-style service with no user-facing API. It operates
autonomously based on source configurations and scheduling rules.

# 2. Architecture

## 2.1. Service Responsibilities

The discovery service is responsible for:

1. **Source Polling** -- Periodically checking each enabled source for new
   content
2. **Content Fetching** -- Retrieving RSS/Atom feeds or scraping web pages
3. **Item Conversion** -- Converting external formats to NewsItem structures
4. **Deduplication** -- Avoiding duplicate items in the feed
5. **Storage** -- Persisting new items to the news feed
6. **Error Handling** -- Managing fetch failures and retry logic
7. **Metadata Updates** -- Recording fetch status and error information

## 2.2. Service Lifecycle

The discovery service runs continuously and follows this lifecycle:

1. **Initialization** -- Connect to metadata store and news feed storage
2. **Source Discovery** -- Load enabled sources from metadata store
3. **Polling Loop** -- Continuously check sources according to their schedules
4. **Graceful Shutdown** -- Complete in-progress operations on termination

# 3. Source Scheduling

## 3.1. Polling Intervals

Each source has a polling interval that determines how frequently it should be
checked:

- Sources may specify a `polling_interval` (e.g., "15m", "1h", "4h")
- If not specified, the system default from config is used
- Minimum polling interval: 5 minutes (to avoid overwhelming sources)
- Maximum polling interval: 24 hours

## 3.2. Scheduling Algorithm

The service uses a simple time-based scheduler:

1. Track `last_fetched_at` for each source
2. Calculate `next_fetch_at = last_fetched_at + polling_interval`
3. In each iteration, fetch all sources where `current_time >= next_fetch_at`
4. Process sources in parallel (with configurable concurrency limit)

## 3.3. Startup Behavior

On startup, the service should:

- Fetch sources that have never been fetched (`last_fetched_at` is null)
- Fetch sources that are overdue (current_time > next_fetch_at)
- For all other sources, wait for their next scheduled time

# 4. RSS/Atom Feed Processing

## 4.1. Feed Fetching

For sources with `source_type` of "rss" or "atom":

1. Use `FetchFeed(url)` from feedparser.go (RFC 2)
2. Handle HTTP caching headers:
   - Send `If-Modified-Since` header if `last_modified` is set
   - Send `If-None-Match` header if `etag` is set
   - On 304 Not Modified, skip processing (no new items)
3. Parse feed into NewsItems using `FeedToNewsItems(feed)` (RFC 2)

## 4.2. Feed Item Processing

The feed parser should conditionally limit processing to the 20 most recent
items from each feed (RFC 2, section 2.2.3). The limit applies only when:
- The source has never been fetched (`last_fetched_at` is null), OR
- The source has not been synced for more than 15 days

For regular polling (source fetched within 15 days), all items are processed.

For each item in the (potentially limited) set:

1. Check if item URL already exists using `URLExists(feed, url)` (RFC 3)
2. If URL exists, skip the item (already in feed)
3. If URL is new, add to feed using `feed.Add(item)` (RFC 1)

## 4.3. Metadata Updates

After processing a feed (success or failure):

- Update `last_fetched_at` to current time
- Update `last_modified` and `etag` from HTTP response headers
- Reset `fetch_error_count` to 0 on success
- Increment `fetch_error_count` on failure
- Set `last_error` to error description on failure

# 5. Web Scraping

## 5.1. Scraper Modes

For sources with `source_type` of "website", the scraper operates in one of
two modes based on `scraper_config.discovery_mode`:

### 5.1.1. Direct Mode

In "direct" mode, the source URL itself is the article page:

1. Fetch HTML from source URL using `FetchHTML(url)` (RFC 3)
2. Extract article using `ExtractArticle(doc, config, url)` (RFC 3)
3. Validate using `ValidateScrapedArticle(article, sourceURL)` (RFC 3)
4. Convert to NewsItem using `ScrapedArticleToNewsItem(article, name)` (RFC 3)
5. Check for duplicates and add to feed

### 5.1.2. List Mode

In "list" mode, the source URL contains a list of article links:

1. Fetch HTML from source URL
2. Extract article URLs using `list_config.article_selector`
3. Conditionally limit to 20 articles maximum (RFC 3, section 3.1.1)
   - Limit applies if source has never been fetched OR not synced for >15 days
   - No limit for regular polling (fetched within 15 days)
4. For each article URL (up to limit if applicable):
   - Check if URL already exists (deduplication)
   - Fetch and extract article (as in direct mode)
   - Add to feed if valid and new
5. Handle pagination if `list_config.pagination_selector` is set:
   - Extract next page URL
   - Fetch next page and repeat
   - Stop after collecting limit (if applicable), after `list_config.max_pages`,
     or when no next page found (whichever comes first)

## 5.2. Scraping Metadata Updates

After scraping (success or failure):

- Update `last_fetched_at`, `fetch_error_count`, `last_error` as with feeds
- For list mode, track number of articles discovered per fetch
- HTTP caching headers are not used for scraped content

# 6. Deduplication

## 6.1. Deduplication Strategy

The service prevents duplicate items using URL-based deduplication:

1. Before adding any item, check `URLExists(feed, item.URL)` (RFC 3)
2. Only add items with URLs not already in the feed
3. This applies to both RSS/Atom feeds and scraped articles

## 6.2. Deduplication Edge Cases

- If a feed contains the same item multiple times, only the first occurrence
  is added
- If two different sources contain the same URL, only the first one seen is
  added
- URL comparison is exact (no normalization beyond what the source provides)

# 7. Error Handling

## 7.1. Transient Errors

Transient errors (network timeouts, temporary server errors) should trigger
retry logic:

- Increment `fetch_error_count` for the source
- Record error in `last_error` field
- Continue polling according to normal schedule
- Log error for monitoring

## 7.2. Permanent Errors

Permanent errors (invalid feed format, 404 Not Found) should:

- Increment `fetch_error_count`
- Record error in `last_error`
- Disable the source (set `enabled = false`)
- Log error for monitoring
- Admin can re-enable sources via metadata API (RFC 6)

## 7.3. Error Thresholds

Sources with excessive consecutive transient failures:

- After 10 consecutive failures, automatically disable the source
- Log error and send notification when source is auto-disabled
- Admin can re-enable sources via metadata API (RFC 6)

## 7.4. Validation Errors

For scraped articles, `ValidateScrapedArticle` may reject articles:

- Invalid URLs, missing titles, dates in future, etc.
- These should be logged but should not increment `fetch_error_count`
- Source is still considered "successfully fetched" even if some articles are
  invalid

# 8. Concurrency

## 8.1. Parallel Fetching

To improve performance, the service should fetch multiple sources in parallel:

- Default concurrency: 5 sources fetched simultaneously
- Configurable via environment variable or config file
- Each source fetch is independent (no shared state during fetch)

## 8.2. Rate Limiting

To avoid overwhelming external sources:

- Limit requests to the same domain
- Default: max 1 request per second per domain
- Configurable per source or globally

## 8.3. Timeouts

All network operations should have timeouts:

- HTTP request timeout: 10 seconds (per RFC 3 section 3.2)
- Total fetch timeout per source: 60 seconds
- If timeout exceeded, treat as transient error

# 9. Configuration

## 9.1. Service Configuration

The discovery service accepts the following configuration:

### 9.1.1. Storage Configuration

Metadata and news feed storage are configured independently with pluggable
backends:

```
# Metadata Storage
NEWSFED_METADATA_TYPE        Storage type: "sqlite", "postgres", "mysql" (required)
NEWSFED_METADATA_DSN         Connection string/DSN for metadata storage (required)
                             Examples:
                             - sqlite: "file:/path/to/metadata.db"
                             - postgres: "postgres://user:pass@host:5432/dbname"
                             - mysql: "user:pass@tcp(host:3306)/dbname"

# News Feed Storage
NEWSFED_FEED_TYPE            Storage type: "file", "sqlite", "postgres" (required)
NEWSFED_FEED_DSN             Connection string/DSN for feed storage (required)
                             Examples:
                             - file: "file:/path/to/feed/directory"
                             - sqlite: "file:/path/to/feed.db"
                             - postgres: "postgres://user:pass@host:5432/dbname"
```

### 9.1.2. Service Configuration

```
NEWSFED_POLL_INTERVAL        Global polling interval (default: "1h")
NEWSFED_CONCURRENCY          Max parallel source fetches (default: 5)
NEWSFED_FETCH_TIMEOUT        Timeout per source fetch (default: "60s")
NEWSFED_DISABLE_THRESHOLD    Auto-disable after N failures (default: 10)
```

### 9.1.3. Implementation Notes

The service should use a factory pattern or dependency injection to instantiate
the appropriate storage backends based on the configured types. Initial
implementation may only support:

- Metadata: SQLite only
- Feed: File-based only

Additional backends can be added later without changing the service logic.

## 9.2. Runtime Configuration

Configuration can be updated at runtime via the config API (RFC 6):

- Changes to `default_polling_interval` affect sources without explicit
  interval
- Service should reload enabled sources periodically to detect new sources
- Recommended: check for new sources every 5 minutes

# 10. Logging and Monitoring

## 10.1. Logging

The service should log:

- INFO: Service start/stop, source fetches (with count of new items)
- WARN: Repeated failures (>10 consecutive), slow fetches (>30s)
- ERROR: Permanent errors, unexpected exceptions

## 10.2. Metrics

Recommended metrics for monitoring:

- `sources_total` -- Total number of enabled sources
- `sources_fetched_total` -- Counter of successful fetches
- `sources_failed_total` -- Counter of failed fetches
- `items_discovered_total` -- Counter of new items added
- `fetch_duration_seconds` -- Histogram of fetch durations

# 11. Implementation Notes

## 11.1. Entry Point

The discovery service should be implemented as a separate command:

```bash
newsfed discover
```

This command runs the discovery service loop until interrupted.

## 11.2. Required Components

The service implementation will use:

- `MetadataStore` from metadata.go (RFC 5) to read sources
- `NewsFeed` from newsfeed.go (RFC 1) to store items
- `FetchFeed`, `FeedToNewsItems` from feedparser.go (RFC 2) for RSS/Atom
- `FetchHTML`, `ExtractArticle`, `ScrapedArticleToNewsItem`,
  `ValidateScrapedArticle`, `URLExists` from scraper.go (RFC 3) for web
  scraping

## 11.3. State Management

The service is largely stateless:

- Source metadata in database serves as persistent state
- No in-memory caching of items or sources required
- Can be stopped and restarted without loss of data

## 11.4. Signal Handling

The service should handle signals gracefully:

- SIGTERM/SIGINT: Complete current fetches, then exit
- SIGHUP: Reload source list and configuration
- Maximum shutdown time: 60 seconds

# 12. Example Workflow

## 12.1. RSS Feed Example (First-Time Sync)

1. Service reads metadata, finds RSS source with
`url="http://example.com/feed"`
2. Checks `last_fetched_at` -- it's null (never fetched before)
3. Calls `FetchFeed("http://example.com/feed")`
4. Receives feed with 50 items
5. Since `last_fetched_at` is null, limits to 20 most recent items based on
   published_at (RFC 2 section 2.2.3)
6. Converts 20 items with `FeedToNewsItems(feed)` → 20 NewsItems
7. For each NewsItem:
   - Calls `URLExists(feed, item.URL)`
   - If false, calls `feed.Add(item)`
   - If true, skips item
8. Discovers 20 new items (all items are new for first sync)
9. Updates source metadata:
   - `last_fetched_at = now()`
   - `fetch_error_count = 0`
   - `last_error = null`
10. Logs: "Fetched http://example.com/feed: 20 new items"

## 12.1a. RSS Feed Example (Regular Polling)

1. Service reads metadata, finds RSS source last fetched 2 days ago
2. Checks `last_fetched_at + polling_interval`, determines fetch is due
3. Calls `FetchFeed("http://example.com/feed")`
4. Receives feed with 50 items
5. Since source was fetched within 15 days, NO limit is applied -- processes
   all 50 items (RFC 2 section 2.2.3)
6. Converts all items with `FeedToNewsItems(feed)` → 50 NewsItems
7. For each NewsItem:
   - Calls `URLExists(feed, item.URL)`
   - If false, calls `feed.Add(item)`
   - If true, skips item
8. Discovers 3 new items (47 were duplicates already in feed)
9. Updates source metadata:
   - `last_fetched_at = now()`
   - `fetch_error_count = 0`
   - `last_error = null`
10. Logs: "Fetched http://example.com/feed: 3 new items"

## 12.2. Website Scraping Example (List Mode, Stale Source)

1. Service reads metadata, finds website source with `discovery_mode="list"`
2. Checks `last_fetched_at` -- source was last fetched 20 days ago (stale)
3. Calls `FetchHTML("http://blog.example.com")`
4. Extracts article links using `list_config.article_selector`
5. Finds 25 article URLs
6. Since source is stale (>15 days), limits to first 20 article URLs (RFC 3
   section 3.1.1)
7. For each article URL (up to 20):
   - Calls `URLExists(feed, articleURL)`
   - If already exists, skip
   - Otherwise, calls `ScrapeArticle(articleURL, article_config)`
   - Validates with `ValidateScrapedArticle`
   - Converts with `ScrapedArticleToNewsItem`
   - Adds to feed with `feed.Add(item)`
8. Discovers 5 new articles (15 were duplicates or invalid)
9. Updates source metadata
10. Logs: "Scraped http://blog.example.com: 5 new items"

# 13. Future Considerations

## 13.1. Incremental Fetching

Future versions could optimize RSS/Atom fetching:

- Track GUIDs of seen items
- Skip processing items with known GUIDs
- This avoids URL checks for every item on every fetch

## 13.2. Conditional Scraping

For websites that update infrequently:

- Use HTTP caching headers (Last-Modified, ETag) for HTML pages
- Skip scraping if page hasn't changed

## 13.3. Webhook Support

Future sources could push updates instead of being polled:

- Add `source_type="webhook"`
- Discovery service exposes HTTP endpoint for push notifications
- Immediately processes pushed items without waiting for poll interval

## 13.4. Distributed Operation

For high-volume deployments:

- Multiple discovery service instances
- Distributed locking to prevent duplicate fetches
- Load balancing across instances
