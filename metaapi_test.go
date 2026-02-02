package newsfed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: create a test metadata store
func setupTestStore(t *testing.T) *MetadataStore {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewMetadataStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

// Test helper: create a test server
func setupTestServer(t *testing.T) (*MetadataAPIServer, *MetadataStore) {
	store := setupTestStore(t)
	server := NewMetadataAPIServer(store)
	return server, store
}

// TestSplitPath verifies path splitting behavior
func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/api/v1/meta/sources", []string{"api", "v1", "meta", "sources"}},
		{"/api/v1/meta/sources/", []string{"api", "v1", "meta", "sources"}},
		{"api/v1/meta/sources", []string{"api", "v1", "meta", "sources"}},
		{"/", nil},
		{"", nil},
		{"//api//v1//", []string{"api", "v1"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result, "splitPath should handle various path formats")
		})
	}
}

// TestExtractSourceID verifies source ID extraction from URLs
func TestExtractSourceID(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "valid path with UUID",
			path:      fmt.Sprintf("/api/v1/meta/sources/%s", validID),
			expectErr: false,
		},
		{
			name:      "path too short",
			path:      "/api/v1",
			expectErr: true,
		},
		{
			name:      "invalid UUID",
			path:      "/api/v1/meta/sources/not-a-uuid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := extractSourceID(tt.path)
			if tt.expectErr {
				assert.Error(t, err, "extractSourceID should error on invalid paths")
			} else {
				assert.NoError(t, err, "extractSourceID should succeed on valid paths")
				assert.Equal(t, validID, id, "extracted ID should match")
			}
		})
	}
}

// TestWriteJSON verifies JSON response writing
func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code, "status code should match")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "content-type should be JSON")

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err, "response should be valid JSON")
	assert.Equal(t, data, result, "response body should match input data")
}

// TestWriteError verifies error response writing
func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, http.StatusBadRequest, "test_error", "Test message")

	assert.Equal(t, http.StatusBadRequest, w.Code, "status code should match")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err, "error response should be valid JSON")
	assert.Equal(t, "test_error", errResp.Error.Code, "error code should match")
	assert.Equal(t, "Test message", errResp.Error.Message, "error message should match")
}

// TestHandleListSources_MethodValidation ensures only GET is allowed
func TestHandleListSources_MethodValidation(t *testing.T) {
	server, _ := setupTestServer(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/meta/sources", nil)
			w := httptest.NewRecorder()

			server.HandleListSources(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "non-GET methods should return 405")
		})
	}
}

// TestHandleListSources_EmptyList verifies behavior with no sources
func TestHandleListSources_EmptyList(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta/sources", nil)
	w := httptest.NewRecorder()

	server.HandleListSources(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "should return 200 for empty list")

	var resp ListSourcesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total, "total should be zero for empty list")
	assert.Empty(t, resp.Sources, "sources list should be empty")
}

// TestHandleListSources_WithFilters tests type and enabled filters
func TestHandleListSources_WithFilters(t *testing.T) {
	server, store := setupTestServer(t)

	// Create test sources
	now := time.Now()
	store.CreateSource("rss", "http://example.com/rss1", "RSS Feed 1", nil, &now)
	store.CreateSource("atom", "http://example.com/atom1", "Atom Feed 1", nil, &now)
	store.CreateSource("rss", "http://example.com/rss2", "RSS Feed 2", nil, nil) // disabled

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
	}{
		{"no filter", "", 3},
		{"type filter rss", "?type=rss", 2},
		{"type filter atom", "?type=atom", 1},
		{"enabled filter true", "?enabled=true", 2},
		{"enabled filter false", "?enabled=false", 1},
		{"combined filters", "?type=rss&enabled=true", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/meta/sources"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			server.HandleListSources(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp ListSourcesResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, resp.Total, "filter should return correct count")
		})
	}
}

// TestHandleGetSource_Success verifies successful source retrieval
func TestHandleGetSource_Success(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/meta/sources/%s", source.SourceID), nil)
	w := httptest.NewRecorder()

	server.HandleGetSource(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Source
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, source.SourceID, resp.SourceID, "returned source should match created source")
	assert.Equal(t, "Test Feed", resp.Name)
}

// TestHandleGetSource_NotFound verifies 404 for non-existent source
func TestHandleGetSource_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/meta/sources/%s", nonExistentID), nil)
	w := httptest.NewRecorder()

	server.HandleGetSource(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 for non-existent source")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "not_found", errResp.Error.Code)
}

// TestHandleGetSource_InvalidID verifies 400 for invalid UUID
func TestHandleGetSource_InvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta/sources/invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.HandleGetSource(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 for invalid UUID")
}

// TestHandleCreateSource_Success verifies successful source creation
func TestHandleCreateSource_Success(t *testing.T) {
	server, _ := setupTestServer(t)

	reqBody := CreateSourceRequest{
		SourceType: "rss",
		URL:        "http://example.com/feed",
		Name:       "Test Feed",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleCreateSource(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "successful creation should return 201")

	var resp Source
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Test Feed", resp.Name)
	assert.Equal(t, "rss", resp.SourceType)
	assert.NotNil(t, resp.EnabledAt, "source should be enabled by default")
}

// TestHandleCreateSource_ExplicitlyDisabled verifies disabled source creation
func TestHandleCreateSource_ExplicitlyDisabled(t *testing.T) {
	server, _ := setupTestServer(t)

	// Use raw JSON to send explicit null
	bodyJSON := `{"source_type":"rss","url":"http://example.com/feed","name":"Test Feed","enabled_at":null}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.HandleCreateSource(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp Source
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp.EnabledAt, "explicitly disabled source should have nil EnabledAt")
}

// TestHandleCreateSource_ValidationErrors tests various validation failures
func TestHandleCreateSource_ValidationErrors(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		name         string
		body         string
		expectedCode string
	}{
		{
			name:         "missing required fields",
			body:         `{"source_type":"rss"}`,
			expectedCode: "validation_error",
		},
		{
			name:         "invalid source type",
			body:         `{"source_type":"invalid","url":"http://example.com","name":"Test"}`,
			expectedCode: "validation_error",
		},
		{
			name:         "website without scraper config",
			body:         `{"source_type":"website","url":"http://example.com","name":"Test"}`,
			expectedCode: "validation_error",
		},
		{
			name:         "invalid polling interval",
			body:         `{"source_type":"rss","url":"http://example.com","name":"Test","polling_interval":"invalid"}`,
			expectedCode: "validation_error",
		},
		{
			name:         "invalid JSON",
			body:         `{invalid json}`,
			expectedCode: "bad_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			server.HandleCreateSource(w, req)

			assert.True(t, w.Code >= 400, "validation error should return 4xx status")

			var errResp ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errResp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, errResp.Error.Code, "error code should match expected")
		})
	}
}

// TestHandleCreateSource_DuplicateURL verifies duplicate URL handling
func TestHandleCreateSource_DuplicateURL(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	store.CreateSource("rss", "http://example.com/feed", "Feed 1", nil, &now)

	reqBody := CreateSourceRequest{
		SourceType: "rss",
		URL:        "http://example.com/feed",
		Name:       "Feed 2",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleCreateSource(w, req)

	assert.Equal(t, http.StatusConflict, w.Code, "duplicate URL should return 409")

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "conflict", errResp.Error.Code)
}

// TestHandleCreateSource_WithPollingInterval verifies polling interval
// handling
func TestHandleCreateSource_WithPollingInterval(t *testing.T) {
	server, _ := setupTestServer(t)

	interval := "30m"
	reqBody := CreateSourceRequest{
		SourceType:      "rss",
		URL:             "http://example.com/feed",
		Name:            "Test Feed",
		PollingInterval: &interval,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleCreateSource(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp Source
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.PollingInterval)
	assert.Equal(t, "30m", *resp.PollingInterval)
}

// TestHandleUpdateSource_Success verifies successful source update
func TestHandleUpdateSource_Success(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Old Name", nil, &now)
	require.NoError(t, err)

	newName := "New Name"
	reqBody := UpdateSourceRequest{
		Name: &newName,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/meta/sources/%s", source.SourceID), bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleUpdateSource(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Source
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "New Name", resp.Name, "name should be updated")
}

// TestHandleUpdateSource_DisableSource verifies disabling a source
func TestHandleUpdateSource_DisableSource(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	// Use raw JSON to send explicit null
	bodyJSON := `{"enabled_at":null}`

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/meta/sources/%s", source.SourceID), strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.HandleUpdateSource(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Source
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp.EnabledAt, "source should be disabled")
}

// TestHandleUpdateSource_NotFound verifies 404 for non-existent source
func TestHandleUpdateSource_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	nonExistentID := uuid.New()
	newName := "New Name"
	reqBody := UpdateSourceRequest{
		Name: &newName,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/meta/sources/%s", nonExistentID), bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleUpdateSource(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleDeleteSource_Success verifies successful source deletion
func TestHandleDeleteSource_Success(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Test Feed", nil, &now)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/meta/sources/%s", source.SourceID), nil)
	w := httptest.NewRecorder()

	server.HandleDeleteSource(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code, "successful deletion should return 204")

	// Verify source is actually deleted
	_, err = store.GetSource(source.SourceID)
	assert.Error(t, err, "deleted source should not be retrievable")
}

// TestHandleDeleteSource_NotFound verifies 404 for non-existent source
func TestHandleDeleteSource_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/meta/sources/%s", nonExistentID), nil)
	w := httptest.NewRecorder()

	server.HandleDeleteSource(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleGetConfig_Success verifies config retrieval
func TestHandleGetConfig_Success(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta/config", nil)
	w := httptest.NewRecorder()

	server.HandleGetConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Config
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.DefaultPollingInterval, "config should have default polling interval")
}

// TestHandleUpdateConfig_Success verifies config update
func TestHandleUpdateConfig_Success(t *testing.T) {
	server, _ := setupTestServer(t)

	reqBody := Config{
		DefaultPollingInterval: "2h",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/meta/config", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleUpdateConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Config
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "2h", resp.DefaultPollingInterval)
}

// TestHandleUpdateConfig_InvalidDuration verifies validation of polling
// interval
func TestHandleUpdateConfig_InvalidDuration(t *testing.T) {
	server, _ := setupTestServer(t)

	bodyJSON := `{"default_polling_interval":"invalid"}`

	req := httptest.NewRequest(http.MethodPut, "/api/v1/meta/config", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.HandleUpdateConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "validation_error", errResp.Error.Code)
}

// TestHandleUpdateConfig_EmptyBody verifies behavior with empty update
func TestHandleUpdateConfig_EmptyBody(t *testing.T) {
	server, _ := setupTestServer(t)

	bodyJSON := `{}`

	req := httptest.NewRequest(http.MethodPut, "/api/v1/meta/config", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	server.HandleUpdateConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "empty update should return current config")
}

// TestRouteSources_MethodRouting verifies routing delegates to correct
// handlers
func TestRouteSources_MethodRouting(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		method       string
		path         string
		expectedCode int
	}{
		{http.MethodGet, "/api/v1/meta/sources", http.StatusOK},
		{http.MethodPost, "/api/v1/meta/sources", http.StatusBadRequest}, // will fail validation but routes correctly
		{http.MethodOptions, "/api/v1/meta/sources", http.StatusOK},
		{http.MethodPatch, "/api/v1/meta/sources", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.RouteSources(w, req)

			assert.Equal(t, tt.expectedCode, w.Code, "router should handle method correctly")
		})
	}
}

// TestRouteConfig_MethodRouting verifies config routing
func TestRouteConfig_MethodRouting(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		method       string
		expectedCode int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPut, http.StatusBadRequest}, // will fail with bad body but routes correctly
		{http.MethodOptions, http.StatusOK},
		{http.MethodPost, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/meta/config", nil)
			w := httptest.NewRecorder()

			server.RouteConfig(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

// Property test: Any valid source type should be accepted in creation
func TestCreateSource_AcceptsAllValidSourceTypes(t *testing.T) {
	server, _ := setupTestServer(t)
	validTypes := []string{"rss", "atom", "website"}

	for i, sourceType := range validTypes {
		t.Run(sourceType, func(t *testing.T) {
			reqBody := CreateSourceRequest{
				SourceType: sourceType,
				URL:        fmt.Sprintf("http://example.com/feed%d", i),
				Name:       fmt.Sprintf("Test %s", sourceType),
			}

			// Website requires scraper config
			if sourceType == "website" {
				reqBody.ScraperConfig = &ScraperConfig{
					DiscoveryMode: "direct",
					ArticleConfig: ArticleConfig{
						TitleSelector:   "h1",
						ContentSelector: "article",
					},
				}
			}

			bodyBytes, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/sources", bytes.NewReader(bodyBytes))
			w := httptest.NewRecorder()

			server.HandleCreateSource(w, req)

			assert.Equal(t, http.StatusCreated, w.Code, "valid source type should be accepted")
		})
	}
}

// Property test: Any source update should preserve unchanged fields
func TestUpdateSource_PreservesUnchangedFields(t *testing.T) {
	server, store := setupTestServer(t)

	now := time.Now()
	source, err := store.CreateSource("rss", "http://example.com/feed", "Original Name", nil, &now)
	require.NoError(t, err)

	// Update only the name
	newName := "Updated Name"
	reqBody := UpdateSourceRequest{
		Name: &newName,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/meta/sources/%s", source.SourceID), bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	server.HandleUpdateSource(w, req)

	var resp Source
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify unchanged fields are preserved
	assert.Equal(t, source.URL, resp.URL, "URL should be unchanged")
	assert.Equal(t, source.SourceType, resp.SourceType, "SourceType should be unchanged")
	assert.Equal(t, source.SourceID, resp.SourceID, "SourceID should be unchanged")
	assert.Equal(t, "Updated Name", resp.Name, "Name should be updated")
}
