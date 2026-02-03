package newsfed

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DiscoveryService is a background service that automatically discovers and
// ingests news items from configured sources. Implements RFC 7.
type DiscoveryService struct {
	metadataStore   *MetadataStore
	newsFeed        *NewsFeed
	config          *DiscoveryConfig
	httpClient      *http.Client
	stopChan        chan struct{}
	wg              sync.WaitGroup
	sourceSemaphore chan struct{}
}

// DiscoveryConfig holds configuration for the discovery service.
type DiscoveryConfig struct {
	// Global polling interval for sources without explicit interval
	PollInterval time.Duration
	// Maximum number of sources to fetch in parallel
	Concurrency int
	// Timeout per source fetch
	FetchTimeout time.Duration
	// Number of consecutive failures before auto-disabling a source
	DisableThreshold int
}

// DefaultDiscoveryConfig returns the default configuration per RFC 7 section
// 9.1.2.
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		PollInterval:     1 * time.Hour,
		Concurrency:      5,
		FetchTimeout:     60 * time.Second,
		DisableThreshold: 10,
	}
}

// NewDiscoveryService creates a new discovery service.
func NewDiscoveryService(
	metadataStore *MetadataStore,
	newsFeed *NewsFeed,
	config *DiscoveryConfig,
) *DiscoveryService {
	if config == nil {
		config = DefaultDiscoveryConfig()
	}

	return &DiscoveryService{
		metadataStore: metadataStore,
		newsFeed:      newsFeed,
		config:        config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Per RFC 3 section 3.2
		},
		stopChan:        make(chan struct{}),
		sourceSemaphore: make(chan struct{}, config.Concurrency),
	}
}

// Run starts the discovery service loop. It runs until Stop() is called or
// the context is cancelled.
func (ds *DiscoveryService) Run(ctx context.Context) error {
	log.Println("Discovery service starting")

	// Fetch sources immediately on startup per RFC 7 section 3.3
	if err := ds.fetchSources(ctx); err != nil {
		log.Printf("ERROR: Initial source fetch failed: %v", err)
	}

	// Start polling loop
	ticker := time.NewTicker(5 * time.Minute) // Check for due sources every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Discovery service stopping (context cancelled)")
			ds.wg.Wait() // Wait for in-progress fetches to complete
			return ctx.Err()
		case <-ds.stopChan:
			log.Println("Discovery service stopping")
			ds.wg.Wait() // Wait for in-progress fetches to complete
			return nil
		case <-ticker.C:
			if err := ds.fetchSources(ctx); err != nil {
				log.Printf("ERROR: Source fetch failed: %v", err)
			}
		}
	}
}

// Stop signals the discovery service to stop gracefully.
func (ds *DiscoveryService) Stop() {
	close(ds.stopChan)
}

// fetchSources fetches all sources that are due for polling.
func (ds *DiscoveryService) fetchSources(ctx context.Context) error {
	// Get all sources from metadata store
	sources, err := ds.metadataStore.ListSources()
	if err != nil {
		return fmt.Errorf("failed to list sources: %w", err)
	}

	// Filter for enabled sources that are due
	dueSources := ds.filterDueSources(sources)
	if len(dueSources) == 0 {
		return nil
	}

	log.Printf("INFO: Fetching %d due sources", len(dueSources))

	// Fetch sources in parallel with concurrency limit
	for _, source := range dueSources {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ds.sourceSemaphore <- struct{}{}: // Acquire semaphore
			ds.wg.Add(1)
			go func(s Source) {
				defer ds.wg.Done()
				defer func() { <-ds.sourceSemaphore }() // Release semaphore

				if err := ds.fetchSource(ctx, s); err != nil {
					log.Printf("ERROR: Failed to fetch source %s (%s): %v", s.Name, s.URL, err)
				}
			}(source)
		}
	}

	return nil
}

// filterDueSources returns sources that are enabled and due for fetching.
// Implements RFC 7 section 3.2 and 3.3.
func (ds *DiscoveryService) filterDueSources(sources []Source) []Source {
	now := time.Now()
	var dueSources []Source

	for _, source := range sources {
		// Skip disabled sources (enabled_at is nil)
		if source.EnabledAt == nil {
			continue
		}

		// Get polling interval for this source
		interval := ds.getPollingInterval(source)

		// Check if source is due
		if ds.isSourceDue(source, interval, now) {
			dueSources = append(dueSources, source)
		}
	}

	return dueSources
}

// getPollingInterval returns the polling interval for a source. Uses the
// source's specific interval if set, otherwise uses the global default.
// Implements RFC 7 section 3.1.
func (ds *DiscoveryService) getPollingInterval(source Source) time.Duration {
	if source.PollingInterval != nil {
		interval, err := time.ParseDuration(*source.PollingInterval)
		if err == nil {
			// Enforce minimum polling interval of 5 minutes per RFC 7 section
			// 3.1
			if interval < 5*time.Minute {
				interval = 5 * time.Minute
			}
			// Enforce maximum polling interval of 24 hours per RFC 7 section
			// 3.1
			if interval > 24*time.Hour {
				interval = 24 * time.Hour
			}
			return interval
		}
	}
	return ds.config.PollInterval
}

// isSourceDue checks if a source is due for fetching based on its last fetch
// time and polling interval. Implements RFC 7 section 3.2 and 3.3.
func (ds *DiscoveryService) isSourceDue(source Source, interval time.Duration, now time.Time) bool {
	// Never fetched -- fetch immediately per RFC 7 section 3.3
	if source.LastFetchedAt == nil {
		return true
	}

	// Calculate next fetch time
	nextFetchAt := source.LastFetchedAt.Add(interval)

	// Overdue or due now
	return now.After(nextFetchAt) || now.Equal(nextFetchAt)
}

// fetchSource fetches a single source and processes its items. Implements RFC
// 7 section 4 for RSS/Atom feeds.
func (ds *DiscoveryService) fetchSource(ctx context.Context, source Source) error {
	startTime := time.Now()

	// Create context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, ds.config.FetchTimeout)
	defer cancel()

	// Process based on source type
	var newItemCount int
	var err error

	switch source.SourceType {
	case "rss", "atom":
		newItemCount, err = ds.fetchRSSFeed(fetchCtx, source)
	case "website":
		// Web scraping will be implemented in later sections
		return fmt.Errorf("website scraping not yet implemented")
	default:
		return fmt.Errorf("unsupported source type: %s", source.SourceType)
	}

	duration := time.Since(startTime)

	// Update source metadata
	if err != nil {
		ds.handleFetchError(source, err)
		return err
	}

	// Success -- update metadata
	ds.handleFetchSuccess(source)

	// Log success
	if duration > 30*time.Second {
		log.Printf("WARN: Slow fetch for %s (%s): %d new items in %v", source.Name, source.URL, newItemCount, duration)
	} else {
		log.Printf("INFO: Fetched %s (%s): %d new items in %v", source.Name, source.URL, newItemCount, duration)
	}

	return nil
}

// fetchRSSFeed fetches and processes an RSS or Atom feed. Implements RFC 7
// section 4.
func (ds *DiscoveryService) fetchRSSFeed(_ context.Context, source Source) (int, error) {
	// Fetch the feed (FetchFeed from RFC 2)
	feed, err := FetchFeed(source.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch feed: %w", err)
	}

	// Convert feed items to NewsItems (FeedToNewsItems from RFC 2)
	newsItems := FeedToNewsItems(feed)

	// Process each item with deduplication
	newItemCount := 0
	for _, item := range newsItems {
		// Check if URL already exists (RFC 7 section 4.2)
		exists, err := URLExists(ds.newsFeed, item.URL)
		if err != nil {
			log.Printf("WARN: Failed to check URL existence for %s: %v", item.URL, err)
			continue
		}

		if exists {
			// Skip duplicate
			continue
		}

		// Add new item to feed
		if err := ds.newsFeed.Add(item); err != nil {
			log.Printf("WARN: Failed to add item %s: %v", item.URL, err)
			continue
		}

		newItemCount++
	}

	return newItemCount, nil
}

// handleFetchSuccess updates source metadata after a successful fetch.
// Implements RFC 7 section 4.3.
func (ds *DiscoveryService) handleFetchSuccess(source Source) {
	now := time.Now()
	updates := map[string]any{
		"last_fetched_at":   &now,
		"fetch_error_count": 0,
		"last_error":        (*string)(nil),
	}

	if err := ds.metadataStore.UpdateSource(source.SourceID, updates); err != nil {
		log.Printf("ERROR: Failed to update source metadata for %s: %v", source.Name, err)
	}
}

// handleFetchError updates source metadata after a fetch error. Implements
// RFC 7 section 7 (Error Handling).
func (ds *DiscoveryService) handleFetchError(source Source, fetchErr error) {
	now := time.Now()
	errorMsg := fetchErr.Error()

	// Determine if error is permanent or transient
	isPermanent := ds.isPermanentError(fetchErr)

	updates := map[string]any{
		"last_fetched_at": &now,
		"last_error":      &errorMsg,
	}

	if isPermanent {
		// Permanent errors -- disable immediately (RFC 7 section 7.2)
		log.Printf("ERROR: Disabling source %s (%s) due to permanent error: %v", source.Name, source.URL, fetchErr)
		updates["enabled_at"] = (*time.Time)(nil)
		updates["fetch_error_count"] = source.FetchErrorCount + 1
	} else {
		// Transient errors -- increment counter and check threshold (RFC 7 section 7.1 and 7.3)
		newErrorCount := source.FetchErrorCount + 1
		updates["fetch_error_count"] = newErrorCount

		if newErrorCount >= ds.config.DisableThreshold {
			log.Printf("ERROR: Auto-disabling source %s (%s) after %d consecutive failures", source.Name, source.URL, newErrorCount)
			updates["enabled_at"] = (*time.Time)(nil)
		}
	}

	if err := ds.metadataStore.UpdateSource(source.SourceID, updates); err != nil {
		log.Printf("ERROR: Failed to update source metadata for %s: %v", source.Name, err)
	}
}

// isPermanentError determines if an error is permanent (requiring immediate
// disable) or transient (retryable). Implements RFC 7 section 7.1 and 7.2.
func (ds *DiscoveryService) isPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// HTTP 404 Not Found
	if strings.Contains(errMsg, "404") || strings.Contains(errMsg, "not found") {
		return true
	}

	// HTTP 410 Gone
	if strings.Contains(errMsg, "410") || strings.Contains(errMsg, "gone") {
		return true
	}

	// Invalid feed format
	if strings.Contains(errMsg, "failed to parse") || strings.Contains(errMsg, "invalid feed") {
		return true
	}

	// Invalid HTML/XML
	if strings.Contains(errMsg, "failed to parse html") || strings.Contains(errMsg, "invalid html") {
		return true
	}

	// DNS errors (domain doesn't exist)
	if strings.Contains(errMsg, "no such host") {
		return true
	}

	// Invalid URL
	if strings.Contains(errMsg, "invalid url") || strings.Contains(errMsg, "unsupported protocol") {
		return true
	}

	// Check for wrapped errors
	var unwrapped error
	if errors.Unwrap(err) != nil {
		unwrapped = errors.Unwrap(err)
		return ds.isPermanentError(unwrapped)
	}

	// Default to transient (network timeouts, temporary server errors, etc.)
	return false
}

// UpdateSourceFetchMetadata updates the fetch metadata for a source. This is
// a helper function for external use.
func (ds *DiscoveryService) UpdateSourceFetchMetadata(
	sourceID uuid.UUID,
	lastModified, etag *string,
) error {
	updates := map[string]any{}
	if lastModified != nil {
		updates["last_modified"] = lastModified
	}
	if etag != nil {
		updates["etag"] = etag
	}

	if len(updates) == 0 {
		return nil
	}

	return ds.metadataStore.UpdateSource(sourceID, updates)
}
