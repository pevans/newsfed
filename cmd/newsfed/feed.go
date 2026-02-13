package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed/config"
	"github.com/pevans/newsfed/newsfeed"
)

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
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	// Get all items
	result, err := newsFeed.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list news items: %v\n", err)
		os.Exit(1)
	}

	// Report any partial failures after displaying results
	defer func() {
		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "\nWarning: %d item(s) could not be read:\n", len(result.Errors))
			for _, readErr := range result.Errors {
				fmt.Fprintf(os.Stderr, "  %s\n", readErr.Error())
			}
		}
	}()

	// Apply filters
	var filtered []newsfeed.NewsItem
	for _, item := range result.Items {
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
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
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
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
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
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
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

func handleOpen(metadataPath, feedDir string, args []string) {
	// Parse flags for open command
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	echo := fs.Bool("echo", false, "Echo the command instead of executing it")
	fs.Parse(args)

	// Get item ID from remaining args
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: item ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed open [-echo] <item-id>\n")
		os.Exit(1)
	}

	itemID := fs.Args()[0]

	// Parse UUID
	id, err := uuid.Parse(itemID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid item ID: %v\n", err)
		os.Exit(1)
	}

	// Initialize news feed
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
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

	// Load config to check for custom browser command
	var browserCmd string
	var cmdArgs []string

	configStore, err := config.NewConfigStore(metadataPath)
	if err == nil {
		defer configStore.Close()
		cfg, err := configStore.GetConfig()
		if err == nil && cfg.BrowserCommand != "" {
			// Use configured browser command
			browserCmd = cfg.BrowserCommand
			cmdArgs = []string{item.URL}
		}
	}

	// If no configured browser command, determine based on platform
	if browserCmd == "" {
		switch runtime.GOOS {
		case "darwin":
			browserCmd = "open"
			cmdArgs = []string{item.URL}
		case "linux":
			browserCmd = "xdg-open"
			cmdArgs = []string{item.URL}
		case "windows":
			browserCmd = "cmd"
			cmdArgs = []string{"/c", "start", item.URL}
		default:
			fmt.Fprintf(os.Stderr, "Error: unsupported platform: %s\n", runtime.GOOS)
			os.Exit(1)
		}
	}

	// If echo mode, print the command instead of executing it
	if *echo {
		fmt.Printf("%s %s\n", browserCmd, strings.Join(cmdArgs, " "))
		return
	}

	// Build and execute the command
	cmd := exec.Command(browserCmd, cmdArgs...)

	err = cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open URL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Opening in browser: %s\n", item.Title)
}
