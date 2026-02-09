package newsfed

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/pevans/newsfed/newsfeed"
)

// FetchFeed fetches and parses an RSS or Atom feed from the given URL. The
// gofeed library automatically detects and handles both RSS and Atom formats.
func FetchFeed(url string) (*gofeed.Feed, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}
	return feed, nil
}

// FeedItemToNewsItem converts an RSS or Atom feed item to a
// newsfeed.NewsItem. Implements RFC 2 section 2.3.1 (RSS) and section 2.4.1
// (Atom) mappings. The gofeed library normalizes both formats into a common
// structure, so this function handles both RSS and Atom feeds transparently.
func FeedItemToNewsItem(item *gofeed.Item, feedTitle string) newsfeed.NewsItem {
	// Generate new UUID for the item
	id := uuid.New()

	// Title: from <title> element (both RSS and Atom)
	title := item.Title
	if title == "" {
		title = "(No title)"
	}

	// Summary: from <description> (RSS) or <summary>/<content> (Atom) gofeed
	// normalizes both to item.Description
	summary := item.Description

	// URL: from <link> (RSS) or <link rel="alternate"> (Atom) gofeed
	// normalizes both to item.Link
	url := item.Link

	// Publisher: from feed-level title (passed as feedTitle parameter)
	var publisher *string
	if feedTitle != "" {
		publisher = &feedTitle
	}

	// Authors: from <author> (RSS/Atom) or <dc:creator> (Dublin Core
	// extension). Atom feeds may have structured <author><name> elements.
	authors := make([]string, 0)
	if item.Author != nil && item.Author.Name != "" {
		authors = append(authors, item.Author.Name)
	}

	// Also check for multiple authors in Authors field
	for _, author := range item.Authors {
		if author.Name != "" && !contains(authors, author.Name) {
			authors = append(authors, author.Name)
		}
	}

	// Check Dublin Core creator
	if item.DublinCoreExt != nil {
		for _, creator := range item.DublinCoreExt.Creator {
			if creator != "" && !contains(authors, creator) {
				authors = append(authors, creator)
			}
		}
	}

	// Published_at: from <updated> (most current) or <published> as fallback.
	// gofeed parses both RSS <pubDate> and Atom <published>/<updated> into
	// PublishedParsed and UpdatedParsed.
	var publishedAt time.Time
	if item.UpdatedParsed != nil {
		// Prefer updated date as it's more current
		publishedAt = *item.UpdatedParsed
	} else if item.PublishedParsed != nil {
		publishedAt = *item.PublishedParsed
	} else {
		// If no date available, use current time
		publishedAt = time.Now()
	}

	// Discovered_at: set to current time when ingesting
	discoveredAt := time.Now()

	// Pinned_at: set to nil (not yet pinned)
	var pinnedAt *time.Time

	return newsfeed.NewsItem{
		ID:           id,
		Title:        title,
		Summary:      summary,
		URL:          url,
		Publisher:    publisher,
		Authors:      authors,
		PublishedAt:  publishedAt,
		DiscoveredAt: discoveredAt,
		PinnedAt:     pinnedAt,
	}
}

// FeedToNewsItems converts all items in an RSS or Atom feed to
// newsfeed.NewsItems. Implements RFC 2 section 2.2.3: conditionally limits to
// 20 most recent items based on published_at timestamp.
//
// The applyLimit parameter determines whether to apply the 20-item cap:
//   - true: limit to 20 most recent items (for first-time sync or stale
//     sources)
//   - false: process all items (for regular polling)
func FeedToNewsItems(feed *gofeed.Feed, applyLimit bool) []newsfeed.NewsItem {
	// Convert all items to newsfeed.NewsItems
	items := make([]newsfeed.NewsItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		newsItem := FeedItemToNewsItem(item, feed.Title)
		items = append(items, newsItem)
	}

	// Sort by published_at (most recent first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})

	// Conditionally limit to 20 most recent items per RFC 2 section 2.2.3
	// Apply limit only for first-time syncs or stale sources (>15 days)
	if applyLimit {
		const maxItems = 20
		if len(items) > maxItems {
			items = items[:maxItems]
		}
	}

	return items
}

// contains checks if a string slice contains a specific string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, str) {
			return true
		}
	}
	return false
}
