package newsfeed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Test helper: create a test news feed
func setupTestFeed(t *testing.T) *NewsFeed {
	tempDir := t.TempDir()
	feed, err := NewNewsFeed(tempDir)
	require.NoError(t, err)
	return feed
}

// Test helper: create a test API server
func setupTestAPIServer(t *testing.T) (*APIServer, *NewsFeed) {
	feed := setupTestFeed(t)
	server := NewAPIServer(feed)
	return server, feed
}

// Test helper: create a test router
func setupTestNewsFeedRouter(t *testing.T) (*gin.Engine, *NewsFeed) {
	feed := setupTestFeed(t)
	server := NewAPIServer(feed)
	router := server.SetupRouter()
	return router, feed
}

// Test helper: create a sample news item
func createSampleItem(
	id uuid.UUID,
	publisher string,
	authors []string,
	publishedAt, discoveredAt time.Time,
	pinnedAt *time.Time,
) NewsItem {
	var pub *string
	if publisher != "" {
		pub = &publisher
	}
	return NewsItem{
		ID:           id,
		Title:        "Test Article",
		Summary:      "Test summary",
		URL:          "http://example.com/article",
		Publisher:    pub,
		Authors:      authors,
		PublishedAt:  publishedAt,
		DiscoveredAt: discoveredAt,
		PinnedAt:     pinnedAt,
	}
}

// TestParseItemID verifies item ID extraction from URLs
func TestParseItemID(t *testing.T) {
	server, _ := setupTestAPIServer(t)
	validID := uuid.New()

	tests := []struct {
		name      string
		path      string
		prefix    string
		expectErr bool
	}{
		{
			name:      "valid path with UUID",
			path:      fmt.Sprintf("/api/v1/items/%s", validID),
			prefix:    "/api/v1/items/",
			expectErr: false,
		},
		{
			name:      "valid path with trailing slash",
			path:      fmt.Sprintf("/api/v1/items/%s/pin", validID),
			prefix:    "/api/v1/items/",
			expectErr: false,
		},
		{
			name:      "no ID provided",
			path:      "/api/v1/items/",
			prefix:    "/api/v1/items/",
			expectErr: true,
		},
		{
			name:      "invalid UUID",
			path:      "/api/v1/items/not-a-uuid",
			prefix:    "/api/v1/items/",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := server.parseItemID(tt.path, tt.prefix)
			if tt.expectErr {
				assert.Error(t, err, "parseItemID should error on invalid paths")
			} else {
				assert.NoError(t, err, "parseItemID should succeed on valid paths")
				assert.Equal(t, validID, id, "extracted ID should match")
			}
		})
	}
}

// TestHandleListItems_EmptyList verifies behavior with no items
func TestHandleListItems_EmptyList(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should return 200 for empty list")

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total, "total should be zero for empty list")
	assert.Empty(t, resp.Items, "items list should be empty")
	assert.Equal(t, 50, resp.Limit, "default limit should be 50")
	assert.Equal(t, 0, resp.Offset, "default offset should be 0")
}

// TestHandleListItems_WithItems verifies basic item listing
func TestHandleListItems_WithItems(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1"}, now.Add(-2*time.Hour), now.Add(-1*time.Hour), nil)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 2"}, now.Add(-1*time.Hour), now, nil)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total, "should return all items")
	assert.Len(t, resp.Items, 2, "should return all items")
}

// TestFilterByPinned verifies pinned status filtering
func TestFilterByPinned(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	pinnedTime := now
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1"}, now, now, &pinnedTime)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 2"}, now, now, nil)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))

	tests := []struct {
		name          string
		pinnedParam   string
		expectedCount int
	}{
		{"pinned=true", "true", 1},
		{"pinned=false", "false", 1},
		{"pinned=invalid", "invalid", 0}, // invalid values match nothing
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/items?pinned="+tt.pinnedParam, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			var resp ListItemsResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, resp.Total, "filter should return correct count")
		})
	}
}

// TestFilterByPublisher verifies publisher filtering
func TestFilterByPublisher(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1"}, now, now, nil)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 2"}, now, now, nil)
	item3 := createSampleItem(uuid.New(), "", []string{"Author 3"}, now, now, nil) // no publisher

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))
	require.NoError(t, feed.Add(item3))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?publisher=Publisher+A", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total, "should filter by publisher")
	assert.Equal(t, "Publisher A", *resp.Items[0].Publisher)
}

// TestFilterByAuthor verifies author filtering
func TestFilterByAuthor(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1", "Author 2"}, now, now, nil)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 3"}, now, now, nil)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?author=Author+2", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total, "should filter by author")
	assert.Contains(t, resp.Items[0].Authors, "Author 2")
}

// TestFilterBySince verifies since timestamp filtering
func TestFilterBySince(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1"}, now, now.Add(-2*time.Hour), nil)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 2"}, now, now.Add(-1*time.Hour), nil)
	item3 := createSampleItem(uuid.New(), "Publisher C", []string{"Author 3"}, now, now, nil)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))
	require.NoError(t, feed.Add(item3))

	sinceTime := now.Add(-90 * time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?since="+sinceTime.Format(time.RFC3339), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total, "should filter items discovered after since time")
}

// TestFilterBySince_InvalidFormat verifies error handling for invalid since
// parameter
func TestFilterBySince_InvalidFormat(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?since=invalid-date", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "invalid since format should return 400")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_parameter", errResp.Error.Code)
}

// TestFilterByUntil verifies until timestamp filtering
func TestFilterByUntil(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item1 := createSampleItem(uuid.New(), "Publisher A", []string{"Author 1"}, now, now.Add(-2*time.Hour), nil)
	item2 := createSampleItem(uuid.New(), "Publisher B", []string{"Author 2"}, now, now.Add(-1*time.Hour), nil)
	item3 := createSampleItem(uuid.New(), "Publisher C", []string{"Author 3"}, now, now, nil)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))
	require.NoError(t, feed.Add(item3))

	untilTime := now.Add(-90 * time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?until="+untilTime.Format(time.RFC3339), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp ListItemsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total, "should filter items discovered before until time")
}

// TestFilterByUntil_InvalidFormat verifies error handling for invalid until
// parameter
func TestFilterByUntil_InvalidFormat(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items?until=invalid-date", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "invalid until format should return 400")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_parameter", errResp.Error.Code)
}

// TestSortItems verifies various sorting options
func TestSortItems(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	pinnedTime1 := now.Add(-1 * time.Hour)
	pinnedTime2 := now

	item1 := createSampleItem(uuid.New(), "A", []string{"Author 1"}, now.Add(-3*time.Hour), now.Add(-3*time.Hour), nil)
	item2 := createSampleItem(uuid.New(), "B", []string{"Author 2"}, now.Add(-2*time.Hour), now.Add(-2*time.Hour), &pinnedTime1)
	item3 := createSampleItem(uuid.New(), "C", []string{"Author 3"}, now.Add(-1*time.Hour), now.Add(-1*time.Hour), &pinnedTime2)

	require.NoError(t, feed.Add(item1))
	require.NoError(t, feed.Add(item2))
	require.NoError(t, feed.Add(item3))

	tests := []struct {
		name          string
		sortParam     string
		expectedFirst string // publisher of first item
	}{
		{"default (published_desc)", "", "C"},
		{"published_desc", "published_desc", "C"},
		{"published_asc", "published_asc", "A"},
		{"discovered_desc", "discovered_desc", "C"},
		{"discovered_asc", "discovered_asc", "A"},
		{"pinned_desc", "pinned_desc", "C"}, // C has most recent pinned time
		{"pinned_asc", "pinned_asc", "A"},   // A has nil (comes first), then B (oldest pinned)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/items"
			if tt.sortParam != "" {
				url += "?sort=" + tt.sortParam
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			var resp ListItemsResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, 3, resp.Total)
			if len(resp.Items) > 0 && resp.Items[0].Publisher != nil {
				assert.Equal(t, tt.expectedFirst, *resp.Items[0].Publisher, "sort order should be correct")
			}
		})
	}

	// Test invalid sort separately (order is unpredictable)
	t.Run("invalid sort", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/items?sort=invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var resp ListItemsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 3, resp.Total, "should return all items even with invalid sort")
	})
}

// TestPagination verifies limit and offset parameters
func TestPagination(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	for i := range 100 {
		item := createSampleItem(uuid.New(), fmt.Sprintf("Publisher %d", i), []string{"Author"}, now.Add(time.Duration(-i)*time.Hour), now, nil)
		require.NoError(t, feed.Add(item))
	}

	tests := []struct {
		name          string
		limit         string
		offset        string
		expectedLimit int
		expectedCount int
	}{
		{"default pagination", "", "", 50, 50},
		{"custom limit", "10", "", 10, 10},
		{"with offset", "10", "5", 10, 10},
		{"offset at end", "10", "95", 10, 5}, // only 5 items remaining
		{"offset beyond end", "10", "200", 10, 0},
		{"max limit enforcement", "2000", "", 1000, 100}, // max is 1000
		{"limit=1", "1", "", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/items"
			params := []string{}
			if tt.limit != "" {
				params = append(params, "limit="+tt.limit)
			}
			if tt.offset != "" {
				params = append(params, "offset="+tt.offset)
			}
			if len(params) > 0 {
				url += "?" + strings.Join(params, "&")
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			var resp ListItemsResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, 100, resp.Total, "total should always be 100")
			assert.Equal(t, tt.expectedLimit, resp.Limit, "limit should match expected")
			assert.Len(t, resp.Items, tt.expectedCount, "item count should match expected")
		})
	}
}

// TestPagination_InvalidParameters verifies error handling for invalid
// pagination
func TestPagination_InvalidParameters(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	tests := []struct {
		name   string
		params string
	}{
		{"invalid limit", "?limit=invalid"},
		{"negative limit", "?limit=-1"},
		{"zero limit", "?limit=0"},
		{"invalid offset", "?offset=invalid"},
		{"negative offset", "?offset=-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/items"+tt.params, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, "invalid pagination should return 400")

			var errResp ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errResp)
			require.NoError(t, err)
			assert.Equal(t, "invalid_parameter", errResp.Error.Code)
		})
	}
}

// TestHandleGetItem_Success verifies successful item retrieval
func TestHandleGetItem_Success(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item := createSampleItem(uuid.New(), "Test Publisher", []string{"Test Author"}, now, now, nil)
	require.NoError(t, feed.Add(item))

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/items/%s", item.ID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp NewsItem
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, item.ID, resp.ID, "returned item should match")
	assert.Equal(t, "Test Publisher", *resp.Publisher)
}

// TestHandleGetItem_NotFound verifies 404 for non-existent item
func TestHandleGetItem_NotFound(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/items/%s", nonExistentID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 for non-existent item")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", errResp.Error.Code)
}

// TestHandleGetItem_InvalidID verifies 400 for invalid UUID
func TestHandleGetItem_InvalidID(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items/invalid-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 for invalid UUID")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_id", errResp.Error.Code)
}

// TestHandlePinItem_Success verifies successful item pinning
func TestHandlePinItem_Success(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item := createSampleItem(uuid.New(), "Test Publisher", []string{"Test Author"}, now, now, nil)
	require.NoError(t, feed.Add(item))
	require.Nil(t, item.PinnedAt, "item should not be pinned initially")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/pin", item.ID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp NewsItem
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.PinnedAt, "item should be pinned")
	assert.WithinDuration(t, time.Now(), *resp.PinnedAt, 2*time.Second, "pinned time should be recent")
}

// TestHandlePinItem_NotFound verifies 404 for non-existent item
func TestHandlePinItem_NotFound(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/pin", nonExistentID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 for non-existent item")
}

// TestHandlePinItem_InvalidID verifies 400 for invalid UUID
func TestHandlePinItem_InvalidID(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/items/invalid-uuid/pin", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 for invalid UUID")
}

// TestHandleUnpinItem_Success verifies successful item unpinning
func TestHandleUnpinItem_Success(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	pinnedTime := now
	item := createSampleItem(uuid.New(), "Test Publisher", []string{"Test Author"}, now, now, &pinnedTime)
	require.NoError(t, feed.Add(item))
	require.NotNil(t, item.PinnedAt, "item should be pinned initially")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/unpin", item.ID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp NewsItem
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp.PinnedAt, "item should be unpinned")
}

// TestHandleUnpinItem_NotFound verifies 404 for non-existent item
func TestHandleUnpinItem_NotFound(t *testing.T) {
	router, _ := setupTestNewsFeedRouter(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/unpin", nonExistentID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 for non-existent item")
}

// TestRouteItems verifies routing to correct handlers
func TestRouteItems(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item := createSampleItem(uuid.New(), "Test", []string{"Author"}, now, now, nil)
	require.NoError(t, feed.Add(item))

	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
	}{
		{"list items", http.MethodGet, "/api/v1/items", http.StatusOK},
		{"list items with slash", http.MethodGet, "/api/v1/items/", http.StatusMovedPermanently}, // Gin redirects
		{"get item", http.MethodGet, fmt.Sprintf("/api/v1/items/%s", item.ID), http.StatusOK},
		{"pin item", http.MethodPost, fmt.Sprintf("/api/v1/items/%s/pin", item.ID), http.StatusOK},
		{"unpin item", http.MethodPost, fmt.Sprintf("/api/v1/items/%s/unpin", item.ID), http.StatusOK},
		{"invalid route", http.MethodGet, "/api/v1/items/invalid/route/path", http.StatusNotFound},
		{"not found", http.MethodGet, "/api/v1/other", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code, "router should handle path correctly")
		})
	}
}

// Property test: Filtering should preserve item integrity
func TestFiltering_PreservesItemIntegrity(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	originalItem := createSampleItem(uuid.New(), "Test Publisher", []string{"Author 1", "Author 2"}, now, now, nil)
	require.NoError(t, feed.Add(originalItem))

	// Apply various filters
	filters := []string{
		"?publisher=Test+Publisher",
		"?author=Author+1",
		"?pinned=false",
		"?sort=published_desc",
		"?limit=10&offset=0",
	}

	for _, filter := range filters {
		t.Run(filter, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/items"+filter, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			var resp ListItemsResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			if len(resp.Items) > 0 {
				returnedItem := resp.Items[0]
				assert.Equal(t, originalItem.ID, returnedItem.ID, "ID should be preserved")
				assert.Equal(t, originalItem.Title, returnedItem.Title, "Title should be preserved")
				assert.Equal(t, originalItem.URL, returnedItem.URL, "URL should be preserved")
				assert.Equal(t, originalItem.Authors, returnedItem.Authors, "Authors should be preserved")
			}
		})
	}
}

// Property test: Pinning and unpinning should be idempotent
func TestPinUnpin_Idempotent(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	item := createSampleItem(uuid.New(), "Test", []string{"Author"}, now, now, nil)
	require.NoError(t, feed.Add(item))

	// Pin twice
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/pin", item.ID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "pinning should succeed each time")

		var resp NewsItem
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.PinnedAt, "item should remain pinned")
	}

	// Unpin twice
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/items/%s/unpin", item.ID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "unpinning should succeed each time")

		var resp NewsItem
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Nil(t, resp.PinnedAt, "item should remain unpinned")
	}
}

// Property test: Pagination should partition items without duplication
func TestPagination_NoOverlapOrGaps(t *testing.T) {
	router, feed := setupTestNewsFeedRouter(t)

	now := time.Now()
	itemCount := 25
	for i := range itemCount {
		item := createSampleItem(uuid.New(), fmt.Sprintf("Publisher %d", i), []string{"Author"}, now.Add(time.Duration(-i)*time.Hour), now, nil)
		require.NoError(t, feed.Add(item))
	}

	pageSize := 10
	seenIDs := make(map[uuid.UUID]bool)
	totalRetrieved := 0

	// Fetch all pages
	for offset := 0; offset < itemCount; offset += pageSize {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/items?limit=%d&offset=%d", pageSize, offset), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var resp ListItemsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, item := range resp.Items {
			assert.False(t, seenIDs[item.ID], "item should not appear in multiple pages")
			seenIDs[item.ID] = true
			totalRetrieved++
		}
	}

	assert.Equal(t, itemCount, totalRetrieved, "pagination should retrieve all items exactly once")
}
