package newsfed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// APIServer represents the HTTP API server. Implements RFC 4.
type APIServer struct {
	feed *NewsFeed
}

// NewAPIServer creates a new API server with the given news feed.
func NewAPIServer(feed *NewsFeed) *APIServer {
	return &APIServer{
		feed: feed,
	}
}

// ListItemsResponse represents the response for GET /api/v1/items. Implements
// RFC 4 section 3.1.
type ListItemsResponse struct {
	Items  []NewsItem `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// ErrorResponse represents an error response. Implements RFC 4 section 4.1.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code and message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HandleListItems handles GET /api/v1/items. Implements RFC 4 section 3.1.
func (s *APIServer) HandleListItems(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Get all items from feed
	allItems, err := s.feed.List()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list items: "+err.Error())
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Filter by pinned status (optional)
	if pinnedParam := query.Get("pinned"); pinnedParam != "" {
		allItems = s.filterByPinned(allItems, pinnedParam)
	}

	// Filter by publisher (optional)
	if publisher := query.Get("publisher"); publisher != "" {
		allItems = s.filterByPublisher(allItems, publisher)
	}

	// Filter by author (optional)
	if author := query.Get("author"); author != "" {
		allItems = s.filterByAuthor(allItems, author)
	}

	// Filter by since (optional)
	if since := query.Get("since"); since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid since parameter: must be ISO 8601 format")
			return
		}
		allItems = s.filterBySince(allItems, sinceTime)
	}

	// Filter by until (optional)
	if until := query.Get("until"); until != "" {
		untilTime, err := time.Parse(time.RFC3339, until)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid until parameter: must be ISO 8601 format")
			return
		}
		allItems = s.filterByUntil(allItems, untilTime)
	}

	// Sort items (default: published_desc)
	sortParam := query.Get("sort")
	if sortParam == "" {
		sortParam = "published_desc"
	}
	s.sortItems(allItems, sortParam)

	// Get total before pagination
	total := len(allItems)

	// Parse pagination parameters
	limit := 50 // default
	if limitParam := query.Get("limit"); limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil || parsedLimit < 1 {
			s.writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid limit parameter")
			return
		}
		if parsedLimit > 1000 {
			parsedLimit = 1000 // max per RFC 4
		}
		limit = parsedLimit
	}

	offset := 0 // default
	if offsetParam := query.Get("offset"); offsetParam != "" {
		parsedOffset, err := strconv.Atoi(offsetParam)
		if err != nil || parsedOffset < 0 {
			s.writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid offset parameter")
			return
		}
		offset = parsedOffset
	}

	// Apply pagination
	paginatedItems := s.paginate(allItems, offset, limit)

	// Write response
	response := ListItemsResponse{
		Items:  paginatedItems,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// filterByPinned filters items by pinned status.
func (s *APIServer) filterByPinned(items []NewsItem, pinnedParam string) []NewsItem {
	var filtered []NewsItem
	for _, item := range items {
		if pinnedParam == "true" && item.PinnedAt != nil {
			filtered = append(filtered, item)
		} else if pinnedParam == "false" && item.PinnedAt == nil {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterByPublisher filters items by publisher name (exact match).
func (s *APIServer) filterByPublisher(items []NewsItem, publisher string) []NewsItem {
	var filtered []NewsItem
	for _, item := range items {
		if item.Publisher != nil && *item.Publisher == publisher {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterByAuthor filters items by author name (matches any author in array).
func (s *APIServer) filterByAuthor(items []NewsItem, author string) []NewsItem {
	var filtered []NewsItem
	for _, item := range items {
		if slices.Contains(item.Authors, author) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterBySince filters items discovered after the given time.
func (s *APIServer) filterBySince(items []NewsItem, since time.Time) []NewsItem {
	var filtered []NewsItem
	for _, item := range items {
		if item.DiscoveredAt.After(since) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterByUntil filters items discovered before the given time.
func (s *APIServer) filterByUntil(items []NewsItem, until time.Time) []NewsItem {
	var filtered []NewsItem
	for _, item := range items {
		if item.DiscoveredAt.Before(until) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// sortItems sorts items by the given sort parameter.
func (s *APIServer) sortItems(items []NewsItem, sortParam string) {
	switch sortParam {
	case "published_desc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		})
	case "published_asc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.Before(items[j].PublishedAt)
		})
	case "discovered_desc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].DiscoveredAt.After(items[j].DiscoveredAt)
		})
	case "discovered_asc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].DiscoveredAt.Before(items[j].DiscoveredAt)
		})
	case "pinned_desc":
		sort.Slice(items, func(i, j int) bool {
			if items[i].PinnedAt == nil {
				return false
			}
			if items[j].PinnedAt == nil {
				return true
			}
			return items[i].PinnedAt.After(*items[j].PinnedAt)
		})
	case "pinned_asc":
		sort.Slice(items, func(i, j int) bool {
			if items[i].PinnedAt == nil {
				return true
			}
			if items[j].PinnedAt == nil {
				return false
			}
			return items[i].PinnedAt.Before(*items[j].PinnedAt)
		})
	}
}

// paginate returns a slice of items for the given offset and limit.
func (s *APIServer) paginate(items []NewsItem, offset, limit int) []NewsItem {
	if offset >= len(items) {
		return []NewsItem{}
	}

	end := min(offset+limit, len(items))

	return items[offset:end]
}

// HandleGetItem handles GET /api/v1/items/{id}. Implements RFC 4 section 3.2.
func (s *APIServer) HandleGetItem(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Parse UUID from path
	id, err := s.parseItemID(r.URL.Path, "/api/v1/items/")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid item ID: "+err.Error())
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get item: "+err.Error())
		return
	}
	if item == nil {
		s.writeError(w, http.StatusNotFound, "not_found", "News item with ID "+id.String()+" not found")
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item)
}

// HandlePinItem handles POST /api/v1/items/{id}/pin. Implements RFC 4 section
// 3.3.
func (s *APIServer) HandlePinItem(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Parse UUID from path
	id, err := s.parseItemID(r.URL.Path, "/api/v1/items/")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid item ID: "+err.Error())
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get item: "+err.Error())
		return
	}
	if item == nil {
		s.writeError(w, http.StatusNotFound, "not_found", "News item with ID "+id.String()+" not found")
		return
	}

	// Set pinned_at to current time
	now := time.Now()
	item.PinnedAt = &now

	// Update item in feed
	if err := s.feed.Update(*item); err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update item: "+err.Error())
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item)
}

// HandleUnpinItem handles POST /api/v1/items/{id}/unpin. Implements RFC 4
// section 3.4.
func (s *APIServer) HandleUnpinItem(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Parse UUID from path
	id, err := s.parseItemID(r.URL.Path, "/api/v1/items/")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid item ID: "+err.Error())
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get item: "+err.Error())
		return
	}
	if item == nil {
		s.writeError(w, http.StatusNotFound, "not_found", "News item with ID "+id.String()+" not found")
		return
	}

	// Set pinned_at to nil
	item.PinnedAt = nil

	// Update item in feed
	if err := s.feed.Update(*item); err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update item: "+err.Error())
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item)
}

// parseItemID extracts a UUID from the URL path.
func (s *APIServer) parseItemID(path, prefix string) (uuid.UUID, error) {
	// Remove prefix and any trailing path
	path = strings.TrimPrefix(path, prefix)

	// Split by / to get the ID part
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return uuid.Nil, fmt.Errorf("no ID provided")
	}

	// Parse UUID
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// writeError writes an error response. Implements RFC 4 section 4.1.
func (s *APIServer) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// corsMiddleware adds CORS headers to responses. Implements RFC 4 section 6.
func (s *APIServer) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers per RFC 4 section 6.1
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests per RFC 4 section 6.2
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server on the given address. Implements RFC 4 section
// 2.
func (s *APIServer) Start(addr string) error {
	mux := http.NewServeMux()

	// Register routes - need both with and without trailing slash to avoid 301
	mux.HandleFunc("/api/v1/items", s.RouteItems)
	mux.HandleFunc("/api/v1/items/", s.RouteItems)

	// Wrap with CORS middleware per RFC 4 section 6
	handler := s.CORSMiddleware(mux)

	return http.ListenAndServe(addr, handler)
}

// routeItems routes /api/v1/items/* requests to appropriate handlers.
func (s *APIServer) RouteItems(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// GET /api/v1/items - List items
	if path == "/api/v1/items" || path == "/api/v1/items/" {
		s.HandleListItems(w, r)
		return
	}

	// Check if path has an ID
	if !strings.HasPrefix(path, "/api/v1/items/") {
		s.writeError(w, http.StatusNotFound, "not_found", "Not found")
		return
	}

	// Parse the path after /api/v1/items/
	suffix := strings.TrimPrefix(path, "/api/v1/items/")
	parts := strings.Split(suffix, "/")

	if len(parts) == 1 {
		// GET /api/v1/items/{id} - Get single item
		s.HandleGetItem(w, r)
	} else if len(parts) == 2 && parts[1] == "pin" {
		// POST /api/v1/items/{id}/pin - Pin item
		s.HandlePinItem(w, r)
	} else if len(parts) == 2 && parts[1] == "unpin" {
		// POST /api/v1/items/{id}/unpin - Unpin item
		s.HandleUnpinItem(w, r)
	} else {
		s.writeError(w, http.StatusNotFound, "not_found", "Not found")
	}
}
// CORSMiddleware is a standalone middleware for adding CORS headers.
// Can be used by any HTTP handler.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
