package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pevans/newsfed"
)

func main() {
	// Parse command line flags
	metadataPath := flag.String("metadata", "metadata.db", "Path to metadata database")
	feedDir := flag.String("feed", ".news", "Path to news feed storage directory")
	pollInterval := flag.Duration("poll-interval", 1*time.Hour, "Default polling interval for sources")
	concurrency := flag.Int("concurrency", 5, "Maximum number of parallel source fetches")
	fetchTimeout := flag.Duration("fetch-timeout", 60*time.Second, "Timeout per source fetch")
	disableThreshold := flag.Int("disable-threshold", 10, "Auto-disable source after N consecutive failures")

	flag.Parse()

	// Initialize metadata store
	log.Printf("Opening metadata store: %s", *metadataPath)
	metadataStore, err := newsfed.NewMetadataStore(*metadataPath)
	if err != nil {
		log.Fatalf("Failed to open metadata store: %v", err)
	}
	defer metadataStore.Close()

	// Initialize news feed
	log.Printf("Opening news feed: %s", *feedDir)
	newsFeed, err := newsfed.NewNewsFeed(*feedDir)
	if err != nil {
		log.Fatalf("Failed to open news feed: %v", err)
	}

	// Create discovery service configuration
	config := &newsfed.DiscoveryConfig{
		PollInterval:     *pollInterval,
		Concurrency:      *concurrency,
		FetchTimeout:     *fetchTimeout,
		DisableThreshold: *disableThreshold,
	}

	// Create discovery service
	service := newsfed.NewDiscoveryService(metadataStore, newsFeed, config)

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Start service in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- service.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		if sig == syscall.SIGHUP {
			// SIGHUP: reload sources (future enhancement)
			log.Println("SIGHUP received (reload not yet implemented)")
		} else {
			// SIGTERM/SIGINT: graceful shutdown
			log.Println("Shutting down gracefully...")
			cancel()
			service.Stop()

			// Wait for shutdown with timeout
			shutdownTimer := time.NewTimer(60 * time.Second)
			select {
			case <-errChan:
				log.Println("Service stopped")
			case <-shutdownTimer.C:
				log.Println("Shutdown timeout exceeded, forcing exit")
			}
		}
	case err := <-errChan:
		if err != nil {
			log.Fatalf("Service error: %v", err)
		}
	}
}
