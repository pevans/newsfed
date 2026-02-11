package sources

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: create a test source store
func createTestSourceStore(t *testing.T) *SourceStore {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewSourceStore(dbPath)
	require.NoError(t, err, "should create source store")
	t.Cleanup(func() { store.Close() })
	return store
}

// Test helper: create a sample scraper config
func createTestScraperConfig() *scraper.ScraperConfig {
	return &scraper.ScraperConfig{
		DiscoveryMode: "direct",
		ArticleConfig: scraper.ArticleConfig{
			TitleSelector:   "h1.title",
			ContentSelector: "div.content",
			AuthorSelector:  "span.author",
		},
	}
}

// TestNewSourceStore_CreatesDatabase verifies database creation
func TestNewSourceStore_CreatesDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewSourceStore(dbPath)
	require.NoError(t, err, "should create store")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify we can perform basic operations
	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err, "should be able to query database")
	assert.Empty(t, sources, "new database should have no sources")
}

// TestNewSourceStore_InitializesSchema verifies schema creation
func TestNewSourceStore_InitializesSchema(t *testing.T) {
	store := createTestSourceStore(t)

	// Verify sources table exists by querying it
	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err, "sources table should exist")
	assert.Empty(t, sources)
}

// TestNewSourceStore_ExistingDatabase verifies opening existing database
func TestNewSourceStore_ExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create first store and add data
	store1, err := NewSourceStore(dbPath)
	require.NoError(t, err)

	now := time.Now()
	_, err = store1.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)
	store1.Close()

	// Open database again
	store2, err := NewSourceStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	// Verify data persisted
	sources, err := store2.ListSources(SourceFilter{})
	require.NoError(t, err)
	assert.Len(t, sources, 1, "data should persist across connections")
}

// TestCreateSource_BasicRSS verifies creating RSS source
func TestCreateSource_BasicRSS(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test RSS", nil, &now)

	require.NoError(t, err)
	require.NotNil(t, source)
	assert.NotEqual(t, uuid.Nil, source.SourceID, "should generate UUID")
	assert.Equal(t, "rss", source.SourceType)
	assert.Equal(t, "http://example.com/feed", source.URL)
	assert.Equal(t, "Test RSS", source.Name)
	assert.NotNil(t, source.EnabledAt, "should be enabled")
	assert.Equal(t, 0, source.FetchErrorCount)
	assert.Nil(t, source.ScraperConfig, "RSS should not have scraper config")
}

// TestCreateSource_WithScraperConfig verifies creating website source
func TestCreateSource_WithScraperConfig(t *testing.T) {
	store := createTestSourceStore(t)

	scraperConfig := createTestScraperConfig()
	now := time.Now()
	source, err := store.CreateSource(
		"website",
		"http://example.com",
		"Test Website",
		scraperConfig,
		&now,
	)

	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, "website", source.SourceType)
	require.NotNil(t, source.ScraperConfig, "website should have scraper config")
	assert.Equal(t, "direct", source.ScraperConfig.DiscoveryMode)
	assert.Equal(t, "h1.title", source.ScraperConfig.ArticleConfig.TitleSelector)
}

// TestCreateSource_DisabledSource verifies creating disabled source
func TestCreateSource_DisabledSource(t *testing.T) {
	store := createTestSourceStore(t)

	source, err := store.CreateSource("rss", "http://example.com", "Disabled", nil, nil)

	require.NoError(t, err)
	assert.Nil(t, source.EnabledAt, "should be disabled when enabledAt is nil")
}

// TestCreateSource_DuplicateURL verifies unique URL constraint
func TestCreateSource_DuplicateURL(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	_, err := store.CreateSource("rss", "http://example.com/feed", "First", nil, &now)
	require.NoError(t, err)

	// Try to create another with same URL
	_, err = store.CreateSource("atom", "http://example.com/feed", "Second", nil, &now)
	assert.ErrorIs(t, err, ErrDuplicateURL, "should return duplicate URL error")
}

// TestCreateSource_Timestamps verifies timestamp generation
func TestCreateSource_Timestamps(t *testing.T) {
	store := createTestSourceStore(t)

	before := time.Now()
	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	after := time.Now()

	require.NoError(t, err)
	assert.True(t, source.CreatedAt.After(before) || source.CreatedAt.Equal(before))
	assert.True(t, source.CreatedAt.Before(after) || source.CreatedAt.Equal(after))
	assert.Equal(t, source.CreatedAt, source.UpdatedAt, "created and updated should be equal initially")
}

// TestGetSource_Success verifies retrieving a source
func TestGetSource_Success(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	created, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	retrieved, err := store.GetSource(created.SourceID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, created.SourceID, retrieved.SourceID)
	assert.Equal(t, created.SourceType, retrieved.SourceType)
	assert.Equal(t, created.URL, retrieved.URL)
	assert.Equal(t, created.Name, retrieved.Name)
}

// TestGetSource_NotFound verifies error for non-existent source
func TestGetSource_NotFound(t *testing.T) {
	store := createTestSourceStore(t)

	randomID := uuid.New()
	_, err := store.GetSource(randomID)
	assert.ErrorIs(t, err, ErrSourceNotFound, "should return not found error")
}

// TestGetSource_PreservesAllFields verifies all fields are retrieved
// correctly
func TestGetSource_PreservesAllFields(t *testing.T) {
	store := createTestSourceStore(t)

	scraperConfig := createTestScraperConfig()
	now := time.Now()
	created, err := store.CreateSource("website", "http://example.com", "Test", scraperConfig, &now)
	require.NoError(t, err)

	// Update with all optional fields
	interval := "30m"
	lastModified := "Mon, 02 Jan 2006 15:04:05 MST"
	etag := "abc123"
	errorMsg := "test error"
	update := SourceUpdate{
		PollingInterval: &interval,
		LastModified:    &lastModified,
		ETag:            &etag,
		LastError:       &errorMsg,
	}
	err = store.UpdateSource(created.SourceID, update)
	require.NoError(t, err)

	retrieved, err := store.GetSource(created.SourceID)
	require.NoError(t, err)

	assert.NotNil(t, retrieved.ScraperConfig)
	assert.Equal(t, interval, *retrieved.PollingInterval)
	assert.Equal(t, lastModified, *retrieved.LastModified)
	assert.Equal(t, etag, *retrieved.ETag)
	assert.Equal(t, errorMsg, *retrieved.LastError)
}

// TestListSources_Empty verifies listing with no sources
func TestListSources_Empty(t *testing.T) {
	store := createTestSourceStore(t)

	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err)
	assert.Empty(t, sources)
}

// TestListSources_Multiple verifies listing multiple sources
func TestListSources_Multiple(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	_, err := store.CreateSource("rss", "http://example.com/1", "Source 1", nil, &now)
	require.NoError(t, err)
	_, err = store.CreateSource("atom", "http://example.com/2", "Source 2", nil, &now)
	require.NoError(t, err)
	_, err = store.CreateSource("website", "http://example.com/3", "Source 3", nil, &now)
	require.NoError(t, err)

	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err)
	assert.Len(t, sources, 3)
}

// TestListSources_OrderByCreatedDesc verifies sources are ordered by
// created_at DESC
func TestListSources_OrderByCreatedDesc(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	first, err := store.CreateSource("rss", "http://example.com/1", "First", nil, &now)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	second, err := store.CreateSource("atom", "http://example.com/2", "Second", nil, &now)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	third, err := store.CreateSource("website", "http://example.com/3", "Third", nil, &now)
	require.NoError(t, err)

	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Should be in reverse creation order
	assert.Equal(t, third.SourceID, sources[0].SourceID)
	assert.Equal(t, second.SourceID, sources[1].SourceID)
	assert.Equal(t, first.SourceID, sources[2].SourceID)
}

// TestUpdateSource_Name verifies updating source name
func TestUpdateSource_Name(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Old Name", nil, &now)
	require.NoError(t, err)

	newName := "New Name"
	update := SourceUpdate{Name: &newName}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

// TestUpdateSource_URL verifies updating source URL
func TestUpdateSource_URL(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/old", "Test", nil, &now)
	require.NoError(t, err)

	newURL := "http://example.com/new"
	update := SourceUpdate{URL: &newURL}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/new", updated.URL)
}

// TestUpdateSource_EnabledAt verifies enabling/disabling sources
func TestUpdateSource_EnabledAt(t *testing.T) {
	store := createTestSourceStore(t)

	// Create enabled source
	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)
	assert.NotNil(t, source.EnabledAt, "should start enabled")

	// Disable it
	update := SourceUpdate{ClearEnabledAt: true}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Nil(t, updated.EnabledAt, "should be disabled")

	// Re-enable it
	newTime := time.Now()
	update = SourceUpdate{EnabledAt: &newTime}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err = store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.NotNil(t, updated.EnabledAt, "should be enabled again")
}

// TestUpdateSource_PollingInterval verifies updating polling interval
func TestUpdateSource_PollingInterval(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	interval := "30m"
	update := SourceUpdate{PollingInterval: &interval}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	require.NotNil(t, updated.PollingInterval)
	assert.Equal(t, "30m", *updated.PollingInterval)
}

// TestUpdateSource_ScraperConfig verifies updating scraper config
func TestUpdateSource_ScraperConfig(t *testing.T) {
	store := createTestSourceStore(t)

	// Create source with initial config
	initialConfig := createTestScraperConfig()
	now := time.Now()
	source, err := store.CreateSource("website", "http://example.com", "Test", initialConfig, &now)
	require.NoError(t, err)

	// Update with new config
	newConfig := &scraper.ScraperConfig{
		DiscoveryMode: "list",
		ListConfig: &scraper.ListConfig{
			ArticleSelector: "article.post",
			MaxPages:        3,
		},
		ArticleConfig: scraper.ArticleConfig{
			TitleSelector:   "h2.headline",
			ContentSelector: "div.body",
		},
	}

	update := SourceUpdate{ScraperConfig: newConfig}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	require.NotNil(t, updated.ScraperConfig)
	assert.Equal(t, "list", updated.ScraperConfig.DiscoveryMode)
	assert.Equal(t, "h2.headline", updated.ScraperConfig.ArticleConfig.TitleSelector)
	require.NotNil(t, updated.ScraperConfig.ListConfig)
	assert.Equal(t, "article.post", updated.ScraperConfig.ListConfig.ArticleSelector)
}

// TestUpdateSource_MultipleFields verifies updating multiple fields at once
func TestUpdateSource_MultipleFields(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	newName := "Updated"
	newURL := "http://updated.com"
	interval := "1h"
	update := SourceUpdate{
		Name:            &newName,
		URL:             &newURL,
		PollingInterval: &interval,
	}

	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "http://updated.com", updated.URL)
	assert.Equal(t, "1h", *updated.PollingInterval)
}

// TestUpdateSource_UpdatesTimestamp verifies updated_at is updated
func TestUpdateSource_UpdatesTimestamp(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	originalUpdated := source.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	newName := "Updated"
	update := SourceUpdate{Name: &newName}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.True(t, updated.UpdatedAt.After(originalUpdated), "updated_at should be newer")
	assert.True(t, updated.CreatedAt.Equal(source.CreatedAt), "created_at should not change")
}

// TestUpdateSource_NotFound verifies error for non-existent source
func TestUpdateSource_NotFound(t *testing.T) {
	store := createTestSourceStore(t)

	randomID := uuid.New()
	newName := "Test"
	update := SourceUpdate{Name: &newName}
	err := store.UpdateSource(randomID, update)
	assert.ErrorIs(t, err, ErrSourceNotFound, "should return not found error")
}

// TestUpdateSource_EmptyUpdates verifies updating with no changes still
// updates timestamp
func TestUpdateSource_EmptyUpdates(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	originalUpdated := source.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	update := SourceUpdate{}
	err = store.UpdateSource(source.SourceID, update)
	require.NoError(t, err)

	updated, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.True(t, updated.UpdatedAt.After(originalUpdated), "updated_at should still be updated")
}

// TestDeleteSource_Success verifies deleting a source
func TestDeleteSource_Success(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	err = store.DeleteSource(source.SourceID)
	require.NoError(t, err)

	// Verify it's gone
	_, err = store.GetSource(source.SourceID)
	assert.ErrorIs(t, err, ErrSourceNotFound)
}

// TestDeleteSource_NotFound verifies error for non-existent source
func TestDeleteSource_NotFound(t *testing.T) {
	store := createTestSourceStore(t)

	randomID := uuid.New()
	err := store.DeleteSource(randomID)
	assert.ErrorIs(t, err, ErrSourceNotFound, "should return not found error")
}

// TestDeleteSource_DoesNotAffectOthers verifies deleting one source doesn't
// affect others
func TestDeleteSource_DoesNotAffectOthers(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source1, err := store.CreateSource("rss", "http://example.com/1", "Source 1", nil, &now)
	require.NoError(t, err)
	source2, err := store.CreateSource("atom", "http://example.com/2", "Source 2", nil, &now)
	require.NoError(t, err)

	err = store.DeleteSource(source1.SourceID)
	require.NoError(t, err)

	// Verify source2 still exists
	retrieved, err := store.GetSource(source2.SourceID)
	require.NoError(t, err)
	assert.Equal(t, source2.Name, retrieved.Name)

	// Verify only one source remains
	sources, err := store.ListSources(SourceFilter{})
	require.NoError(t, err)
	assert.Len(t, sources, 1)
}

// TestSource_IsEnabled verifies the IsEnabled helper method
func TestSource_IsEnabled(t *testing.T) {
	now := time.Now()

	// Enabled source
	enabled := Source{EnabledAt: &now}
	assert.True(t, enabled.IsEnabled())

	// Disabled source
	disabled := Source{EnabledAt: nil}
	assert.False(t, disabled.IsEnabled())
}

// TestRecordError_StoresErrorHistory verifies that errors are persisted
func TestRecordError_StoresErrorHistory(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test", nil, &now)
	require.NoError(t, err)

	err = store.RecordError(source.SourceID, "connection timeout", now)
	require.NoError(t, err)

	errors, err := store.ListErrors(source.SourceID, 10)
	require.NoError(t, err)
	require.Len(t, errors, 1)
	assert.Equal(t, "connection timeout", errors[0].Error)
	assert.Equal(t, source.SourceID, errors[0].SourceID)
}

// TestListErrors_OrdersMostRecentFirst verifies descending chronological order
func TestListErrors_OrdersMostRecentFirst(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test", nil, &now)
	require.NoError(t, err)

	t1 := now.Add(-2 * time.Hour)
	t2 := now.Add(-1 * time.Hour)
	t3 := now

	require.NoError(t, store.RecordError(source.SourceID, "first error", t1))
	require.NoError(t, store.RecordError(source.SourceID, "second error", t2))
	require.NoError(t, store.RecordError(source.SourceID, "third error", t3))

	errors, err := store.ListErrors(source.SourceID, 10)
	require.NoError(t, err)
	require.Len(t, errors, 3)
	assert.Equal(t, "third error", errors[0].Error)
	assert.Equal(t, "second error", errors[1].Error)
	assert.Equal(t, "first error", errors[2].Error)
}

// TestListErrors_RespectsLimit verifies the limit parameter
func TestListErrors_RespectsLimit(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test", nil, &now)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		require.NoError(t, store.RecordError(source.SourceID, "error", now.Add(time.Duration(i)*time.Minute)))
	}

	errors, err := store.ListErrors(source.SourceID, 2)
	require.NoError(t, err)
	assert.Len(t, errors, 2)
}

// TestListErrors_EmptyForCleanSource verifies no errors for a source that
// has never failed
func TestListErrors_EmptyForCleanSource(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test", nil, &now)
	require.NoError(t, err)

	errors, err := store.ListErrors(source.SourceID, 10)
	require.NoError(t, err)
	assert.Empty(t, errors)
}

// TestListErrors_IsolatedPerSource verifies errors are scoped to the correct source
func TestListErrors_IsolatedPerSource(t *testing.T) {
	store := createTestSourceStore(t)

	now := time.Now()
	source1, err := store.CreateSource("rss", "http://example.com/1", "Source 1", nil, &now)
	require.NoError(t, err)
	source2, err := store.CreateSource("rss", "http://example.com/2", "Source 2", nil, &now)
	require.NoError(t, err)

	require.NoError(t, store.RecordError(source1.SourceID, "error for source 1", now))
	require.NoError(t, store.RecordError(source2.SourceID, "error for source 2", now))

	errors1, err := store.ListErrors(source1.SourceID, 10)
	require.NoError(t, err)
	require.Len(t, errors1, 1)
	assert.Equal(t, "error for source 1", errors1[0].Error)

	errors2, err := store.ListErrors(source2.SourceID, 10)
	require.NoError(t, err)
	require.Len(t, errors2, 1)
	assert.Equal(t, "error for source 2", errors2[0].Error)
}
