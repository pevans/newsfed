package newsfed

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	rateLimiter     *domainRateLimiter
	metrics         *DiscoveryMetrics
}

// DiscoveryMetrics tracks service metrics per RFC 7 section 10.2.
type DiscoveryMetrics struct {
	mu                   sync.Mutex
	SourcesTotal         int             // Total enabled sources
	SourcesFetchedTotal  int             // Counter of successful fetches
	SourcesFailedTotal   int             // Counter of failed fetches
	ItemsDiscoveredTotal int             // Counter of new items added
	FetchDurations       []time.Duration // Recent fetch durations for histogram
	maxDurations         int             // Max durations to keep
}

func newDiscoveryMetrics() *DiscoveryMetrics {
	return &DiscoveryMetrics{
		FetchDurations: make([]time.Duration, 0),
		maxDurations:   1000, // Keep last 1000 durations
	}
}

func (m *DiscoveryMetrics) recordFetchSuccess(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SourcesFetchedTotal++
	m.recordDuration(duration)
}

func (m *DiscoveryMetrics) recordFetchFailure(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SourcesFailedTotal++
	m.recordDuration(duration)
}

func (m *DiscoveryMetrics) recordItemsDiscovered(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ItemsDiscoveredTotal += count
}

func (m *DiscoveryMetrics) updateSourcesTotal(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SourcesTotal = count
}

func (m *DiscoveryMetrics) recordDuration(duration time.Duration) {
	m.FetchDurations = append(m.FetchDurations, duration)
	// Keep only the most recent durations
	if len(m.FetchDurations) > m.maxDurations {
		m.FetchDurations = m.FetchDurations[len(m.FetchDurations)-m.maxDurations:]
	}
}

// GetMetrics returns a copy of current metrics (thread-safe).
func (m *DiscoveryMetrics) GetMetrics() (sourcesTotal, sourcesFetched, sourcesFailed, itemsDiscovered int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SourcesTotal, m.SourcesFetchedTotal, m.SourcesFailedTotal, m.ItemsDiscoveredTotal
}

// domainRateLimiter implements per-domain rate limiting per RFC 7 section
// 8.2.
type domainRateLimiter struct {
	mu              sync.Mutex
	lastRequestTime map[string]time.Time
	minInterval     time.Duration
}

func newDomainRateLimiter(minInterval time.Duration) *domainRateLimiter {
	return &domainRateLimiter{
		lastRequestTime: make(map[string]time.Time),
		minInterval:     minInterval,
	}
}

// wait blocks until it's safe to make a request to the given domain.
func (rl *domainRateLimiter) wait(domain string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if lastTime, ok := rl.lastRequestTime[domain]; ok {
		elapsed := time.Since(lastTime)
		if elapsed < rl.minInterval {
			time.Sleep(rl.minInterval - elapsed)
		}
	}

	rl.lastRequestTime[domain] = time.Now()
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
		rateLimiter:     newDomainRateLimiter(1 * time.Second), // 1 req/sec per domain (RFC 7 section 8.2)
		metrics:         newDiscoveryMetrics(),
	}
}

// GetMetrics returns the current metrics for monitoring.
func (ds *DiscoveryService) GetMetrics() *DiscoveryMetrics {
	return ds.metrics
}

// Run starts the discovery service loop. It runs until Stop() is called or
// the context is cancelled.
func (ds *DiscoveryService) Run(ctx context.Context) error {
	log.Println("INFO: Discovery service starting")

	// Fetch sources immediately on startup per RFC 7 section 3.3
	if err := ds.fetchSources(ctx); err != nil {
		log.Printf("ERROR: Initial source fetch failed: %v", err)
	}

	// Start polling loop
	ticker := time.NewTicker(5 * time.Minute) // Check for due sources every 5 minutes
	defer ticker.Stop()

	// Start metrics logging
	metricsTicker := time.NewTicker(15 * time.Minute) // Log metrics every 15 minutes
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("INFO: Discovery service stopping (context cancelled)")
			ds.logMetrics()
			ds.wg.Wait() // Wait for in-progress fetches to complete
			return ctx.Err()
		case <-ds.stopChan:
			log.Println("INFO: Discovery service stopping")
			ds.logMetrics()
			ds.wg.Wait() // Wait for in-progress fetches to complete
			return nil
		case <-ticker.C:
			if err := ds.fetchSources(ctx); err != nil {
				log.Printf("ERROR: Source fetch failed: %v", err)
			}
		case <-metricsTicker.C:
			ds.logMetrics()
		}
	}
}

// logMetrics logs current metrics per RFC 7 section 10.2.
func (ds *DiscoveryService) logMetrics() {
	sourcesTotal, sourcesFetched, sourcesFailed, itemsDiscovered := ds.metrics.GetMetrics()
	log.Printf("INFO: Metrics - Sources: %d enabled, Fetches: %d success / %d failed, Items discovered: %d",
		sourcesTotal, sourcesFetched, sourcesFailed, itemsDiscovered)
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

	// Update metrics with total enabled sources
	enabledCount := 0
	for _, s := range sources {
		if s.EnabledAt != nil {
			enabledCount++
		}
	}
	ds.metrics.updateSourcesTotal(enabledCount)

	// Filter for enabled sources that are due
	dueSources := ds.filterDueSources(sources)
	if len(dueSources) == 0 {
		return nil
	}

	log.Printf("INFO: Fetching %d due sources (of %d enabled)", len(dueSources), enabledCount)

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
		newItemCount, err = ds.fetchWebsite(fetchCtx, source)
	default:
		return fmt.Errorf("unsupported source type: %s", source.SourceType)
	}

	duration := time.Since(startTime)

	// Update source metadata
	if err != nil {
		ds.handleFetchError(source, err)
		ds.metrics.recordFetchFailure(duration)
		return err
	}

	// Success -- update metadata and metrics
	ds.handleFetchSuccess(source)
	ds.metrics.recordFetchSuccess(duration)
	ds.metrics.recordItemsDiscovered(newItemCount)

	// Log success per RFC 7 section 10.1
	if duration > 30*time.Second {
		log.Printf("WARN: Slow fetch for %s (%s): %d new items in %v", source.Name, source.URL, newItemCount, duration)
	} else {
		log.Printf("INFO: Fetched %s (%s): %d new items in %v", source.Name, source.URL, newItemCount, duration)
	}

	return nil
}

// shouldApplyItemLimit determines whether to apply the 20-item limit based on
// source staleness. Per RFC 2 section 2.2.3 and RFC 3 section 3.1.1, the
// limit applies when:
// - First-time sync: source has never been fetched (last_fetched_at is nil)
// - Stale source: source has not been synced for more than 15 days
func (ds *DiscoveryService) shouldApplyItemLimit(source Source) bool {
	// First-time sync -- never fetched before
	if source.LastFetchedAt == nil {
		return true
	}

	// Stale source -- not synced for more than 15 days
	staleDuration := 15 * 24 * time.Hour
	timeSinceLastFetch := time.Since(*source.LastFetchedAt)
	if timeSinceLastFetch > staleDuration {
		return true
	}

	// Regular polling -- don't apply limit
	return false
}

// fetchRSSFeed fetches and processes an RSS or Atom feed. Implements RFC 7
// section 4 with conditional 20-item limit per RFC 2 section 2.2.3.
func (ds *DiscoveryService) fetchRSSFeed(_ context.Context, source Source) (int, error) {
	// Fetch the feed (FetchFeed from RFC 2)
	feed, err := FetchFeed(source.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch feed: %w", err)
	}

	// Determine if we should apply the 20-item limit (RFC 2 section 2.2.3)
	// Limit applies for:
	// 1. First-time sync (last_fetched_at is nil)
	// 2. Stale sources (not synced for >15 days)
	applyLimit := ds.shouldApplyItemLimit(source)

	// Convert feed items to NewsItems (FeedToNewsItems from RFC 2)
	newsItems := FeedToNewsItems(feed, applyLimit)

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

// fetchWebsite fetches and processes a website source. Implements RFC 7
// section 5.
func (ds *DiscoveryService) fetchWebsite(_ context.Context, source Source) (int, error) {
	if source.ScraperConfig == nil {
		return 0, fmt.Errorf("scraper config is required for website sources")
	}

	config := source.ScraperConfig

	// Get domain for rate limiting
	domain, err := ds.extractDomain(source.URL)
	if err != nil {
		return 0, fmt.Errorf("invalid source URL: %w", err)
	}

	switch config.DiscoveryMode {
	case "direct":
		return ds.fetchDirectMode(source, config, domain)
	case "list":
		return ds.fetchListMode(source, config, domain)
	default:
		return 0, fmt.Errorf("unsupported discovery mode: %s", config.DiscoveryMode)
	}
}

// fetchDirectMode fetches a single article page directly. Implements RFC 7
// section 5.1.1.
func (ds *DiscoveryService) fetchDirectMode(source Source, config *ScraperConfig, domain string) (int, error) {
	// Rate limit before fetching
	ds.rateLimiter.wait(domain)

	// Scrape the article
	article, err := ScrapeArticle(source.URL, config.ArticleConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to scrape article: %w", err)
	}

	// Validate the article
	if err := ValidateScrapedArticle(article, source.URL); err != nil {
		// Validation errors don't count as fetch failures per RFC 7 section
		// 7.4
		log.Printf("WARN: Validation failed for %s: %v", source.URL, err)
		return 0, nil
	}

	// Check for duplicates
	exists, err := URLExists(ds.newsFeed, article.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to check URL existence: %w", err)
	}

	if exists {
		// Already have this article
		return 0, nil
	}

	// Convert to NewsItem
	newsItem := ScrapedArticleToNewsItem(article, source.Name)

	// Add to feed
	if err := ds.newsFeed.Add(newsItem); err != nil {
		return 0, fmt.Errorf("failed to add item: %w", err)
	}

	return 1, nil
}

// fetchListMode fetches articles from a list/index page. Implements RFC 7
// section 5.1.2 with conditional 20-article cap per RFC 3 section 3.1.1.
func (ds *DiscoveryService) fetchListMode(source Source, config *ScraperConfig, domain string) (int, error) {
	if config.ListConfig == nil {
		return 0, fmt.Errorf("list_config is required for list mode")
	}

	listConfig := config.ListConfig
	newItemCount := 0
	currentURL := source.URL
	pagesProcessed := 0
	articlesCollected := 0 // Track total articles collected across all pages

	// Determine if we should apply the 20-article limit (RFC 3 section 3.1.1)
	// Limit applies for first-time sync or stale sources (>15 days)
	applyLimit := ds.shouldApplyItemLimit(source)
	const maxArticles = 20 // RFC 3 section 3.1.1

	for {
		// Enforce max pages limit
		if pagesProcessed >= listConfig.MaxPages {
			break
		}

		// Conditionally enforce max articles limit per RFC 3 section 3.1.1
		// Only apply for first-time syncs or stale sources
		if applyLimit && articlesCollected >= maxArticles {
			break
		}

		// Rate limit before fetching
		ds.rateLimiter.wait(domain)

		// Fetch the list page
		doc, err := FetchHTML(currentURL)
		if err != nil {
			return newItemCount, fmt.Errorf("failed to fetch list page: %w", err)
		}

		// Extract article URLs
		articleURLs := ds.extractArticleURLs(doc, listConfig.ArticleSelector, currentURL)
		if len(articleURLs) == 0 {
			log.Printf("WARN: No articles found on list page %s", currentURL)
			break
		}

		// Conditionally limit article URLs to not exceed max (RFC 3 section
		// 3.1.1) Only apply limit for first-time syncs or stale sources
		if applyLimit {
			remainingSlots := maxArticles - articlesCollected
			if len(articleURLs) > remainingSlots {
				articleURLs = articleURLs[:remainingSlots]
			}
		}

		// Process each article URL
		for _, articleURL := range articleURLs {
			// Only increment counter if limit is being applied
			if applyLimit {
				articlesCollected++
			}

			// Check if URL already exists (deduplication)
			exists, err := URLExists(ds.newsFeed, articleURL)
			if err != nil {
				log.Printf("WARN: Failed to check URL existence for %s: %v", articleURL, err)
				continue
			}

			if exists {
				// Skip duplicate
				continue
			}

			// Rate limit before fetching article
			ds.rateLimiter.wait(domain)

			// Scrape the article
			article, err := ScrapeArticle(articleURL, config.ArticleConfig)
			if err != nil {
				log.Printf("WARN: Failed to scrape article %s: %v", articleURL, err)
				continue
			}

			// Validate the article
			if err := ValidateScrapedArticle(article, source.URL); err != nil {
				log.Printf("WARN: Validation failed for %s: %v", articleURL, err)
				continue
			}

			// Convert to NewsItem
			newsItem := ScrapedArticleToNewsItem(article, source.Name)

			// Add to feed
			if err := ds.newsFeed.Add(newsItem); err != nil {
				log.Printf("WARN: Failed to add item %s: %v", articleURL, err)
				continue
			}

			newItemCount++
		}

		pagesProcessed++

		// Stop if we've reached the article limit (only if limit is being
		// applied)
		if applyLimit && articlesCollected >= maxArticles {
			break
		}

		// Check for pagination
		if listConfig.PaginationSelector == "" {
			break
		}

		// Extract next page URL
		nextURL := ds.extractNextPageURL(doc, listConfig.PaginationSelector, currentURL)
		if nextURL == "" {
			// No more pages
			break
		}

		currentURL = nextURL
	}

	return newItemCount, nil
}

// extractArticleURLs extracts article URLs from a list page.
func (ds *DiscoveryService) extractArticleURLs(doc *goquery.Document, selector string, baseURL string) []string {
	var urls []string

	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Resolve relative URLs
		absoluteURL, err := ds.resolveURL(baseURL, href)
		if err != nil {
			log.Printf("WARN: Failed to resolve URL %s: %v", href, err)
			return
		}

		urls = append(urls, absoluteURL)
	})

	return urls
}

// extractNextPageURL extracts the next page URL from pagination.
func (ds *DiscoveryService) extractNextPageURL(doc *goquery.Document, selector string, baseURL string) string {
	nextHref, exists := doc.Find(selector).First().Attr("href")
	if !exists {
		return ""
	}

	// Resolve relative URLs
	absoluteURL, err := ds.resolveURL(baseURL, nextHref)
	if err != nil {
		log.Printf("WARN: Failed to resolve pagination URL %s: %v", nextHref, err)
		return ""
	}

	return absoluteURL
}

// resolveURL resolves a potentially relative URL against a base URL.
func (ds *DiscoveryService) resolveURL(baseURL, href string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(ref).String(), nil
}

// extractDomain extracts the domain from a URL for rate limiting.
func (ds *DiscoveryService) extractDomain(urlStr string) (string, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
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
		// Transient errors -- increment counter and check threshold (RFC 7
		// section 7.1 and 7.3)
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

// SyncResult contains the results of a manual sync operation.
type SyncResult struct {
	SourcesSynced   int
	SourcesFailed   int
	ItemsDiscovered int
	Errors          []SyncError
}

// SyncError contains details about a source sync failure.
type SyncError struct {
	Source Source
	Error  error
}

// SyncSources performs a manual sync of sources. If sourceID is provided,
// only that source is synced. Otherwise, all enabled sources are synced. This
// is a synchronous operation that returns when all fetches complete.
func (ds *DiscoveryService) SyncSources(ctx context.Context, sourceID *uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{
		Errors: make([]SyncError, 0),
	}
	var resultMu sync.Mutex

	var sources []Source

	if sourceID != nil {
		// Sync single source
		source, err := ds.metadataStore.GetSource(*sourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get source: %w", err)
		}
		sources = []Source{*source}
	} else {
		// Sync all enabled sources
		allSources, err := ds.metadataStore.ListSources()
		if err != nil {
			return nil, fmt.Errorf("failed to list sources: %w", err)
		}

		// Filter to only enabled sources
		for _, source := range allSources {
			if source.EnabledAt != nil {
				sources = append(sources, source)
			}
		}
	}

	if len(sources) == 0 {
		return result, nil
	}

	// Use a concurrency limit (default to 5 concurrent fetches)
	concurrency := 5
	if ds.config.Concurrency > 0 {
		concurrency = ds.config.Concurrency
	}
	semaphore := make(chan struct{}, concurrency)

	// Fetch sources concurrently with WaitGroup
	var wg sync.WaitGroup

	for _, source := range sources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case semaphore <- struct{}{}: // Acquire semaphore
			wg.Add(1)
			go func(s Source) {
				defer wg.Done()
				defer func() { <-semaphore }() // Release semaphore

				startTime := time.Now()

				// Create context with timeout
				fetchCtx, cancel := context.WithTimeout(ctx, ds.config.FetchTimeout)
				defer cancel()

				// Process based on source type
				var newItemCount int
				var fetchErr error

				switch s.SourceType {
				case "rss", "atom":
					newItemCount, fetchErr = ds.fetchRSSFeed(fetchCtx, s)
				case "website":
					newItemCount, fetchErr = ds.fetchWebsite(fetchCtx, s)
				default:
					fetchErr = fmt.Errorf("unsupported source type: %s", s.SourceType)
				}

				duration := time.Since(startTime)

				// Update source metadata and results (with mutex protection)
				resultMu.Lock()
				if fetchErr != nil {
					ds.handleFetchError(s, fetchErr)
					result.SourcesFailed++
					result.Errors = append(result.Errors, SyncError{
						Source: s,
						Error:  fetchErr,
					})
					log.Printf("ERROR: Failed to sync %s (%s): %v", s.Name, s.URL, fetchErr)
				} else {
					// Success -- update metadata
					ds.handleFetchSuccess(s)
					result.SourcesSynced++
					result.ItemsDiscovered += newItemCount
					log.Printf("INFO: Synced %s (%s): %d new items in %v", s.Name, s.URL, newItemCount, duration)
				}
				resultMu.Unlock()
			}(source)
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return result, nil
}
