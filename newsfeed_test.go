package newsfed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: create a sample news item for testing
func createTestItem(title string) newsfeed.NewsItem {
	publisher := "Test Publisher"
	return newsfeed.NewsItem{
		ID:           uuid.New(),
		Title:        title,
		Summary:      "Test summary for " + title,
		URL:          "http://example.com/" + title,
		Publisher:    &publisher,
		Authors:      []string{"Author 1", "Author 2"},
		PublishedAt:  time.Now().Add(-24 * time.Hour),
		DiscoveredAt: time.Now(),
		PinnedAt:     nil,
	}
}

// TestNew_CreatesDirectory verifies that New creates the storage directory
func TestNew_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "newsfeed")

	// Directory should not exist yet
	_, err := os.Stat(storageDir)
	assert.True(t, os.IsNotExist(err), "directory should not exist before New")

	feed, err := newsfeed.NewNewsFeed(storageDir)
	require.NoError(t, err, "New should succeed")
	require.NotNil(t, feed, "feed should not be nil")

	// Directory should exist now
	stat, err := os.Stat(storageDir)
	require.NoError(t, err, "directory should exist after New")
	assert.True(t, stat.IsDir(), "path should be a directory")
}

// TestNew_CreatesNestedDirectories verifies nested directory creation
func TestNew_CreatesNestedDirectories(t *testing.T) {
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "deeply", "nested", "newsfeed")

	feed, err := newsfeed.NewNewsFeed(storageDir)
	require.NoError(t, err, "New should create nested directories")
	require.NotNil(t, feed)

	// Verify directory exists
	stat, err := os.Stat(storageDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}

// TestNew_ExistingDirectory verifies New works with existing directory
func TestNew_ExistingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "newsfeed")

	// Create directory first
	err := os.MkdirAll(storageDir, 0o755)
	require.NoError(t, err)

	feed, err := newsfeed.NewNewsFeed(storageDir)
	require.NoError(t, err, "New should work with existing directory")
	require.NotNil(t, feed)
}

// TestAdd_CreatesFile verifies that Add creates a JSON file
func TestAdd_CreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err, "Add should succeed")

	// Verify file exists
	filename := filepath.Join(tempDir, item.ID.String()+".json")
	_, err = os.Stat(filename)
	assert.NoError(t, err, "JSON file should exist")
}

// TestAdd_FileContents verifies JSON file contents are correct
func TestAdd_FileContents(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err)

	// Read and verify file contents
	filename := filepath.Join(tempDir, item.ID.String()+".json")
	data, err := os.ReadFile(filename)
	require.NoError(t, err, "should be able to read file")

	var savedItem newsfeed.NewsItem
	err = json.Unmarshal(data, &savedItem)
	require.NoError(t, err, "file should contain valid JSON")

	// Verify all fields match
	assert.Equal(t, item.ID, savedItem.ID, "ID should match")
	assert.Equal(t, item.Title, savedItem.Title, "Title should match")
	assert.Equal(t, item.Summary, savedItem.Summary, "Summary should match")
	assert.Equal(t, item.URL, savedItem.URL, "URL should match")
	assert.Equal(t, item.Authors, savedItem.Authors, "Authors should match")
}

// TestAdd_Overwrite verifies that Add can overwrite existing files
func TestAdd_Overwrite(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Original Title")
	err = feed.Add(item)
	require.NoError(t, err)

	// Modify and add again with same ID
	item.Title = "Updated Title"
	err = feed.Add(item)
	require.NoError(t, err, "Add should allow overwriting")

	// Verify updated contents
	filename := filepath.Join(tempDir, item.ID.String()+".json")
	data, err := os.ReadFile(filename)
	require.NoError(t, err)

	var savedItem newsfeed.NewsItem
	err = json.Unmarshal(data, &savedItem)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", savedItem.Title, "file should contain updated data")
}

// TestList_EmptyDirectory verifies List returns empty slice for empty feed
func TestList_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	items, err := feed.List()
	require.NoError(t, err, "List should succeed on empty directory")
	assert.Empty(t, items, "List should return empty slice")
}

// TestList_SingleItem verifies List returns single item
func TestList_SingleItem(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err)

	items, err := feed.List()
	require.NoError(t, err)
	require.Len(t, items, 1, "should return one item")
	assert.Equal(t, item.ID, items[0].ID, "returned item should match")
}

// TestList_MultipleItems verifies List returns all items
func TestList_MultipleItems(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	itemCount := 5
	addedIDs := make(map[uuid.UUID]bool)

	for i := range itemCount {
		item := createTestItem("Article " + string(rune('A'+i)))
		err = feed.Add(item)
		require.NoError(t, err)
		addedIDs[item.ID] = true
	}

	items, err := feed.List()
	require.NoError(t, err)
	assert.Len(t, items, itemCount, "should return all items")

	// Verify all IDs are present
	for _, item := range items {
		assert.True(t, addedIDs[item.ID], "returned item should be one we added")
	}
}

// TestList_IgnoresNonJSONFiles verifies List skips non-JSON files
func TestList_IgnoresNonJSONFiles(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	// Add a valid item
	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err)

	// Create non-JSON files
	err = os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("test"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte("test"), 0o644)
	require.NoError(t, err)

	items, err := feed.List()
	require.NoError(t, err)
	assert.Len(t, items, 1, "should only return JSON files")
}

// TestList_IgnoresDirectories verifies List skips subdirectories
func TestList_IgnoresDirectories(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	// Add a valid item
	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err)

	// Create a subdirectory
	err = os.Mkdir(filepath.Join(tempDir, "subdir"), 0o755)
	require.NoError(t, err)

	items, err := feed.List()
	require.NoError(t, err)
	assert.Len(t, items, 1, "should ignore subdirectories")
}

// TestList_SkipsCorruptedFiles verifies List continues on corrupted files
func TestList_SkipsCorruptedFiles(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	// Add valid items
	item1 := createTestItem("Article 1")
	item2 := createTestItem("Article 2")
	err = feed.Add(item1)
	require.NoError(t, err)
	err = feed.Add(item2)
	require.NoError(t, err)

	// Create a corrupted JSON file
	corruptedFile := filepath.Join(tempDir, uuid.New().String()+".json")
	err = os.WriteFile(corruptedFile, []byte("{invalid json}"), 0o644)
	require.NoError(t, err)

	items, err := feed.List()
	require.NoError(t, err, "List should succeed despite corrupted file")
	assert.Len(t, items, 2, "should return valid items and skip corrupted ones")
}

// TestList_SkipsUnreadableFiles verifies List continues on unreadable files
func TestList_SkipsUnreadableFiles(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root (permission test won't work)")
	}

	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	// Add valid item
	item := createTestItem("Article 1")
	err = feed.Add(item)
	require.NoError(t, err)

	// Create an unreadable file
	unreadableFile := filepath.Join(tempDir, uuid.New().String()+".json")
	err = os.WriteFile(unreadableFile, []byte("{}"), 0o000)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Chmod(unreadableFile, 0o644) // Restore permissions for cleanup
	})

	items, err := feed.List()
	require.NoError(t, err, "List should succeed despite unreadable file")
	assert.Len(t, items, 1, "should return readable items")
}

// TestGet_Success verifies Get retrieves item by ID
func TestGet_Success(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Test Article")
	err = feed.Add(item)
	require.NoError(t, err)

	retrieved, err := feed.Get(item.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved, "Get should return item")
	assert.Equal(t, item.ID, retrieved.ID, "retrieved item should match")
	assert.Equal(t, item.Title, retrieved.Title, "all fields should match")
}

// TestGet_NotFound verifies Get returns nil for non-existent item
func TestGet_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	nonExistentID := uuid.New()
	retrieved, err := feed.Get(nonExistentID)
	require.NoError(t, err, "Get should not error on not found")
	assert.Nil(t, retrieved, "Get should return nil for non-existent item")
}

// TestGet_CorruptedFile verifies Get returns error for corrupted file
func TestGet_CorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	// Create a corrupted JSON file
	id := uuid.New()
	filename := filepath.Join(tempDir, id.String()+".json")
	err = os.WriteFile(filename, []byte("{invalid json}"), 0o644)
	require.NoError(t, err)

	retrieved, err := feed.Get(id)
	assert.Error(t, err, "Get should error on corrupted file")
	assert.Nil(t, retrieved, "Get should return nil on error")
}

// TestUpdate_Success verifies Update modifies existing item
func TestUpdate_Success(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Original Title")
	err = feed.Add(item)
	require.NoError(t, err)

	// Modify and update
	item.Title = "Updated Title"
	item.Summary = "Updated summary"
	err = feed.Update(item)
	require.NoError(t, err, "Update should succeed")

	// Verify changes persisted
	retrieved, err := feed.Get(item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title, "title should be updated")
	assert.Equal(t, "Updated summary", retrieved.Summary, "summary should be updated")
}

// TestUpdate_NotFound verifies Update returns error for non-existent item
func TestUpdate_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Non-existent")
	err = feed.Update(item)
	assert.Error(t, err, "Update should error on non-existent item")
	assert.Contains(t, err.Error(), "not found", "error should indicate item not found")
}

// TestUpdate_PreservesOtherFields verifies Update doesn't corrupt data
func TestUpdate_PreservesOtherFields(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	now := time.Now()
	pinnedTime := now
	publisher := "Original Publisher"
	item := newsfeed.NewsItem{
		ID:           uuid.New(),
		Title:        "Original",
		Summary:      "Original summary",
		URL:          "http://example.com/original",
		Publisher:    &publisher,
		Authors:      []string{"Author 1", "Author 2"},
		PublishedAt:  now.Add(-24 * time.Hour),
		DiscoveredAt: now,
		PinnedAt:     &pinnedTime,
	}
	err = feed.Add(item)
	require.NoError(t, err)

	// Update only title
	item.Title = "Updated"
	err = feed.Update(item)
	require.NoError(t, err)

	// Verify all fields intact
	retrieved, err := feed.Get(item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", retrieved.Title, "title should be updated")
	assert.Equal(t, item.Summary, retrieved.Summary, "summary should be preserved")
	assert.Equal(t, item.URL, retrieved.URL, "URL should be preserved")
	assert.Equal(t, item.Authors, retrieved.Authors, "authors should be preserved")
	assert.NotNil(t, retrieved.PinnedAt, "pinned status should be preserved")
}

// Property test: Add and Get are inverse operations
func TestAddGet_InverseOperations(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	now := time.Now()
	publisher := "Test Publisher"
	pinnedTime := now
	testCases := []newsfeed.NewsItem{
		{
			ID:           uuid.New(),
			Title:        "Simple item",
			Summary:      "Summary",
			URL:          "http://example.com",
			Publisher:    &publisher,
			Authors:      []string{"Author"},
			PublishedAt:  now,
			DiscoveredAt: now,
			PinnedAt:     nil,
		},
		{
			ID:           uuid.New(),
			Title:        "Item with pinned",
			Summary:      "Summary",
			URL:          "http://example.com/2",
			Publisher:    nil,
			Authors:      []string{"Author 1", "Author 2", "Author 3"},
			PublishedAt:  now,
			DiscoveredAt: now,
			PinnedAt:     &pinnedTime,
		},
		{
			ID:           uuid.New(),
			Title:        "Item with no publisher",
			Summary:      "Summary",
			URL:          "http://example.com/3",
			Publisher:    nil,
			Authors:      []string{},
			PublishedAt:  now,
			DiscoveredAt: now,
			PinnedAt:     nil,
		},
	}

	for _, original := range testCases {
		err = feed.Add(original)
		require.NoError(t, err)

		retrieved, err := feed.Get(original.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify all fields match
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Title, retrieved.Title)
		assert.Equal(t, original.Summary, retrieved.Summary)
		assert.Equal(t, original.URL, retrieved.URL)
		assert.Equal(t, original.Publisher, retrieved.Publisher)
		assert.Equal(t, original.Authors, retrieved.Authors)
		assert.True(t, original.PublishedAt.Equal(retrieved.PublishedAt), "PublishedAt should match")
		assert.True(t, original.DiscoveredAt.Equal(retrieved.DiscoveredAt), "DiscoveredAt should match")

		if original.PinnedAt == nil {
			assert.Nil(t, retrieved.PinnedAt)
		} else {
			require.NotNil(t, retrieved.PinnedAt)
			assert.True(t, original.PinnedAt.Equal(*retrieved.PinnedAt), "PinnedAt should match")
		}
	}
}

// Property test: List returns all added items
func TestList_ReturnsAllAdded(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	itemCount := 20
	addedIDs := make(map[uuid.UUID]bool)

	for i := range itemCount {
		item := createTestItem("Article " + string(rune('A'+i)))
		err = feed.Add(item)
		require.NoError(t, err)
		addedIDs[item.ID] = true
	}

	items, err := feed.List()
	require.NoError(t, err)
	assert.Len(t, items, itemCount, "List should return exactly all added items")

	// Verify each returned item was added
	for _, item := range items {
		assert.True(t, addedIDs[item.ID], "each returned item should be one we added")
		delete(addedIDs, item.ID)
	}

	assert.Empty(t, addedIDs, "all added items should be returned")
}

// Property test: Update followed by Get returns updated data
func TestUpdateGet_ConsistentState(t *testing.T) {
	tempDir := t.TempDir()
	feed, err := newsfeed.NewNewsFeed(tempDir)
	require.NoError(t, err)

	item := createTestItem("Original")
	err = feed.Add(item)
	require.NoError(t, err)

	// Perform multiple updates
	updates := []string{"Update 1", "Update 2", "Update 3"}
	for _, title := range updates {
		item.Title = title
		err = feed.Update(item)
		require.NoError(t, err)

		retrieved, err := feed.Get(item.ID)
		require.NoError(t, err)
		assert.Equal(t, title, retrieved.Title, "Get should always return latest Update")
	}
}
