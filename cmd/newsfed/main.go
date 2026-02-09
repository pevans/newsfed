package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed"
	"github.com/pevans/newsfed/sources"
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
	case "pin":
		handlePin(feedDir, os.Args[2:])
	case "unpin":
		handleUnpin(feedDir, os.Args[2:])
	case "sync":
		handleSync(metadataPath, feedDir, os.Args[2:])
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
	// Initialize source store
	sourceStore, err := sources.NewSourceStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open source store: %v\n", err)
		os.Exit(1)
	}
	defer sourceStore.Close()

	switch action {
	case "list":
		handleSourcesList(sourceStore, args)
	case "show":
		handleSourcesShow(sourceStore, args)
	case "add":
		handleSourcesAdd(sourceStore, args)
	case "update":
		handleSourcesUpdate(sourceStore, args)
	case "delete":
		handleSourcesDelete(sourceStore, args)
	case "enable":
		handleSourcesEnable(sourceStore, args)
	case "disable":
		handleSourcesDisable(sourceStore, args)
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
	fmt.Println("  pin        Pin a news item for later reference")
	fmt.Println("  unpin      Unpin a news item")
	fmt.Println("  sync       Manually sync sources to fetch new items")
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
	fmt.Println("  show       Show detailed source information")
	fmt.Println("  add        Add a new source")
	fmt.Println("  update     Update source configuration")
	fmt.Println("  delete     Delete a source")
	fmt.Println("  enable     Enable a source")
	fmt.Println("  disable    Disable a source")
	fmt.Println("  help       Show this help message")
}

func handleList(feedDir string, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "Show all items regardless of age")
	pinned := fs.Bool("pinned", false, "Show only pinned items")
	unpinned := fs.Bool("unpinned", false, "Show only unpinned items")
	publisher := fs.String("publisher", "", "Filter by publisher")
	since := fs.String("since", "", "Show items discovered since duration (e.g., 24h, 7d)")
	sortBy := fs.String("sort", "published", "Sort by: published, discovered, pinned")
	limit := fs.Int("limit", 20, "Maximum number of items to display")
	offset := fs.Int("offset", 0, "Number of items to skip")
	format := fs.String("format", "table", "Output format: table, json, compact")
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
		// Default filter: show items from past 3 days OR pinned items (unless
		// --all is set)
		if !*all && *since == "" && !*pinned && !*unpinned {
			threeDaysAgo := time.Now().Add(-3 * 24 * time.Hour)
			isRecent := item.DiscoveredAt.After(threeDaysAgo)
			isPinned := item.PinnedAt != nil
			if !isRecent && !isPinned {
				continue
			}
		}

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

		// Filter by discovered time (explicit --since overrides default)
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

	// Display results based on format
	switch *format {
	case "json":
		printListJSON(paged, total)
	case "compact":
		printListCompact(paged)
	case "table":
		printListTable(paged, total, *offset)
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid format: %s (must be table, json, or compact)\n", *format)
		os.Exit(1)
	}
}

// printListTable prints items in human-readable table format
func printListTable(items []newsfed.NewsItem, total, offset int) {
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
func printListJSON(items []newsfed.NewsItem, total int) {
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
func printListCompact(items []newsfed.NewsItem) {
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

func handlePin(feedDir string, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: item ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed pin <item-id>\n")
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

	// Check if already pinned
	if item.PinnedAt != nil {
		fmt.Printf("Item is already pinned (pinned at: %s)\n", item.PinnedAt.Format("2006-01-02 15:04:05"))
		return
	}

	// Pin the item
	now := time.Now()
	item.PinnedAt = &now

	err = newsFeed.Update(*item)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to pin item: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Pinned item: %s\n", item.Title)
}

func handleUnpin(feedDir string, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: item ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed unpin <item-id>\n")
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

	// Check if already unpinned
	if item.PinnedAt == nil {
		fmt.Println("Item is already unpinned")
		return
	}

	// Unpin the item
	item.PinnedAt = nil

	err = newsFeed.Update(*item)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to unpin item: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Unpinned item: %s\n", item.Title)
}

func handleSync(metadataPath, feedDir string, args []string) {
	// Parse flags for sync command
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show verbose output")
	fs.Parse(args)

	// Check if a specific source ID was provided
	var sourceID *uuid.UUID
	if len(fs.Args()) > 0 {
		id, err := uuid.Parse(fs.Args()[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
			os.Exit(1)
		}
		sourceID = &id
	}

	// Initialize source store
	sourceStore, err := sources.NewSourceStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open source store: %v\n", err)
		os.Exit(1)
	}
	defer sourceStore.Close()

	// Initialize news feed
	newsFeed, err := newsfed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	// Create discovery service
	config := &newsfed.DiscoveryConfig{
		FetchTimeout: 60 * time.Second,
	}
	service := newsfed.NewDiscoveryService(sourceStore, newsFeed, config)

	// Perform sync
	if sourceID != nil {
		source, err := sourceStore.GetSource(*sourceID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Syncing source: %s\n", source.Name)
	} else {
		fmt.Println("Syncing all enabled sources...")
	}

	ctx := context.Background()
	result, err := service.SyncSources(ctx, sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: sync failed: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Println()
	fmt.Println("Sync completed:")
	fmt.Printf("  Sources synced: %d\n", result.SourcesSynced)
	fmt.Printf("  Sources failed: %d\n", result.SourcesFailed)
	fmt.Printf("  Items discovered: %d\n", result.ItemsDiscovered)

	// Show errors if any
	if len(result.Errors) > 0 && *verbose {
		fmt.Println()
		fmt.Println("Errors:")
		for _, syncErr := range result.Errors {
			fmt.Printf("  - %s: %v\n", syncErr.Source.Name, syncErr.Error)
		}
	}

	// Exit with error code if any sources failed
	if result.SourcesFailed > 0 {
		os.Exit(1)
	}
}

func handleSourcesList(metadataStore *sources.SourceStore, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("sources list", flag.ExitOnError)
	fs.Parse(args)

	// Get all sources
	sourceList, err := metadataStore.ListSources(sources.SourceFilter{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sources: %v\n", err)
		os.Exit(1)
	}

	if len(sourceList) == 0 {
		fmt.Println("No sources configured.")
		return
	}

	// Print table header
	fmt.Printf("%-36s %-10s %-50s %s\n", "ID", "TYPE", "NAME", "URL")
	fmt.Println("----------------------------------------------------------------------------------------------------")

	// Print each source
	for _, source := range sourceList {
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

func handleSourcesShow(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources show <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Display the source
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(source.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Basic info
	fmt.Printf("Type:        %s\n", source.SourceType)
	fmt.Printf("URL:         %s\n", source.URL)
	fmt.Println()

	// Status
	if source.EnabledAt != nil {
		fmt.Printf("Status:      ✓ Enabled (since %s)\n", source.EnabledAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Status:      ✗ Disabled")
	}
	fmt.Println()

	// Operational metadata
	fmt.Println("Operational Info:")
	if source.LastFetchedAt != nil {
		fmt.Printf("  Last Fetched:    %s\n", source.LastFetchedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("  Last Fetched:    Never")
	}

	if source.PollingInterval != nil {
		fmt.Printf("  Poll Interval:   %s\n", *source.PollingInterval)
	} else {
		fmt.Println("  Poll Interval:   Default")
	}
	fmt.Println()

	// Health status
	fmt.Println("Health:")
	fmt.Printf("  Error Count:     %d\n", source.FetchErrorCount)
	if source.LastError != nil {
		fmt.Printf("  Last Error:      %s\n", *source.LastError)
	} else {
		fmt.Println("  Last Error:      None")
	}
	fmt.Println()

	// HTTP cache headers
	if source.LastModified != nil || source.ETag != nil {
		fmt.Println("HTTP Cache:")
		if source.LastModified != nil {
			fmt.Printf("  Last-Modified:   %s\n", *source.LastModified)
		}
		if source.ETag != nil {
			fmt.Printf("  ETag:            %s\n", *source.ETag)
		}
		fmt.Println()
	}

	// Scraper config (for website sources)
	if source.ScraperConfig != nil {
		fmt.Println("Scraper Configuration:")
		fmt.Printf("  Discovery Mode:     %s\n", source.ScraperConfig.DiscoveryMode)
		if source.ScraperConfig.ListConfig != nil {
			fmt.Printf("  Article Selector:   %s\n", source.ScraperConfig.ListConfig.ArticleSelector)
			if source.ScraperConfig.ListConfig.PaginationSelector != "" {
				fmt.Printf("  Pagination:         %s\n", source.ScraperConfig.ListConfig.PaginationSelector)
			}
			fmt.Printf("  Max Pages:          %d\n", source.ScraperConfig.ListConfig.MaxPages)
		}
		fmt.Printf("  Title Selector:     %s\n", source.ScraperConfig.ArticleConfig.TitleSelector)
		fmt.Printf("  Content Selector:   %s\n", source.ScraperConfig.ArticleConfig.ContentSelector)
		if source.ScraperConfig.ArticleConfig.AuthorSelector != "" {
			fmt.Printf("  Author Selector:    %s\n", source.ScraperConfig.ArticleConfig.AuthorSelector)
		}
		if source.ScraperConfig.ArticleConfig.DateSelector != "" {
			fmt.Printf("  Date Selector:      %s\n", source.ScraperConfig.ArticleConfig.DateSelector)
		}
		fmt.Println()
	}

	// Dates
	fmt.Printf("Created:     %s\n", source.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", source.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// ID
	fmt.Printf("ID:          %s\n", source.SourceID.String())
}

func handleSourcesAdd(metadataStore *sources.SourceStore, args []string) {
	// Parse flags for add command
	fs := flag.NewFlagSet("sources add", flag.ExitOnError)
	sourceType := fs.String("type", "", "Source type (rss, atom, or website)")
	url := fs.String("url", "", "Source URL")
	name := fs.String("name", "", "Source name")
	configFile := fs.String("config", "", "Scraper config file (for website sources)")
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
	if *sourceType != "rss" && *sourceType != "atom" && *sourceType != "website" {
		fmt.Fprintf(os.Stderr, "Error: --type must be 'rss', 'atom', or 'website'\n")
		os.Exit(1)
	}

	// For website sources, config is required
	var scraperConfig *newsfed.ScraperConfig
	if *sourceType == "website" {
		if *configFile == "" {
			fmt.Fprintf(os.Stderr, "Error: --config is required for website sources\n")
			os.Exit(1)
		}

		// Read and parse config file
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read config file: %v\n", err)
			os.Exit(1)
		}

		scraperConfig = &newsfed.ScraperConfig{}
		if err := json.Unmarshal(data, scraperConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse config file: %v\n", err)
			os.Exit(1)
		}
	}

	// Create the source (enabled by default)
	now := time.Now()
	source, err := metadataStore.CreateSource(*sourceType, *url, *name, scraperConfig, &now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created source: %s\n", source.SourceID.String())
	fmt.Printf("  Type: %s\n", source.SourceType)
	fmt.Printf("  Name: %s\n", source.Name)
	fmt.Printf("  URL: %s\n", source.URL)
	if scraperConfig != nil {
		fmt.Println("  Scraper: Configured")
	}
}

func handleSourcesUpdate(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources update <source-id> [flags]\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Parse flags for update command
	fs := flag.NewFlagSet("sources update", flag.ExitOnError)
	name := fs.String("name", "", "Update source name")
	interval := fs.String("interval", "", "Update polling interval (e.g., 30m, 1h)")
	configFile := fs.String("config", "", "Update scraper config file (for website sources)")
	fs.Parse(args[1:])

	// Check if any updates were provided
	if *name == "" && *interval == "" && *configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: at least one update flag is required (--name, --interval, or --config)\n")
		os.Exit(1)
	}

	// Build updates struct
	update := sources.SourceUpdate{}

	if *name != "" {
		update.Name = name
	}

	if *interval != "" {
		// Validate interval format by parsing it
		_, err := parseDuration(*interval)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid interval format: %v\n", err)
			os.Exit(1)
		}
		update.PollingInterval = interval
	}

	if *configFile != "" {
		// Read and parse config file
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read config file: %v\n", err)
			os.Exit(1)
		}

		scraperConfig := &newsfed.ScraperConfig{}
		if err := json.Unmarshal(data, scraperConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse config file: %v\n", err)
			os.Exit(1)
		}
		update.ScraperConfig = scraperConfig
	}

	// Apply updates
	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to update source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Updated source: %s\n", sourceID)
	if *name != "" {
		fmt.Printf("  Name: %s\n", *name)
	}
	if *interval != "" {
		fmt.Printf("  Interval: %s\n", *interval)
	}
	if *configFile != "" {
		fmt.Println("  Scraper: Updated")
	}
}

func handleSourcesDelete(metadataStore *sources.SourceStore, args []string) {
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

func handleSourcesEnable(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources enable <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Check if already enabled
	if source.EnabledAt != nil {
		fmt.Printf("Source is already enabled (enabled at: %s)\n", source.EnabledAt.Format("2006-01-02 15:04:05"))
		return
	}

	// Enable the source
	now := time.Now()
	update := sources.SourceUpdate{
		EnabledAt: &now,
	}

	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to enable source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Enabled source: %s\n", source.Name)
}

func handleSourcesDisable(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources disable <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Check if already disabled
	if source.EnabledAt == nil {
		fmt.Println("Source is already disabled")
		return
	}

	// Disable the source
	update := sources.SourceUpdate{
		ClearEnabledAt: true,
	}

	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to disable source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Disabled source: %s\n", source.Name)
}
