package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

func main() {
	// Create a news feed stored in the .news directory
	feed, err := NewNewsFeed(".news")
	if err != nil {
		log.Fatalf("Failed to create news feed: %v", err)
	}

	// Create some example news items
	publisher1 := "TechNews Daily"
	item1 := NewsItem{
		ID:           uuid.New(),
		Title:        "Go 1.26 Released with New Features",
		Summary:      "The Go team has announced the release of Go 1.26, bringing improved performance and new standard library features.",
		URL:          "https://go.dev/blog/go1.26",
		Publisher:    &publisher1,
		Authors:      []string{"The Go Team", "Jane Smith"},
		PublishedAt:  time.Now().Add(-24 * time.Hour),
		DiscoveredAt: time.Now(),
		ViewedAt:     nil,
	}

	publisher2 := "Dev Weekly"
	item2 := NewsItem{
		ID:           uuid.New(),
		Title:        "Understanding Cloud Native Architecture",
		Summary:      "A deep dive into cloud native patterns and how they enable scalable applications.",
		URL:          "https://devweekly.com/cloud-native",
		Publisher:    &publisher2,
		Authors:      []string{"Alice Johnson"},
		PublishedAt:  time.Now().Add(-48 * time.Hour),
		DiscoveredAt: time.Now(),
		ViewedAt:     nil,
	}

	// Add items to the feed
	if err := feed.Add(item1); err != nil {
		log.Fatalf("Failed to add item1: %v", err)
	}
	if err := feed.Add(item2); err != nil {
		log.Fatalf("Failed to add item2: %v", err)
	}

	fmt.Println("Added 2 items to the news feed in .news/")
	fmt.Println()

	// List all items in the feed
	items, err := feed.List()
	if err != nil {
		log.Fatalf("Failed to list items: %v", err)
	}

	fmt.Printf("News feed contains %d items:\n", len(items))
	for i, item := range items {
		fmt.Printf("\n%d. %s\n", i+1, item.Title)
		fmt.Printf("   Published: %s\n", item.PublishedAt.Format("2006-01-02"))
		fmt.Printf("   URL: %s\n", item.URL)
	}
}
