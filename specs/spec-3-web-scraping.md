---
Specification: 3
Title: Web Scraping Ingestion
Drafted At: 2026-01-30
Authors:
  - Peter Evans
---

# 1. Web Scraping Ingestion

For publications that do not provide RSS or Atom feeds, newsfed supports web
scraping as a fallback ingestion method. Web scraping extracts article
information directly from HTML pages and converts it into NewsItem structures.

This method is inherently more fragile than feed-based ingestion (Spec 2) since
website structure can change without notice. However, it enables newsfed to
aggregate content from any web publication.

# 2. Scraping Source Configuration

## 2.1. Scraper Source Definition

A web scraper source is identified by:

- `source_id`, a unique identifier for the website source (UUID)
- `source_type`, set to "website" to indicate web scraping ingestion
- `url`, the base URL of the publication or a specific page to scrape
- `name`, a human-readable name for the publication
- `enabled`, a boolean indicating whether the scraper should be actively run
- `scraper_config`, a configuration object defining how to extract articles
  from this specific website

## 2.2. Scraper Configuration Structure

The `scraper_config` object specifies how to locate and extract article data
from HTML pages. It includes:

- `discovery_mode`, how to find articles: "list" (scrape index/listing pages)
  or "direct" (URL points directly to an article)
- `list_config`, used when `discovery_mode` is "list":
  - `article_selector`, CSS selector or XPath to find article links on the
    listing page
  - `pagination_selector`, optional selector for "next page" links to follow
  - `max_pages`, maximum number of pages to follow (default: 1)
- `article_config`, defines how to extract data from individual article pages:
  - `title_selector`, CSS selector for the article title
  - `content_selector`, CSS selector for the main article content
  - `author_selector`, optional CSS selector for author name(s)
  - `date_selector`, optional CSS selector for publication date
  - `date_format`, format string for parsing the date (e.g., "2006-01-02" for
    Go time parsing)

## 2.3. Selector Syntax

Selectors use CSS selector syntax (e.g., "article.post h1", "#main-content",
".author-name"). The system should support:

- Element selectors (`div`, `p`, `article`)
- Class selectors (`.classname`)
- ID selectors (`#idname`)
- Attribute selectors (`[data-type="article"]`)
- Descendant combinators (`article h1`)
- Multiple selectors (first match wins)

XPath support is optional but recommended for more complex extraction needs.

# 3. Scraping Process

## 3.1. Discovery and Extraction Flow

The scraping process follows these steps:

1. **Fetch** -- Retrieve the HTML content from the source URL
2. **Discover** -- Identify article URLs (if in "list" mode) or proceed
directly to extraction (if in "direct" mode)
3. **Limit** -- Select up to 20 articles to process (see 3.1.1)
4. **Extract** -- For each article URL, fetch the page and extract metadata
using configured selectors
5. **Transform** -- Convert extracted data to NewsItem structure
6. **Deduplicate** -- Check if the item already exists (by URL)
7. **Store** -- Add new items to the local news feed

### 3.1.1. Article Limiting

To prevent excessive scraping and storage growth when first discovering a
source or re-syncing a stale source, the system should conditionally limit the
number of articles processed per scrape operation:

**When the limit applies:**
- First-time sync: source has never been fetched (`last_fetched_at` is null)
- Stale source: source has not been synced for more than 15 days

**When the limit does NOT apply:**
- Regular polling: source was fetched within the last 15 days

**Limit behavior:**
- Process a maximum of 20 articles per source per scrape (when limit applies)
- In "list" mode, extract up to 20 article URLs from the discovered links
- In "direct" mode, this limit is naturally 1 (single article URL)
- When pagination is used, stop after collecting 20 article URLs total across
  all pages, even if `max_pages` has not been reached
- Articles already in the local feed (detected during deduplication) do not
  count against this limit for the purpose of processing, but the initial
  selection of 20 articles happens before deduplication

The conditional 20-article cap ensures that initial scrapes and stale source
re-scrapes don't result in ingesting hundreds of historical articles, while
allowing regular polling to capture all new articles since the last fetch.

## 3.2. HTML Fetching

When fetching HTML pages, the system should:

- Use HTTP/HTTPS GET requests with a reasonable timeout (e.g., 10 seconds)
- Include a `User-Agent` header identifying the newsfed system
- Respect `robots.txt` directives
- Follow HTTP redirects (up to a reasonable limit, e.g., 5 redirects)
- Handle HTTP errors gracefully (404, 500, etc.) and skip failed pages
- Support HTTPS with proper certificate validation

## 3.3. Rate Limiting and Politeness

To avoid overwhelming target websites, the scraper should:

- Implement rate limiting: minimum delay between requests to the same domain
  (default: 1 second)
- Allow per-source rate limit configuration via `scraper_config`
- Respect `Crawl-delay` directive in `robots.txt`
- Make sequential requests to the same domain (no parallel requests)

## 3.4. Content Extraction

When extracting content from article pages:

- **Title**: Extract text content from the element matching `title_selector`;
  if empty or not found, use "(No title)"
- **Content/Summary**: Extract text content from `content_selector`; remove
  HTML tags and normalize whitespace; truncate to a reasonable length (e.g.,
  500 characters) if used as summary
- **Authors**: Extract text from `author_selector`; if multiple elements
  match, collect all; split on common delimiters (", ", " and ") if multiple
  authors in single element
- **Published date**: Extract text from `date_selector` and parse using
  `date_format`; if parsing fails or not found, use current time as fallback

## 3.5. Error Handling

The scraper should be resilient to common failures:

- **Missing selectors**: If a configured selector doesn't match any elements,
  use sensible defaults (empty string for optional fields, current time for
  dates)
- **Invalid HTML**: Use a lenient HTML parser that handles malformed markup
- **Network errors**: Log the error and skip the page; retry with exponential
  backoff for transient failures (timeouts, 5xx errors)
- **Rate limit responses (429)**: Back off and retry after the specified delay

# 4. Scraper to NewsItem Mapping

## 4.1. Field Mapping

Scraped article data maps to NewsItem fields as follows:

- `id` -- Generated as a new UUID when ingesting
- `title` -- From extracted title (via `title_selector`)
- `summary` -- From extracted content (via `content_selector`), truncated if
  necessary
- `url` -- The URL of the article page
- `publisher` -- From source-level `name` field
- `authors` -- From extracted author(s) (via `author_selector`)
- `published_at` -- From extracted date (via `date_selector` and
  `date_format`); fallback to current time
- `discovered_at` -- Set to current time when ingesting
- `pinned_at` -- Set to nil (not yet pinned)

## 4.2. Deduplication Strategy

To avoid duplicate items when re-scraping:

- Use the article's URL as the unique identifier
- Before adding an item, check if an item with the same URL already exists in
  the local feed
- If a URL already exists, skip adding it (do not update existing items)

# 5. Scraper Scheduling

## 5.1. Polling Frequency

Web scrapers should be run less frequently than feed-based ingesters to
minimize server load:

- Default polling interval: every 1-4 hours
- Make frequency configurable per source
- Avoid scraping during peak hours if possible (configurable)

## 5.2. Incremental Scraping

To reduce redundant work:

- Track the last successful scrape time per source
- In "list" mode, stop pagination when encountering articles older than the
  last scrape time
- Cache discovered article URLs to avoid re-fetching unchanged content

# 6. Implementation Considerations

## 6.1. HTML Parsing Library

Use a robust HTML parsing library that:

- Handles malformed HTML gracefully
- Supports CSS selector queries
- Provides text extraction with whitespace normalization
- Optionally supports XPath for complex queries

## 6.2. JavaScript-Rendered Content

Many modern websites render content dynamically with JavaScript. Initial
implementation may skip such sites. Future enhancements could include:

- Headless browser support (e.g., Chrome/Firefox in headless mode)
- Detection of JavaScript-heavy sites
- Fallback to alternative methods or manual RSS feed addition

## 6.3. Content Validation

Before storing scraped items, validate:

- Title is non-empty and reasonable length (< 500 characters)
- URL is valid and points to the same domain as the source
- Summary is non-empty (warn if empty but don't reject)
- Published date is reasonable (not in future, not before some minimum date
  like 1990-01-01)
