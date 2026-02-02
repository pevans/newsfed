package newsfed

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: create a test metadata store
func createTestMetadataStore(t *testing.T) *MetadataStore {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	require.NoError(t, err, "should create metadata store")
	t.Cleanup(func() { store.Close() })
	return store
}

// Test helper: create a sample scraper config
func createTestScraperConfig() *ScraperConfig {
	return &ScraperConfig{
		DiscoveryMode: "direct",
		ArticleConfig: ArticleConfig{
			TitleSelector:   "h1.title",
			ContentSelector: "div.content",
			AuthorSelector:  "span.author",
		},
	}
}

// TestNewMetadataStore_CreatesDatabase verifies database creation
func TestNewMetadataStore_CreatesDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewMetadataStore(dbPath)
	require.NoError(t, err, "should create store")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify we can perform basic operations
	sources, err := store.ListSources()
	require.NoError(t, err, "should be able to query database")
	assert.Empty(t, sources, "new database should have no sources")
}

// TestNewMetadataStore_InitializesSchema verifies schema creation
func TestNewMetadataStore_InitializesSchema(t *testing.T) {
	store := createTestMetadataStore(t)

	// Verify sources table exists by querying it
	sources, err := store.ListSources()
	require.NoError(t, err, "sources table should exist")
	assert.Empty(t, sources)

	// Verify config table exists by querying it
	config, err := store.GetConfig()
	require.NoError(t, err, "config table should exist")
	assert.NotNil(t, config)
}

// TestNewMetadataStore_ExistingDatabase verifies opening existing database
func TestNewMetadataStore_ExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create first store and add data
	store1, err := NewMetadataStore(dbPath)
	require.NoError(t, err)

	now := time.Now()
	_, err = store1.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)
	store1.Close()

	// Open database again
	store2, err := NewMetadataStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	// Verify data persisted
	sources, err := store2.ListSources()
	require.NoError(t, err)
	assert.Len(t, sources, 1, "data should persist across connections")
}

// TestCreateSource_BasicRSS verifies creating RSS source
func TestCreateSource_BasicRSS(t *testing.T) {
	store := createTestMetadataStore(t)

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
	store := createTestMetadataStore(t)

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
	store := createTestMetadataStore(t)

	source, err := store.CreateSource("rss", "http://example.com", "Disabled", nil, nil)

	require.NoError(t, err)
	assert.Nil(t, source.EnabledAt, "should be disabled when enabledAt is nil")
}

// TestCreateSource_DuplicateURL verifies unique URL constraint
func TestCreateSource_DuplicateURL(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	_, err := store.CreateSource("rss", "http://example.com/feed", "First", nil, &now)
	require.NoError(t, err)

	// Try to create another with same URL
	_, err = store.CreateSource("atom", "http://example.com/feed", "Second", nil, &now)
	assert.Error(t, err, "should reject duplicate URL")
}

// TestCreateSource_Timestamps verifies timestamp handling
func TestCreateSource_Timestamps(t *testing.T) {
	store := createTestMetadataStore(t)

	before := time.Now()
	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	after := time.Now()

	require.NoError(t, err)
	assert.True(t, source.CreatedAt.After(before) || source.CreatedAt.Equal(before))
	assert.True(t, source.CreatedAt.Before(after) || source.CreatedAt.Equal(after))
	assert.Equal(t, source.CreatedAt, source.UpdatedAt, "timestamps should initially match")
}

// TestGetSource_Success verifies retrieving source by ID
func TestGetSource_Success(t *testing.T) {
	store := createTestMetadataStore(t)

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

// TestGetSource_NotFound verifies error on non-existent source
func TestGetSource_NotFound(t *testing.T) {
	store := createTestMetadataStore(t)

	nonExistentID := uuid.New()
	retrieved, err := store.GetSource(nonExistentID)

	assert.Error(t, err, "should error on not found")
	assert.Nil(t, retrieved)
	assert.Contains(t, err.Error(), "not found")
}

// TestGetSource_PreservesAllFields verifies all fields are retrieved
func TestGetSource_PreservesAllFields(t *testing.T) {
	store := createTestMetadataStore(t)

	scraperConfig := createTestScraperConfig()
	now := time.Now()
	pollingInterval := "30m"

	created, err := store.CreateSource("website", "http://example.com", "Test", scraperConfig, &now)
	require.NoError(t, err)

	// Update with additional fields
	err = store.UpdateSource(created.SourceID, map[string]interface{}{
		"polling_interval": pollingInterval,
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(created.SourceID)
	require.NoError(t, err)

	assert.Equal(t, created.SourceID, retrieved.SourceID)
	assert.NotNil(t, retrieved.EnabledAt)
	require.NotNil(t, retrieved.PollingInterval)
	assert.Equal(t, pollingInterval, *retrieved.PollingInterval)
	require.NotNil(t, retrieved.ScraperConfig)
	assert.Equal(t, "direct", retrieved.ScraperConfig.DiscoveryMode)
}

// TestListSources_Empty verifies listing empty sources
func TestListSources_Empty(t *testing.T) {
	store := createTestMetadataStore(t)

	sources, err := store.ListSources()

	require.NoError(t, err)
	assert.Empty(t, sources, "should return empty slice")
}

// TestListSources_Multiple verifies listing multiple sources
func TestListSources_Multiple(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	count := 5
	for i := range count {
		_, err := store.CreateSource("rss", "http://example.com/"+string(rune('a'+i)), "Source", nil, &now)
		require.NoError(t, err)
	}

	sources, err := store.ListSources()

	require.NoError(t, err)
	assert.Len(t, sources, count, "should return all sources")
}

// TestListSources_OrderByCreatedDesc verifies sources are ordered
func TestListSources_OrderByCreatedDesc(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()

	// Create first source
	source1, err := store.CreateSource("rss", "http://example.com/1", "First", nil, &now)
	require.NoError(t, err)

	// Sleep long enough for timestamp to differ (RFC3339 has second
	// precision)
	time.Sleep(1100 * time.Millisecond)

	source2, err := store.CreateSource("rss", "http://example.com/2", "Second", nil, &now)
	require.NoError(t, err)

	sources, err := store.ListSources()
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Should be ordered by created_at DESC (newest first)
	assert.Equal(t, source2.SourceID, sources[0].SourceID, "newest should be first")
	assert.Equal(t, source1.SourceID, sources[1].SourceID, "oldest should be last")
}

// TestUpdateSource_Name verifies updating source name
func TestUpdateSource_Name(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Original", nil, &now)
	require.NoError(t, err)

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"name": "Updated Name",
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
}

// TestUpdateSource_URL verifies updating source URL
func TestUpdateSource_URL(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/old", "Test", nil, &now)
	require.NoError(t, err)

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"url": "http://example.com/new",
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/new", retrieved.URL)
}

// TestUpdateSource_EnabledAt verifies toggling enabled status
func TestUpdateSource_EnabledAt(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)
	require.NotNil(t, source.EnabledAt, "should start enabled")

	// Disable the source
	var nilTime *time.Time
	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"enabled_at": nilTime,
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Nil(t, retrieved.EnabledAt, "should be disabled")

	// Re-enable the source
	newTime := time.Now()
	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"enabled_at": &newTime,
	})
	require.NoError(t, err)

	retrieved, err = store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.EnabledAt, "should be re-enabled")
}

// TestUpdateSource_PollingInterval verifies updating polling interval
func TestUpdateSource_PollingInterval(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"polling_interval": "2h",
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.PollingInterval)
	assert.Equal(t, "2h", *retrieved.PollingInterval)
}

// TestUpdateSource_ScraperConfig verifies updating scraper config
func TestUpdateSource_ScraperConfig(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	oldConfig := createTestScraperConfig()
	source, err := store.CreateSource("website", "http://example.com", "Test", oldConfig, &now)
	require.NoError(t, err)

	newConfig := &ScraperConfig{
		DiscoveryMode: "list",
		ListConfig: &ListConfig{
			ArticleSelector: "article.post",
			MaxPages:        3,
		},
		ArticleConfig: ArticleConfig{
			TitleSelector:   "h2.heading",
			ContentSelector: "div.body",
		},
	}

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"scraper_config": newConfig,
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.ScraperConfig)
	assert.Equal(t, "list", retrieved.ScraperConfig.DiscoveryMode)
	require.NotNil(t, retrieved.ScraperConfig.ListConfig)
	assert.Equal(t, "article.post", retrieved.ScraperConfig.ListConfig.ArticleSelector)
	assert.Equal(t, 3, retrieved.ScraperConfig.ListConfig.MaxPages)
}

// TestUpdateSource_MultipleFields verifies updating multiple fields
func TestUpdateSource_MultipleFields(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Original", nil, &now)
	require.NoError(t, err)

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"name":             "Updated",
		"url":              "http://new.example.com",
		"polling_interval": "30m",
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", retrieved.Name)
	assert.Equal(t, "http://new.example.com", retrieved.URL)
	require.NotNil(t, retrieved.PollingInterval)
	assert.Equal(t, "30m", *retrieved.PollingInterval)
}

// TestUpdateSource_UpdatesTimestamp verifies updated_at changes
func TestUpdateSource_UpdatesTimestamp(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	originalUpdatedAt := source.UpdatedAt

	// Sleep long enough for timestamp to differ (RFC3339 has second
	// precision)
	time.Sleep(1100 * time.Millisecond)

	err = store.UpdateSource(source.SourceID, map[string]interface{}{
		"name": "Changed",
	})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.True(t, retrieved.UpdatedAt.After(originalUpdatedAt), "updated_at should change")
}

// TestUpdateSource_NotFound verifies error on non-existent source
func TestUpdateSource_NotFound(t *testing.T) {
	store := createTestMetadataStore(t)

	nonExistentID := uuid.New()
	err := store.UpdateSource(nonExistentID, map[string]interface{}{
		"name": "Test",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestUpdateSource_EmptyUpdates verifies behavior with no updates
func TestUpdateSource_EmptyUpdates(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	// Update with empty map (only updated_at should change)
	err = store.UpdateSource(source.SourceID, map[string]interface{}{})
	require.NoError(t, err)

	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	assert.Equal(t, source.Name, retrieved.Name, "fields should be unchanged")
}

// TestDeleteSource_Success verifies deleting a source
func TestDeleteSource_Success(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	err = store.DeleteSource(source.SourceID)
	require.NoError(t, err)

	// Verify source is gone
	retrieved, err := store.GetSource(source.SourceID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

// TestDeleteSource_NotFound verifies error on non-existent source
func TestDeleteSource_NotFound(t *testing.T) {
	store := createTestMetadataStore(t)

	nonExistentID := uuid.New()
	err := store.DeleteSource(nonExistentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDeleteSource_DoesNotAffectOthers verifies selective deletion
func TestDeleteSource_DoesNotAffectOthers(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source1, err := store.CreateSource("rss", "http://example.com/1", "First", nil, &now)
	require.NoError(t, err)
	source2, err := store.CreateSource("rss", "http://example.com/2", "Second", nil, &now)
	require.NoError(t, err)

	err = store.DeleteSource(source1.SourceID)
	require.NoError(t, err)

	// Verify source2 still exists
	retrieved, err := store.GetSource(source2.SourceID)
	require.NoError(t, err)
	assert.Equal(t, source2.SourceID, retrieved.SourceID)

	// Verify only one source remains
	sources, err := store.ListSources()
	require.NoError(t, err)
	assert.Len(t, sources, 1)
}

// TestGetConfig_Default verifies default config
func TestGetConfig_Default(t *testing.T) {
	store := createTestMetadataStore(t)

	config, err := store.GetConfig()

	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, "1h", config.DefaultPollingInterval, "should return default")
}

// TestUpdateConfig_Success verifies updating config
func TestUpdateConfig_Success(t *testing.T) {
	store := createTestMetadataStore(t)

	newConfig := &Config{
		DefaultPollingInterval: "2h",
	}

	err := store.UpdateConfig(newConfig)
	require.NoError(t, err)

	retrieved, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, "2h", retrieved.DefaultPollingInterval)
}

// TestUpdateConfig_Overwrites verifies config replacement
func TestUpdateConfig_Overwrites(t *testing.T) {
	store := createTestMetadataStore(t)

	// Set initial config
	err := store.UpdateConfig(&Config{DefaultPollingInterval: "2h"})
	require.NoError(t, err)

	// Update again
	err = store.UpdateConfig(&Config{DefaultPollingInterval: "30m"})
	require.NoError(t, err)

	retrieved, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, "30m", retrieved.DefaultPollingInterval)
}

// TestFormatTime verifies time formatting helper
func TestFormatTime(t *testing.T) {
	// Nil time should return nil
	assert.Nil(t, formatTime(nil))

	// Non-nil time should format as RFC3339
	now := time.Now()
	formatted := formatTime(&now)
	require.NotNil(t, formatted)

	str, ok := formatted.(string)
	require.True(t, ok, "should return string")

	// Verify it's valid RFC3339
	parsed, err := time.Parse(time.RFC3339, str)
	require.NoError(t, err)

	// Compare truncated to seconds since RFC3339 doesn't preserve nanoseconds
	assert.True(t, now.Truncate(time.Second).Equal(parsed.Truncate(time.Second)), "should round-trip correctly")
}

// TestParseTime verifies time parsing helper
func TestParseTime(t *testing.T) {
	// Valid RFC3339 string
	timeStr := "2024-01-15T10:30:00Z"
	parsed := parseTime(timeStr)
	assert.Equal(t, 2024, parsed.Year())
	assert.Equal(t, time.January, parsed.Month())
	assert.Equal(t, 15, parsed.Day())

	// Invalid string should return zero time
	invalid := parseTime("invalid")
	assert.True(t, invalid.IsZero(), "should return zero time on parse error")
}

// Property test: CreateSource and GetSource are inverse operations
func TestCreateGet_InverseOperations(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	testCases := []struct {
		name          string
		sourceType    string
		url           string
		scraperConfig *ScraperConfig
		enabledAt     *time.Time
	}{
		{
			name:       "RSS enabled",
			sourceType: "rss",
			url:        "http://example.com/rss",
			enabledAt:  &now,
		},
		{
			name:       "Atom disabled",
			sourceType: "atom",
			url:        "http://example.com/atom",
			enabledAt:  nil,
		},
		{
			name:          "Website with config",
			sourceType:    "website",
			url:           "http://example.com",
			scraperConfig: createTestScraperConfig(),
			enabledAt:     &now,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := store.CreateSource(tc.sourceType, tc.url, tc.name, tc.scraperConfig, tc.enabledAt)
			require.NoError(t, err)

			retrieved, err := store.GetSource(created.SourceID)
			require.NoError(t, err)

			assert.Equal(t, created.SourceID, retrieved.SourceID)
			assert.Equal(t, created.SourceType, retrieved.SourceType)
			assert.Equal(t, created.URL, retrieved.URL)
			assert.Equal(t, created.Name, retrieved.Name)

			if tc.enabledAt == nil {
				assert.Nil(t, retrieved.EnabledAt)
			} else {
				require.NotNil(t, retrieved.EnabledAt)
			}

			if tc.scraperConfig != nil {
				require.NotNil(t, retrieved.ScraperConfig)
				assert.Equal(t, tc.scraperConfig.DiscoveryMode, retrieved.ScraperConfig.DiscoveryMode)
			} else {
				assert.Nil(t, retrieved.ScraperConfig)
			}
		})
	}
}

// Property test: UpdateSource followed by GetSource returns updated data
func TestMetadataUpdateGet_ConsistentState(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Original", nil, &now)
	require.NoError(t, err)

	updates := []map[string]interface{}{
		{"name": "Updated 1"},
		{"name": "Updated 2", "url": "http://new.example.com"},
		{"polling_interval": "45m"},
	}

	for _, update := range updates {
		err = store.UpdateSource(source.SourceID, update)
		require.NoError(t, err)

		retrieved, err := store.GetSource(source.SourceID)
		require.NoError(t, err)

		// Verify updates applied
		if name, ok := update["name"].(string); ok {
			assert.Equal(t, name, retrieved.Name)
		}
		if url, ok := update["url"].(string); ok {
			assert.Equal(t, url, retrieved.URL)
		}
		if interval, ok := update["polling_interval"].(string); ok {
			require.NotNil(t, retrieved.PollingInterval)
			assert.Equal(t, interval, *retrieved.PollingInterval)
		}
	}
}

// Property test: ListSources returns all created sources
func TestMetadataList_ReturnsAllCreated(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	count := 10
	createdIDs := make(map[uuid.UUID]bool)

	for i := range count {
		source, err := store.CreateSource("rss", "http://example.com/"+string(rune('a'+i)), "Source", nil, &now)
		require.NoError(t, err)
		createdIDs[source.SourceID] = true
	}

	sources, err := store.ListSources()
	require.NoError(t, err)
	assert.Len(t, sources, count, "should return all created sources")

	for _, source := range sources {
		assert.True(t, createdIDs[source.SourceID], "each returned source should be one we created")
		delete(createdIDs, source.SourceID)
	}

	assert.Empty(t, createdIDs, "all created sources should be returned")
}

// Property test: DeleteSource removes source completely
func TestMetadataDelete_RemovesCompletely(t *testing.T) {
	store := createTestMetadataStore(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com", "Test", nil, &now)
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := store.GetSource(source.SourceID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Delete it
	err = store.DeleteSource(source.SourceID)
	require.NoError(t, err)

	// Verify it's gone from GetSource
	retrieved, err = store.GetSource(source.SourceID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)

	// Verify it's gone from ListSources
	sources, err := store.ListSources()
	require.NoError(t, err)
	assert.Empty(t, sources)
}
