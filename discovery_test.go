package newsfed

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscoveryService_filterDueSources verifies source scheduling logic per
// RFC 7 section 3.2.
func TestDiscoveryService_filterDueSources(t *testing.T) {
	// Create temporary storage
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	config := DefaultDiscoveryConfig()
	config.PollInterval = 1 * time.Hour
	service := NewDiscoveryService(metadataStore, newsFeed, config)

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	thirtyMinutesAgo := now.Add(-30 * time.Minute)

	// Create test sources
	sources := []Source{
		{
			// Never fetched -- should be due
			EnabledAt:     &now,
			LastFetchedAt: nil,
		},
		{
			// Fetched 1 hour ago, 1 hour interval -- should be due
			EnabledAt:     &now,
			LastFetchedAt: &oneHourAgo,
		},
		{
			// Fetched 30 minutes ago, 1 hour interval -- should NOT be due
			EnabledAt:     &now,
			LastFetchedAt: &thirtyMinutesAgo,
		},
		{
			// Disabled source -- should NOT be due
			EnabledAt:     nil,
			LastFetchedAt: nil,
		},
	}

	dueSources := service.filterDueSources(sources)

	// Should have 2 due sources: never fetched and overdue
	assert.Equal(t, 2, len(dueSources))
}

// TestDiscoveryService_getPollingInterval verifies polling interval logic per
// RFC 7 section 3.1.
func TestDiscoveryService_getPollingInterval(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	config := DefaultDiscoveryConfig()
	config.PollInterval = 1 * time.Hour
	service := NewDiscoveryService(metadataStore, newsFeed, config)

	tests := []struct {
		name             string
		pollingInterval  *string
		expectedInterval time.Duration
	}{
		{
			name:             "no interval specified -- uses default",
			pollingInterval:  nil,
			expectedInterval: 1 * time.Hour,
		},
		{
			name:             "valid interval specified",
			pollingInterval:  strPtr("30m"),
			expectedInterval: 30 * time.Minute,
		},
		{
			name:             "below minimum -- clamped to 5 minutes",
			pollingInterval:  strPtr("2m"),
			expectedInterval: 5 * time.Minute,
		},
		{
			name:             "above maximum -- clamped to 24 hours",
			pollingInterval:  strPtr("48h"),
			expectedInterval: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := Source{
				PollingInterval: tt.pollingInterval,
			}
			interval := service.getPollingInterval(source)
			assert.Equal(t, tt.expectedInterval, interval)
		})
	}
}

// TestDiscoveryService_isSourceDue verifies due checking logic per RFC 7
// section 3.2 and 3.3.
func TestDiscoveryService_isSourceDue(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	now := time.Now()
	interval := 1 * time.Hour

	tests := []struct {
		name            string
		lastFetchedAt   *time.Time
		expectedDue     bool
		expectedMessage string
	}{
		{
			name:            "never fetched",
			lastFetchedAt:   nil,
			expectedDue:     true,
			expectedMessage: "sources never fetched should be due",
		},
		{
			name:            "fetched long ago",
			lastFetchedAt:   timePtr(now.Add(-2 * time.Hour)),
			expectedDue:     true,
			expectedMessage: "sources fetched longer than interval ago should be due",
		},
		{
			name:            "fetched recently",
			lastFetchedAt:   timePtr(now.Add(-30 * time.Minute)),
			expectedDue:     false,
			expectedMessage: "sources fetched within interval should not be due",
		},
		{
			name:            "fetched exactly at interval",
			lastFetchedAt:   timePtr(now.Add(-1 * time.Hour)),
			expectedDue:     true,
			expectedMessage: "sources fetched exactly interval ago should be due",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := Source{
				LastFetchedAt: tt.lastFetchedAt,
			}
			isDue := service.isSourceDue(source, interval, now)
			assert.Equal(t, tt.expectedDue, isDue, tt.expectedMessage)
		})
	}
}

// TestDiscoveryService_handleFetchError verifies error handling per RFC 7
// section 7.
func TestDiscoveryService_handleFetchError(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	config := DefaultDiscoveryConfig()
	config.DisableThreshold = 3
	service := NewDiscoveryService(metadataStore, newsFeed, config)

	// Create a test source
	now := time.Now()
	source, err := metadataStore.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	// Simulate failures
	testErr := assert.AnError

	// First failure -- should increment error count but not disable
	service.handleFetchError(*source, testErr)
	updated, err := metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.FetchErrorCount)
	assert.NotNil(t, updated.EnabledAt, "source should still be enabled after 1 failure")

	// Second failure
	service.handleFetchError(*updated, testErr)
	updated, err = metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.FetchErrorCount)
	assert.NotNil(t, updated.EnabledAt, "source should still be enabled after 2 failures")

	// Third failure -- should disable source
	service.handleFetchError(*updated, testErr)
	updated, err = metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, 3, updated.FetchErrorCount)
	assert.Nil(t, updated.EnabledAt, "source should be disabled after reaching threshold")
}

// TestDiscoveryService_handleFetchSuccess verifies success handling per RFC 7
// section 4.3.
func TestDiscoveryService_handleFetchSuccess(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	// Create a test source with error count
	now := time.Now()
	source, err := metadataStore.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	// Simulate previous error
	errorMsg := "previous error"
	err = metadataStore.UpdateSource(source.SourceID, map[string]any{
		"fetch_error_count": 5,
		"last_error":        &errorMsg,
	})
	require.NoError(t, err)

	// Handle success
	updated, err := metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	service.handleFetchSuccess(*updated)

	// Verify error count reset and last_fetched_at updated
	updated, err = metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, 0, updated.FetchErrorCount, "error count should be reset to 0")
	assert.NotNil(t, updated.LastFetchedAt, "last_fetched_at should be set")
}

// TestDiscoveryService_Run_StartupBehavior verifies that the service fetches
// sources immediately on startup per RFC 7 section 3.3.
func TestDiscoveryService_Run_StartupBehavior(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	config := DefaultDiscoveryConfig()
	config.PollInterval = 1 * time.Hour
	service := NewDiscoveryService(metadataStore, newsFeed, config)

	// Create enabled source that has never been fetched
	now := time.Now()
	_, err = metadataStore.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	// Start service with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run service -- it should attempt to fetch immediately. This test just
	// verifies it doesn't crash; actual fetch will fail because the URL is
	// fake
	err = service.Run(ctx)

	// Expect context timeout, not a crash
	assert.Equal(t, context.DeadlineExceeded, err)
}

// TestDiscoveryService_isPermanentError verifies permanent vs transient error
// detection per RFC 7 section 7.1 and 7.2.
func TestDiscoveryService_isPermanentError(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	tests := []struct {
		name        string
		errorMsg    string
		isPermanent bool
	}{
		{
			name:        "404 not found",
			errorMsg:    "HTTP error: 404 Not Found",
			isPermanent: true,
		},
		{
			name:        "410 gone",
			errorMsg:    "HTTP error: 410 Gone",
			isPermanent: true,
		},
		{
			name:        "invalid feed format",
			errorMsg:    "failed to parse feed: invalid XML",
			isPermanent: true,
		},
		{
			name:        "no such host",
			errorMsg:    "dial tcp: lookup example.invalid: no such host",
			isPermanent: true,
		},
		{
			name:        "timeout (transient)",
			errorMsg:    "context deadline exceeded",
			isPermanent: false,
		},
		{
			name:        "500 server error (transient)",
			errorMsg:    "HTTP error: 500 Internal Server Error",
			isPermanent: false,
		},
		{
			name:        "connection refused (transient)",
			errorMsg:    "dial tcp: connection refused",
			isPermanent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := assert.AnError
			// Wrap error with message
			err = fmt.Errorf("%s", tt.errorMsg)
			isPermanent := service.isPermanentError(err)
			assert.Equal(t, tt.isPermanent, isPermanent)
		})
	}
}

// TestDiscoveryService_handleFetchError_PermanentError verifies that
// permanent errors immediately disable the source per RFC 7 section 7.2.
func TestDiscoveryService_handleFetchError_PermanentError(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	// Create a test source
	now := time.Now()
	source, err := metadataStore.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	// Simulate permanent error (404)
	permanentErr := fmt.Errorf("HTTP error: 404 Not Found")
	service.handleFetchError(*source, permanentErr)

	// Verify source was disabled immediately
	updated, err := metadataStore.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Nil(t, updated.EnabledAt, "source should be disabled immediately on permanent error")
	assert.Equal(t, 1, updated.FetchErrorCount, "error count should be incremented")
}

// TestDiscoveryService_domainRateLimiter verifies rate limiting per RFC 7
// section 8.2.
func TestDiscoveryService_domainRateLimiter(t *testing.T) {
	limiter := newDomainRateLimiter(100 * time.Millisecond)

	domain := "example.com"

	// First request should be immediate
	start := time.Now()
	limiter.wait(domain)
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 50*time.Millisecond, "first request should be immediate")

	// Second request should be rate limited
	start = time.Now()
	limiter.wait(domain)
	elapsed = time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "second request should wait at least 100ms")

	// Request to different domain should be immediate
	start = time.Now()
	limiter.wait("other.com")
	elapsed = time.Since(start)
	assert.Less(t, elapsed, 50*time.Millisecond, "request to different domain should be immediate")
}

// TestDiscoveryService_extractDomain verifies domain extraction for rate
// limiting.
func TestDiscoveryService_extractDomain(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	tests := []struct {
		name           string
		url            string
		expectedDomain string
	}{
		{
			name:           "simple domain",
			url:            "http://example.com/feed",
			expectedDomain: "example.com",
		},
		{
			name:           "subdomain",
			url:            "https://blog.example.com/articles",
			expectedDomain: "blog.example.com",
		},
		{
			name:           "with port",
			url:            "http://example.com:8080/feed",
			expectedDomain: "example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := service.extractDomain(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDomain, domain)
		})
	}
}

// TestDiscoveryService_resolveURL verifies URL resolution for relative links.
func TestDiscoveryService_resolveURL(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := tempDir + "/metadata.db"
	feedDir := tempDir + "/.news"

	metadataStore, err := NewMetadataStore(metadataPath)
	require.NoError(t, err)
	defer metadataStore.Close()

	newsFeed, err := NewNewsFeed(feedDir)
	require.NoError(t, err)

	service := NewDiscoveryService(metadataStore, newsFeed, nil)

	tests := []struct {
		name        string
		baseURL     string
		href        string
		expectedURL string
	}{
		{
			name:        "absolute URL",
			baseURL:     "http://example.com/page",
			href:        "http://other.com/article",
			expectedURL: "http://other.com/article",
		},
		{
			name:        "relative path",
			baseURL:     "http://example.com/page",
			href:        "/article/123",
			expectedURL: "http://example.com/article/123",
		},
		{
			name:        "relative with dot",
			baseURL:     "http://example.com/blog/page",
			href:        "../article/123",
			expectedURL: "http://example.com/article/123",
		},
		{
			name:        "same directory",
			baseURL:     "http://example.com/blog/page",
			href:        "article",
			expectedURL: "http://example.com/blog/article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := service.resolveURL(tt.baseURL, tt.href)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, resolved)
		})
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
