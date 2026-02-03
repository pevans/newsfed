package newsfed

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MetadataAPIServer represents the HTTP API server for metadata management.
// Implements RFC 6.
type MetadataAPIServer struct {
	store *MetadataStore
}

// NewMetadataAPIServer creates a new metadata API server.
func NewMetadataAPIServer(store *MetadataStore) *MetadataAPIServer {
	return &MetadataAPIServer{
		store: store,
	}
}

// ListSourcesResponse represents the response for GET /api/v1/meta/sources.
// Implements RFC 6 section 3.1.
type ListSourcesResponse struct {
	Sources []Source `json:"sources"`
	Total   int      `json:"total"`
}

// CreateSourceRequest represents the request for POST /api/v1/meta/sources.
// Implements RFC 6 section 3.3.
type CreateSourceRequest struct {
	SourceType      string         `json:"source_type"`
	URL             string         `json:"url"`
	Name            string         `json:"name"`
	PollingInterval *string        `json:"polling_interval,omitempty"`
	ScraperConfig   *ScraperConfig `json:"scraper_config,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"` // Default: true
}

// UpdateSourceRequest represents the request for PUT
// /api/v1/meta/sources/{id}. Implements RFC 6 section 3.4.
type UpdateSourceRequest struct {
	Name            *string        `json:"name,omitempty"`
	URL             *string        `json:"url,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"`
	PollingInterval *string        `json:"polling_interval,omitempty"`
	ScraperConfig   *ScraperConfig `json:"scraper_config,omitempty"`
}

// HandleListSources handles GET /api/v1/meta/sources. Implements RFC 6
// section 3.1.
func (s *MetadataAPIServer) HandleListSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	sources, err := s.store.ListSources()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list sources")
		return
	}

	// Apply filters
	typeFilter := r.URL.Query().Get("type")
	enabledFilter := r.URL.Query().Get("enabled")

	var filtered []Source
	for _, source := range sources {
		// Filter by type
		if typeFilter != "" && source.SourceType != typeFilter {
			continue
		}

		// Filter by enabled status
		if enabledFilter == "true" && source.EnabledAt == nil {
			continue
		}
		if enabledFilter == "false" && source.EnabledAt != nil {
			continue
		}

		filtered = append(filtered, source)
	}

	response := ListSourcesResponse{
		Sources: filtered,
		Total:   len(filtered),
	}

	writeJSON(w, http.StatusOK, response)
}

// HandleGetSource handles GET /api/v1/meta/sources/{id}. Implements RFC 6
// section 3.2.
func (s *MetadataAPIServer) HandleGetSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Extract source_id from path
	sourceID, err := extractSourceID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}

	source, err := s.store.GetSource(sourceID)
	if err != nil {
		if err.Error() == "source not found" {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Source with ID %s not found", sourceID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve source")
		return
	}

	writeJSON(w, http.StatusOK, source)
}

// HandleCreateSource handles POST /api/v1/meta/sources. Implements RFC 6
// section 3.3.
func (s *MetadataAPIServer) HandleCreateSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Read body once to check for explicit null values
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
		return
	}

	// Check if enabled_at was explicitly provided
	var rawBody map[string]any
	if err := json.Unmarshal(bodyBytes, &rawBody); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// Now unmarshal into typed struct
	var req CreateSourceRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// Validate required fields
	if req.SourceType == "" || req.URL == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Missing required fields: source_type, url, name")
		return
	}

	// Validate source_type
	if req.SourceType != "rss" && req.SourceType != "atom" && req.SourceType != "website" {
		writeError(w, http.StatusBadRequest, "validation_error", "source_type must be one of: rss, atom, website")
		return
	}

	// Validate scraper_config for website sources
	if req.SourceType == "website" && req.ScraperConfig == nil {
		writeError(w, http.StatusBadRequest, "validation_error", "scraper_config is required for website sources")
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if _, err := time.ParseDuration(*req.PollingInterval); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid polling_interval: must be a valid duration (e.g., 1h, 30m)")
			return
		}
	}

	// Determine enabled_at value from boolean enabled field
	var enabledAt *time.Time
	if req.Enabled == nil {
		// Field not provided - default to enabled
		now := time.Now()
		enabledAt = &now
	} else if *req.Enabled {
		// Explicitly enabled - set to current time
		now := time.Now()
		enabledAt = &now
	} else {
		// Explicitly disabled - set to nil
		enabledAt = nil
	}

	// Create source
	source, err := s.store.CreateSource(req.SourceType, req.URL, req.Name, req.ScraperConfig, enabledAt)
	if err != nil {
		// Check for duplicate URL (SQLite can return various formats)
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "unique constraint") {
			writeError(w, http.StatusConflict, "conflict", "Source with this URL already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create source")
		return
	}

	// Set polling_interval if provided
	if req.PollingInterval != nil {
		updates := map[string]any{
			"polling_interval": *req.PollingInterval,
		}
		s.store.UpdateSource(source.SourceID, updates)
		source.PollingInterval = req.PollingInterval
	}

	writeJSON(w, http.StatusCreated, source)
}

// HandleUpdateSource handles PUT /api/v1/meta/sources/{id}. Implements RFC 6
// section 3.4.
func (s *MetadataAPIServer) HandleUpdateSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Extract source_id from path
	sourceID, err := extractSourceID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}

	// Read body to check for explicit null values
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
		return
	}

	// Check which fields were provided
	var rawBody map[string]any
	if err := json.Unmarshal(bodyBytes, &rawBody); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// Unmarshal into typed struct
	var req UpdateSourceRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if _, err := time.ParseDuration(*req.PollingInterval); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid polling_interval: must be a valid duration (e.g., 1h, 30m)")
			return
		}
	}

	// Build updates map
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	// Handle enabled - convert boolean to enabled_at timestamp
	if req.Enabled != nil {
		if *req.Enabled {
			// Enable - set to current time
			now := time.Now()
			updates["enabled_at"] = &now
		} else {
			// Disable - set to nil
			var nilTime *time.Time
			updates["enabled_at"] = nilTime
		}
	}
	if req.PollingInterval != nil {
		updates["polling_interval"] = *req.PollingInterval
	}
	if req.ScraperConfig != nil {
		updates["scraper_config"] = req.ScraperConfig
	}

	// Update source
	err = s.store.UpdateSource(sourceID, updates)
	if err != nil {
		if err.Error() == "source not found" {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Source with ID %s not found", sourceID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update source")
		return
	}

	// Return updated source
	source, err := s.store.GetSource(sourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve updated source")
		return
	}

	writeJSON(w, http.StatusOK, source)
}

// HandleDeleteSource handles DELETE /api/v1/meta/sources/{id}. Implements RFC
// 6 section 3.5.
func (s *MetadataAPIServer) HandleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Extract source_id from path
	sourceID, err := extractSourceID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid source ID")
		return
	}

	err = s.store.DeleteSource(sourceID)
	if err != nil {
		if err.Error() == "source not found" {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Source with ID %s not found", sourceID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetConfig handles GET /api/v1/meta/config. Implements RFC 6 section
// 3.6.
func (s *MetadataAPIServer) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	config, err := s.store.GetConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configuration")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// HandleUpdateConfig handles PUT /api/v1/meta/config. Implements RFC 6
// section 3.7.
func (s *MetadataAPIServer) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Read body to check what fields are provided
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
		return
	}

	// Parse as map to check which fields are present
	var rawBody map[string]any
	if err := json.Unmarshal(bodyBytes, &rawBody); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// If no fields provided, return current config unchanged
	if len(rawBody) == 0 {
		config, err := s.store.GetConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configuration")
			return
		}
		writeJSON(w, http.StatusOK, config)
		return
	}

	// Unmarshal into typed struct
	var updates Config
	if err := json.Unmarshal(bodyBytes, &updates); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	// Validate default_polling_interval if provided
	if pollingInterval, ok := rawBody["default_polling_interval"].(string); ok {
		if _, err := time.ParseDuration(pollingInterval); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid default_polling_interval: must be a valid duration (e.g., 1h, 30m)")
			return
		}
	}

	// Update config
	err = s.store.UpdateConfig(&updates)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update configuration")
		return
	}

	writeJSON(w, http.StatusOK, updates)
}

// extractSourceID extracts the source ID from the URL path. Path format:
// /api/v1/meta/sources/{source_id}
func extractSourceID(path string) (uuid.UUID, error) {
	// Split path and get last segment
	parts := splitPath(path)
	if len(parts) < 5 {
		return uuid.UUID{}, fmt.Errorf("invalid path")
	}

	sourceIDStr := parts[len(parts)-1]
	return uuid.Parse(sourceIDStr)
}

// routeSources routes /api/v1/meta/sources/* requests to appropriate
// handlers.
func (s *MetadataAPIServer) RouteSources(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /api/v1/meta/sources (no ID)
	if path == "/api/v1/meta/sources" || path == "/api/v1/meta/sources/" {
		switch r.Method {
		case http.MethodGet:
			s.HandleListSources(w, r)
		case http.MethodPost:
			s.HandleCreateSource(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		}
		return
	}

	// Handle /api/v1/meta/sources/{id} (with ID)
	switch r.Method {
	case http.MethodGet:
		s.HandleGetSource(w, r)
	case http.MethodPut:
		s.HandleUpdateSource(w, r)
	case http.MethodDelete:
		s.HandleDeleteSource(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// routeConfig routes /api/v1/meta/config requests to appropriate handlers.
func (s *MetadataAPIServer) RouteConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.HandleGetConfig(w, r)
	case http.MethodPut:
		s.HandleUpdateConfig(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	writeJSON(w, status, response)
}

func splitPath(path string) []string {
	var parts []string
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
