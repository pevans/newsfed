package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
)

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
	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	// Create discovery service
	config := &discovery.DiscoveryConfig{
		FetchTimeout: 60 * time.Second,
	}
	service := discovery.NewDiscoveryService(sourceStore, newsFeed, config)

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
