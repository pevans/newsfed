---
Request For Comments: 1
Title: newsfed: a news feed
Drafted At: 2026-01-29
Authors:
  - Peter Evans
---

# 1. newsfed: a news feed

newsfed is a system that provides a news feed to a user. This news feed can
contain items from any reasonable source. Users would consume this news feed
from a client and use that client to read the news item in more depth.

# 2. Storage of the news feed

News items are held in storage for the user to read later. This storage is
persistent; items are retained indefinitely. The data store for news items is
structured but its location is configurable. The news feed consists of a list
of items. 

## 2.1. Structure of a single news item

A single news item consists of at least the following fields:

- `id`, an identifier that is a UUID generated at the time of the item's
  creation.
- `title`, a string which holds the title of the news item.
- `summary`, a string which holds a summary of the news item's content.
- `url`, a URL that the user could use to read the source of the news item.
- `publisher`, an optional string that represents the organizational publisher
  of the news item. Useful when several authors publish news in the same
  publication.
- `authors`, a list of strings that are authors of the news item. A single
  author is simply a person's name -- no other metadata is recorded.
- `published_at`, a timestamp representing the most current date for the news
  item. For items with both published and updated dates, this contains the
  updated date (most recent). For items with only a published date, this
  contains the initial publication date.
- `discovered_at`, a timestamp of when the news item was recorded in a news
  feed.
- `pinned_at`, a timestamp of when the news item was pinned by the feed user.
  Items can be pinned to save them for later reference or to mark them as
  important.

## 2.2. Structure of a news feed

A news feed is a list of news items. Each news item remains in the feed
indefinitely. The client uses the items' metadata to determine what to show --
the feed itself does not track what the most "recent" items are.
