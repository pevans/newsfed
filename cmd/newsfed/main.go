package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed"
)

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseDuration extends time.ParseDuration to support 'd' (days) and 'w'
// (weeks)
func parseDuration(s string) (time.Duration, error) {
	// Try standard parsing first
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Handle days (d) and weeks (w)
	if strings.HasSuffix(s, "d") {
		days := s[:len(s)-1]
		var n int
		_, err := fmt.Sscanf(days, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "w") {
		weeks := s[:len(s)-1]
		var n int
		_, err := fmt.Sscanf(weeks, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid duration: %s", s)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse global flags
	metadataPath := getEnv("NEWSFED_METADATA_DSN", "metadata.db")
	feedDir := getEnv("NEWSFED_FEED_DSN", ".news")

	// Get subcommand
	subcommand := os.Args[1]

	switch subcommand {
	case "list":
		handleList(feedDir, os.Args[2:])
	case "show":
		handleShow(feedDir, os.Args[2:])
	case "sources":
		if len(os.Args) < 3 {
			printSourcesUsage()
			os.Exit(1)
		}
		action := os.Args[2]
		handleSourcesCommand(action, metadataPath, os.Args[3:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func handleSourcesCommand(action, metadataPath string, args []string) {
	// Initialize metadata store
	metadataStore, err := newsfed.NewMetadataStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open metadata store: %v\n", err)
		os.Exit(1)
	}
	defer metadataStore.Close()

	switch action {
	case "list":
		handleSourcesList(metadataStore, args)
	case "add":
		handleSourcesAdd(metadataStore, args)
	case "delete":
		handleSourcesDelete(metadataStore, args)
	case "help", "--help", "-h":
		printSourcesUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown sources command: %s\n\n", action)
		printSourcesUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("newsfed - News feed CLI client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  list       List news items")
	fmt.Println("  show       Show detailed view of a news item")
	fmt.Println("  sources    Manage news sources")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEWSFED_METADATA_DSN  Path to metadata database (default: metadata.db)")
	fmt.Println("  NEWSFED_FEED_DSN      Path to news feed storage (default: .news)")
}

func printSourcesUsage() {
	fmt.Println("newsfed sources - Manage news sources")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed sources <action> [arguments]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  list       List all sources")
	fmt.Println("  add        Add a new source")
	fmt.Println("  delete     Delete a source")
	fmt.Println("  help       Show this help message")
}

func handleList(feedDir string, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	pinned := fs.Bool("pinned", false, "Show only pinned items")
	unpinned := fs.Bool("unpinned", false, "Show only unpinned items")
	publisher := fs.String("publisher", "", "Filter by publisher")
	since := fs.String("since", "", "Show items discovered since duration (e.g., 24h, 7d)")
	sortBy := fs.String("sort", "published", "Sort by: published, discovered, pinned")
	limit := fs.Int("limit", 20, "Maximum number of items to display")
	offset := fs.Int("offset", 0, "Number of items to skip")
	fs.Parse(args)

	// Initialize news feed
	newsFeed, err := newsfed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	// Get all items
	items, err := newsFeed.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list news items: %v\n", err)
		os.Exit(1)
	}

	// Apply filters
	var filtered []newsfed.NewsItem
	for _, item := range items {
		// Filter by pinned status
		if *pinned && item.PinnedAt == nil {
			continue
		}
		if *unpinned && item.PinnedAt != nil {
			continue
		}

		// Filter by publisher
		if *publisher != "" {
			if item.Publisher == nil || !strings.Contains(strings.ToLower(*item.Publisher), strings.ToLower(*publisher)) {
				continue
			}
		}

		// Filter by discovered time
		if *since != "" {
			duration, err := parseDuration(*since)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid duration format: %v\n", err)
				os.Exit(1)
			}
			cutoff := time.Now().Add(-duration)
			if item.DiscoveredAt.Before(cutoff) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	// Sort items
	switch *sortBy {
	case "published":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].PublishedAt.After(filtered[j].PublishedAt)
		})
	case "discovered":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].DiscoveredAt.After(filtered[j].DiscoveredAt)
		})
	case "pinned":
		sort.Slice(filtered, func(i, j int) bool {
			// Items with PinnedAt come first, sorted by pinned time (most
			// recent first)
			if filtered[i].PinnedAt == nil && filtered[j].PinnedAt == nil {
				return false
			}
			if filtered[i].PinnedAt == nil {
				return false
			}
			if filtered[j].PinnedAt == nil {
				return true
			}
			return filtered[i].PinnedAt.After(*filtered[j].PinnedAt)
		})
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid sort option: %s (must be published, discovered, or pinned)\n", *sortBy)
		os.Exit(1)
	}

	// Apply pagination
	total := len(filtered)
	if *offset >= total {
		fmt.Println("No items to display.")
		return
	}

	end := min(*offset+*limit, total)
	paged := filtered[*offset:end]

	// Display results
	if len(paged) == 0 {
		fmt.Println("No items found.")
		return
	}

	// Print header
	fmt.Printf("Showing %d-%d of %d items\n\n", *offset+1, *offset+len(paged), total)

	// Print each item
	for _, item := range paged {
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
		fmt.Printf("   %s | Published: %s\n",
			publisher,
			item.PublishedAt.Format("2006-01-02 15:04"),
		)
		if summary != "" {
			fmt.Printf("   %s\n", summary)
		}
		fmt.Printf("   URL: %s\n", item.URL)
		fmt.Printf("   ID: %s\n", item.ID.String())
		fmt.Println()
	}
}

func handleShow(feedDir string, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: item ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed show <item-id>\n")
		os.Exit(1)
	}

	itemID := args[0]

	// Parse UUID
	id, err := uuid.Parse(itemID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid item ID: %v\n", err)
		os.Exit(1)
	}

	// Initialize news feed
	newsFeed, err := newsfed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	// Get the item
	item, err := newsFeed.Get(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get news item: %v\n", err)
		os.Exit(1)
	}

	if item == nil {
		fmt.Fprintf(os.Stderr, "Error: news item not found: %s\n", itemID)
		os.Exit(1)
	}

	// Display the item
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(item.Title)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Publisher
	if item.Publisher != nil {
		fmt.Printf("Publisher:   %s\n", *item.Publisher)
	} else {
		fmt.Println("Publisher:   Unknown")
	}

	// Authors
	if len(item.Authors) > 0 {
		fmt.Printf("Authors:     %s\n", strings.Join(item.Authors, ", "))
	}

	fmt.Println()

	// Dates
	fmt.Printf("Published:   %s\n", item.PublishedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Discovered:  %s\n", item.DiscoveredAt.Format("2006-01-02 15:04:05"))

	// Pinned status
	if item.PinnedAt != nil {
		fmt.Printf("Pinned:      📌 %s\n", item.PinnedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Pinned:      No")
	}

	fmt.Println()

	// URL
	fmt.Printf("URL:         %s\n", item.URL)
	fmt.Println()

	// Summary
	if item.Summary != "" {
		fmt.Println("Summary:")
		fmt.Println(wrapText(item.Summary, 80))
		fmt.Println()
	}

	// ID
	fmt.Printf("ID:          %s\n", item.ID.String())
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

func handleSourcesList(metadataStore *newsfed.MetadataStore, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("sources list", flag.ExitOnError)
	fs.Parse(args)

	// Get all sources
	sources, err := metadataStore.ListSources()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sources: %v\n", err)
		os.Exit(1)
	}

	if len(sources) == 0 {
		fmt.Println("No sources configured.")
		return
	}

	// Print table header
	fmt.Printf("%-36s %-10s %-50s %s\n", "ID", "TYPE", "NAME", "URL")
	fmt.Println("----------------------------------------------------------------------------------------------------")

	// Print each source
	for _, source := range sources {
		// Truncate name and URL if too long
		name := source.Name
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		url := source.URL
		if len(url) > 50 {
			url = url[:47] + "..."
		}

		fmt.Printf("%-36s %-10s %-50s %s\n",
			source.SourceID.String(),
			source.SourceType,
			name,
			url,
		)
	}
}

func handleSourcesAdd(metadataStore *newsfed.MetadataStore, args []string) {
	// Parse flags for add command
	fs := flag.NewFlagSet("sources add", flag.ExitOnError)
	sourceType := fs.String("type", "", "Source type (rss or atom)")
	url := fs.String("url", "", "Source URL")
	name := fs.String("name", "", "Source name")
	fs.Parse(args)

	// Validate required flags
	if *sourceType == "" {
		fmt.Fprintf(os.Stderr, "Error: --type is required\n")
		fs.Usage()
		os.Exit(1)
	}
	if *url == "" {
		fmt.Fprintf(os.Stderr, "Error: --url is required\n")
		fs.Usage()
		os.Exit(1)
	}
	if *name == "" {
		fmt.Fprintf(os.Stderr, "Error: --name is required\n")
		fs.Usage()
		os.Exit(1)
	}

	// Validate source type
	if *sourceType != "rss" && *sourceType != "atom" {
		fmt.Fprintf(os.Stderr, "Error: --type must be 'rss' or 'atom'\n")
		os.Exit(1)
	}

	// Create the source (enabled by default)
	now := time.Now()
	source, err := metadataStore.CreateSource(*sourceType, *url, *name, nil, &now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created source: %s\n", source.SourceID.String())
	fmt.Printf("  Type: %s\n", source.SourceType)
	fmt.Printf("  Name: %s\n", source.Name)
	fmt.Printf("  URL: %s\n", source.URL)
}

func handleSourcesDelete(metadataStore *newsfed.MetadataStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources delete <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Delete the source
	err = metadataStore.DeleteSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to delete source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Deleted source: %s\n", sourceID)
}
