package main

import (
	"fmt"
	"log"
)

func main() {
	// Create a news feed stored in the .news directory
	feed, err := New(".news")
	if err != nil {
		log.Fatalf("Failed to create news feed: %v", err)
	}

	// Fetch a feed (this one is Atom format -- the code handles both RSS and
	// Atom)
	fmt.Println("Fetching feed from https://go.dev/blog/feed.atom...")
	externalFeed, err := FetchFeed("https://go.dev/blog/feed.atom")
	if err != nil {
		log.Fatalf("Failed to fetch feed: %v", err)
	}

	fmt.Printf("Fetched feed: %s (format: %s)\n", externalFeed.Title, externalFeed.FeedType)
	fmt.Printf("Found %d items in feed\n\n", len(externalFeed.Items))

	// Convert feed items to NewsItems (works for both RSS and Atom)
	newsItems := FeedToNewsItems(externalFeed)

	// Add first 5 items to the feed (to avoid cluttering)
	maxItems := 5
	if len(newsItems) < maxItems {
		maxItems = len(newsItems)
	}

	for i := 0; i < maxItems; i++ {
		if err := feed.Add(newsItems[i]); err != nil {
			log.Fatalf("Failed to add item: %v", err)
		}
	}

	fmt.Printf("Added %d items from external feed to local storage\n\n", maxItems)

	// List all items in the feed
	items, err := feed.List()
	if err != nil {
		log.Fatalf("Failed to list items: %v", err)
	}

	fmt.Printf("News feed now contains %d items:\n", len(items))
	for i, item := range items {
		fmt.Printf("\n%d. %s\n", i+1, item.Title)
		if item.Publisher != nil {
			fmt.Printf("   Publisher: %s\n", *item.Publisher)
		}
		if len(item.Authors) > 0 {
			fmt.Printf("   Authors: %s\n", item.Authors[0])
		}
		fmt.Printf("   Published: %s\n", item.PublishedAt.Format("2006-01-02"))
		fmt.Printf("   URL: %s\n", item.URL)
	}
}
