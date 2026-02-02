---
Request For Comments: 2
Title: External News Feed Ingestion
Drafted At: 2026-01-30
Authors:
  - Peter Evans
---

# 1. External News Feed Ingestion

newsfed aggregates news items from external sources into a user's personal
news feed. An external news feed is any source that provides structured news
or article data that can be converted into news items as defined in RFC 1,
section 2.1.

The system should support multiple types of external feed formats, each with
its own parsing logic but all producing the same normalized NewsItem
structure.

# 2. External Feed Sources

## 2.1. Feed Source Configuration

An external feed source is identified by:

- `source_id`, a unique identifier for the feed source (UUID)
- `source_type`, a string indicating the feed format (e.g., "rss", "atom",
  "json")
- `url`, the URL where the feed can be fetched
- `name`, a human-readable name for the feed source
- `enabled`, a boolean indicating whether the feed should be actively polled

## 2.2. Feed Ingestion Process

The ingestion process follows these steps:

1. **Fetch** -- Retrieve the external feed content from its URL
2. **Parse** -- Parse the feed according to its format specification
3. **Transform** -- Convert each external item to the NewsItem structure
4. **Deduplicate** -- Check if the item already exists in the local feed (by
URL or external ID)
5. **Store** -- Add new items to the local news feed

The `discovered_at` timestamp is set when the item is first ingested into the
local feed. The `published_at` timestamp should be taken from the external
feed's publication date if available.

### 2.2.1. Feed Fetching

XML-based feeds (RSS, Atom) should be fetched via HTTP/HTTPS GET requests. The
system should:

- Respect standard HTTP caching headers (`ETag`, `Last-Modified`)
- Include a reasonable `User-Agent` header identifying the newsfed system
- Handle HTTP errors gracefully (404, 500, etc.) and retry with exponential
  backoff
- Support HTTPS with proper certificate validation

### 2.2.2. Polling Frequency

External feeds should be polled periodically to discover new items. The
polling frequency should be configurable per feed source, with a reasonable
default (e.g., every 15 minutes to 1 hour). Feeds that update infrequently can
be polled less often.

## 2.3. RSS Feed Support

RSS (Really Simple Syndication) is a widely-used XML format for syndicating
web content. newsfed supports RSS 2.0 feeds.

### 2.3.1. RSS to NewsItem Mapping

RSS items are mapped to NewsItem fields as follows:

- `id` -- Generated as a new UUID when ingesting
- `title` -- From `<title>` element
- `summary` -- From `<description>` element; if not present, use an empty
  string or truncated content
- `url` -- From `<link>` element (text content)
- `publisher` -- From channel-level `<title>` or `<managingEditor>` if
  item-level publisher is not available
- `authors` -- From `<author>` or `<dc:creator>` element (Dublin Core
  extension); if multiple authors are in a single field, parse as a list
- `published_at` -- From `<pubDate>` element; parse as RFC 822 date format
- `discovered_at` -- Set to current time when ingesting
- `pinned_at` -- Set to nil (not yet pinned)

### 2.3.2. RSS Deduplication Strategy

To avoid duplicate items when re-fetching an RSS feed:

- Use the RSS item's `<guid>` if present and marked as a permalink
- Otherwise, use the item's `<link>` URL as a unique identifier
- Before adding an item, check if an item with the same URL already exists in
  the local feed

## 2.4. Atom Feed Support

Atom is an XML-based web feed format standardized as IETF RFC 4287. Atom feeds
provide more structured metadata than RSS and use different element names and
formats.

### 2.4.1. Atom to NewsItem Mapping

Atom entries are mapped to NewsItem fields as follows:

- `id` -- Generated as a new UUID when ingesting (Atom's `<id>` element can be
  used for deduplication but is not stored as the NewsItem ID)
- `title` -- From `<title>` element
- `summary` -- From `<summary>` element; if not present, use `<content>`
  element (truncated if necessary)
- `url` -- From `<link rel="alternate">` element's `href` attribute; if
  multiple alternate links exist, prefer the first one or the one with
  `type="text/html"`
- `publisher` -- From feed-level `<title>` or `<author><name>` element
- `authors` -- From `<author><name>` element(s); Atom supports multiple author
  elements per entry
- `published_at` -- From `<updated>` element (preferred as most current); if
  not present, use `<published>` element as fallback; parse as ISO 8601 date
  format
- `discovered_at` -- Set to current time when ingesting
- `pinned_at` -- Set to nil (not yet pinned)

### 2.4.2. Atom Deduplication Strategy

To avoid duplicate items when re-fetching an Atom feed:

- Use the Atom entry's `<id>` element as the primary unique identifier (Atom
  IDs are required and intended to be permanent)
- As a fallback, use the `<link rel="alternate">` URL
- Before adding an item, check if an item with the same Atom ID or URL already
  exists in the local feed
