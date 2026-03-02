package main

import (
	"fmt"
	"os"

	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
	"github.com/pevans/newsfed/tui"
)

func handleTUI(metadataPath, feedDir string) {
	sourceStore, err := sources.NewSourceStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open source store: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = sourceStore.Close() }()

	newsFeed, err := newsfeed.NewNewsFeed(feedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open news feed: %v\n", err)
		os.Exit(1)
	}

	discSvc := discovery.NewDiscoveryService(sourceStore, newsFeed, nil)

	if err := tui.Run(sourceStore, newsFeed, discSvc); err != nil {
		fmt.Fprintf(os.Stderr, "Error: TUI exited with error: %v\n", err)
		os.Exit(1)
	}
}
