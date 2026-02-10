package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pevans/newsfed/newsfeed"
)

// printListTable prints items in human-readable table format
func printListTable(items []newsfeed.NewsItem, total, offset int) {
	if len(items) == 0 {
		fmt.Println("No items to display.")
		return
	}

	// Print header
	fmt.Printf("Showing %d-%d of %d items\n\n", offset+1, offset+len(items), total)

	// Print each item
	for _, item := range items {
		pinnedMarker := " "
		if item.PinnedAt != nil {
			pinnedMarker = "📌"
		}

		publisher := "Unknown"
		if item.Publisher != nil {
			publisher = *item.Publisher
		}

		// Truncate title and summary for display
		title := item.Title
		if len(title) > 70 {
			title = title[:67] + "..."
		}

		summary := item.Summary
		if len(summary) > 150 {
			summary = summary[:147] + "..."
		}

		fmt.Printf("%s %s\n", pinnedMarker, title)
		fmt.Printf("   %s | Published: %s | Discovered: %s\n",
			publisher,
			item.PublishedAt.Format("2006-01-02 15:04"),
			item.DiscoveredAt.Format("2006-01-02 15:04"),
		)
		if summary != "" {
			fmt.Printf("   %s\n", summary)
		}
		fmt.Printf("   URL: %s\n", item.URL)
		fmt.Printf("   ID: %s\n", item.ID.String())
		fmt.Println()
	}
}

// printListJSON prints items in JSON format
func printListJSON(items []newsfeed.NewsItem, total int) {
	output := map[string]any{
		"items": items,
		"total": total,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}

// printListCompact prints items in compact format
func printListCompact(items []newsfeed.NewsItem) {
	if len(items) == 0 {
		fmt.Println("No items to display.")
		return
	}

	for _, item := range items {
		// Truncate ID to first 8 characters
		shortID := item.ID.String()
		if len(shortID) > 8 {
			shortID = shortID[:8] + "..."
		}

		publisher := "Unknown"
		if item.Publisher != nil {
			publisher = *item.Publisher
		}

		fmt.Printf("%s %s (%s)\n", shortID, item.Title, publisher)
	}
}

// wrapText wraps text to a maximum line width
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= width {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}
