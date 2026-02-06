package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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

// getEnvDuration parses a duration from environment variable or returns default.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvInt parses an int from environment variable or returns default.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func main() {
	// Parse command line flags with environment variable defaults per RFC 7
	// section 9.1
	metadataPath := flag.String("metadata", getEnv("NEWSFED_METADATA_DSN", "metadata.db"), "Path to metadata database (NEWSFED_METADATA_DSN)")
	feedDir := flag.String("feed", getEnv("NEWSFED_FEED_DSN", ".news"), "Path to news feed storage directory (NEWSFED_FEED_DSN)")
	pollInterval := flag.Duration("poll-interval", getEnvDuration("NEWSFED_POLL_INTERVAL", 1*time.Hour), "Default polling interval for sources (NEWSFED_POLL_INTERVAL)")
	concurrency := flag.Int("concurrency", getEnvInt("NEWSFED_CONCURRENCY", 5), "Maximum number of parallel source fetches (NEWSFED_CONCURRENCY)")
	fetchTimeout := flag.Duration("fetch-timeout", getEnvDuration("NEWSFED_FETCH_TIMEOUT", 60*time.Second), "Timeout per source fetch (NEWSFED_FETCH_TIMEOUT)")
	disableThreshold := flag.Int("disable-threshold", getEnvInt("NEWSFED_DISABLE_THRESHOLD", 10), "Auto-disable source after N consecutive failures (NEWSFED_DISABLE_THRESHOLD)")

	flag.Parse()

	// Initialize source store
	log.Printf("Opening source store: %s", *metadataPath)
	sourceStore, err := sources.NewSourceStore(*metadataPath)
	if err != nil {
		log.Fatalf("Failed to open source store: %v", err)
	}
	defer sourceStore.Close()

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
	service := newsfed.NewDiscoveryService(sourceStore, newsFeed, config)

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
