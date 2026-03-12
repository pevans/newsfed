---
Specification: 10
Title: Feed Autodiscovery
Drafted At: 2026-03-01
Authors:
  - Peter Evans
---

# 1. Feed Autodiscovery

When a user adds a source without specifying a type, newsfed should
automatically determine what kind of source the URL represents. In most cases,
this means finding an RSS or Atom feed -- either because the URL itself is a
feed, or because a feed can be located nearby.

This removes the need for users to know in advance whether a URL is a feed or
a website, and avoids the situation where a publication has a perfectly good
feed that goes unused simply because its URL is not obvious.

# 2. When Autodiscovery Runs

Autodiscovery runs during `newsfed sources add` when the `--type` flag is
omitted. If `--type` is provided, autodiscovery does not run -- the URL is
used as-is with the specified type.

# 3. Discovery Process

Autodiscovery attempts to locate a feed by working through three strategies in
order, stopping as soon as a valid RSS or Atom feed is found.

## 3.1. Direct Feed Detection

First, attempt to parse the given URL directly as a feed. If the response is a
valid RSS or Atom document, the URL is itself a feed and discovery is complete.

This handles the common case where users paste a feed URL directly, or where
the URL's feed nature is not obvious from its path.

## 3.2. HTML Feed Link Detection

If the URL is not itself a feed, fetch the URL as an HTML document and look
for feed links advertised in the page `<head>`:

```html
<link rel="alternate" type="application/rss+xml" href="...">
<link rel="alternate" type="application/atom+xml" href="...">
```

These tags are the standard machine-readable mechanism for advertising feeds,
and are supported by most feed-aware publishing tools. If one or more such
links are found:

- Collect all `href` values whose `type` is `application/rss+xml` or
  `application/atom+xml`
- Resolve relative hrefs against the page URL
- Attempt to parse each in the order they appear
- Use the first one that successfully parses as a valid feed

## 3.3. Common Feed URL Probing

If no feed was found via HTML link tags, probe a set of common feed URL
patterns. Two sets of candidate URLs are generated and tried, in order:

**Path-relative candidates** (derived from the given URL's path, if the path
is not the root `/`):

- `{scheme}://{host}{path}/index.xml`
- `{scheme}://{host}{path}/feed`
- `{scheme}://{host}{path}/feed.xml`
- `{scheme}://{host}{path}/rss.xml`
- `{scheme}://{host}{path}/atom.xml`

Where `{path}` is the given URL's path with any trailing slash removed.

**Root-relative candidates** (derived from the site root):

- `{scheme}://{host}/index.xml`
- `{scheme}://{host}/feed`
- `{scheme}://{host}/feed.xml`
- `{scheme}://{host}/rss`
- `{scheme}://{host}/rss.xml`
- `{scheme}://{host}/atom.xml`
- `{scheme}://{host}/feeds/posts/default`

Path-relative candidates are tried before root-relative candidates. Candidates
that are identical to one already tried are skipped.

Each candidate URL is fetched and parsed. The first that successfully parses
as a valid RSS or Atom feed is used.

# 4. Determining Feed Type

Once a valid feed is found -- regardless of which strategy found it -- the
feed type (`rss` or `atom`) is determined from the parsed feed document. The
format of the `<feed>` or `<rss>` root element identifies the type. The source
is stored with the discovered type.

# 5. Timeouts

## 5.1. Context Propagation

The autodiscovery function must accept a `context.Context` parameter. All HTTP
requests made during discovery -- feed parses, HTML fetches, and probe
attempts -- must respect this context. If the context is cancelled or its
deadline expires, discovery stops immediately and returns the context error.

## 5.2. Overall Discovery Timeout

Callers must provide a context with a deadline. The recommended overall
timeout for a complete discovery operation is 30 seconds. This bounds the
total wall-clock time across all three strategies, which in the worst case may
issue over a dozen sequential HTTP requests.

Individual HTTP requests within the discovery process are subject to the
10-second per-request timeout defined in Spec 2, Section 2.2.1. The overall
discovery timeout is a separate, higher-level limit that caps the cumulative
time of the entire operation.

# 6. Discovery Outcomes

## 6.1. Feed Found

When a feed is found, a source is created using the **discovered feed URL**
(not the original URL the user provided, if different) and the discovered feed
type. The source is created in the enabled state.

If the user provided a `--name` flag, that name is used. If `--name` was not
provided, the feed's own title is used as the source name.

## 6.2. No Feed Found

If all three strategies are exhausted without finding a valid feed, the command
fails with an error. The error message should indicate that no feed was found
at the given URL and suggest adding the source explicitly as a website type
if the user wants to use CSS-selector scraping instead.

# 7. CLI Changes

## 7.1. The `--type` Flag Becomes Optional

The `--type` flag in `newsfed sources add` becomes optional. When omitted,
autodiscovery runs. When provided, the given type is used without any
autodiscovery.

## 7.2. The `--name` Flag Becomes Optional During Autodiscovery

When `--type` is omitted (autodiscovery mode), `--name` is also optional.
If omitted, the name is taken from the discovered feed's title. If `--type`
is provided, `--name` remains required as today.

## 7.3. Output

When autodiscovery finds a feed at a different URL than the one provided, the
output should make clear what was found and where:

```
Discovered RSS feed at https://www.hillelwayne.com/index.xml
Created source: Hillel Wayne (rss)
  ID: 550e8400-e29b-41d4-a716-446655440000
  URL: https://www.hillelwayne.com/index.xml
```

When the given URL is itself a feed, no discovery notice is shown -- just the
standard created source output.

## 7.4. Error Output

When no feed is found, the error output should list the strategies that were
attempted and suggest the website source alternative:

```
Error: no feed found at https://www.hillelwayne.com/post/

Tried:
  https://www.hillelwayne.com/post/ -- not a feed
  https://www.hillelwayne.com/ -- no feed links in page
  https://www.hillelwayne.com/post/index.xml -- 404
  https://www.hillelwayne.com/post/feed -- 404
  https://www.hillelwayne.com/index.xml -- 404
  ...

To add this URL as a website source using CSS-selector scraping:
  newsfed sources add --type=website --url=<url> --name=<name> --config=<file>
```

## 7.5. Example Usage

```bash
# Autodiscovery: type inferred, name taken from feed title
newsfed sources add --url="https://www.hillelwayne.com/post/"

# Autodiscovery with explicit name
newsfed sources add --url="https://www.hillelwayne.com/post/" --name="Hillel Wayne"

# Explicit type: no autodiscovery
newsfed sources add --type=rss --url="https://www.hillelwayne.com/index.xml" --name="Hillel Wayne"
```

# 8. TUI Changes

## 8.1. Add Source Modal

The "Add Source" modal defined in Spec 9, Section 10 is updated to remove
the Type field. The updated modal presents two labeled fields:

- Name: \<input\>
- URL:  \<input\>

The Type field is removed entirely. Autodiscovery always runs when a source is
added through the TUI. Users who need to specify an explicit type (e.g., to add
a website source with a scraper config) must use the CLI.

The Name field is optional. If left blank, the feed's own title is used as the
source name. The URL field is the only required field; if it is empty when the
user presses enter, a status message is shown and the modal remains open.

## 8.2. Discovering State

When the user presses enter with a valid URL, the modal enters a discovering
state:

- The input fields become non-editable
- A status line replaces the field area or appears below the fields, reading:
  "Discovering feed..."
- The user cannot interact with the modal while discovery is in progress
- The escape key has no effect during discovery

## 8.3. Success

On successful discovery, the modal closes and the new source appears in the
source list with the cursor positioned on it, as today.

## 8.4. Failure

If discovery fails, the modal returns to its editable state. The status line
shows a brief error message:

```
No feed found. Check the URL and try again.
```

The user may correct the URL and try again, or press escape to dismiss the
modal without adding a source.

# 9. Relationship to Other Specifications

## 9.1. Feed Ingestion (Spec 2)

Autodiscovery produces sources that are consumed by the feed ingestion process
defined in Spec 2. Discovered feed sources are identical in structure to
manually configured feed sources.

## 9.2. Web Scraping (Spec 3)

Autodiscovery does not fall back to website scraping. If no feed is found, the
command fails. Users who want CSS-selector-based scraping must explicitly use
`--type=website` via the CLI.

## 9.3. Basic Client (Spec 8)

This specification extends the `sources add` command defined in Spec 8,
Section 3.2.3, by making `--type` and `--name` optional and defining the
autodiscovery behavior that runs when `--type` is absent.

## 9.4. Text User Interface (Spec 9)

This specification updates the "Add Source" modal defined in Spec 9, Section
10, removing the Type field and adding the discovering state described in
Section 8 above.
