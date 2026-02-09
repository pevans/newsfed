package newsfed

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pevans/newsfed/newsfeed"
)

// APIServer represents the HTTP API server. Implements RFC 4.
type APIServer struct {
	feed *newsfeed.NewsFeed
}

// NewAPIServer creates a new API server with the given news feed.
func NewAPIServer(feed *newsfeed.NewsFeed) *APIServer {
	return &APIServer{
		feed: feed,
	}
}

// SetupRouter configures the Gin router with all newsfeed API routes
func (s *APIServer) SetupRouter() *gin.Engine {
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})

	api := router.Group("/api/v1/items")
	api.GET("", s.HandleListItems)
	api.GET("/:id", s.HandleGetItem)
	api.POST("/:id/pin", s.HandlePinItem)
	api.POST("/:id/unpin", s.HandleUnpinItem)

	return router
}

// ListItemsResponse represents the response for GET /api/v1/items. Implements
// RFC 4 section 3.1.
type ListItemsResponse struct {
	Items  []newsfeed.NewsItem `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
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
func (s *APIServer) HandleListItems(c *gin.Context) {
	// Get all items from feed
	allItems, err := s.feed.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to list items: " + err.Error(),
			},
		})
		return
	}

	// Filter by pinned status (optional)
	if pinnedParam := c.Query("pinned"); pinnedParam != "" {
		allItems = s.filterByPinned(allItems, pinnedParam)
	}

	// Filter by publisher (optional)
	if publisher := c.Query("publisher"); publisher != "" {
		allItems = s.filterByPublisher(allItems, publisher)
	}

	// Filter by author (optional)
	if author := c.Query("author"); author != "" {
		allItems = s.filterByAuthor(allItems, author)
	}

	// Filter by since (optional)
	if since := c.Query("since"); since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "invalid_parameter",
					"message": "Invalid since parameter: must be ISO 8601 format",
				},
			})
			return
		}
		allItems = s.filterBySince(allItems, sinceTime)
	}

	// Filter by until (optional)
	if until := c.Query("until"); until != "" {
		untilTime, err := time.Parse(time.RFC3339, until)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "invalid_parameter",
					"message": "Invalid until parameter: must be ISO 8601 format",
				},
			})
			return
		}
		allItems = s.filterByUntil(allItems, untilTime)
	}

	// Sort items (default: published_desc)
	sortParam := c.Query("sort")
	if sortParam == "" {
		sortParam = "published_desc"
	}
	s.sortItems(allItems, sortParam)

	// Get total before pagination
	total := len(allItems)

	// Parse pagination parameters
	limit := 50 // default
	if limitParam := c.Query("limit"); limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil || parsedLimit < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "invalid_parameter",
					"message": "Invalid limit parameter",
				},
			})
			return
		}
		if parsedLimit > 1000 {
			parsedLimit = 1000 // max per RFC 4
		}
		limit = parsedLimit
	}

	offset := 0 // default
	if offsetParam := c.Query("offset"); offsetParam != "" {
		parsedOffset, err := strconv.Atoi(offsetParam)
		if err != nil || parsedOffset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "invalid_parameter",
					"message": "Invalid offset parameter",
				},
			})
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

	c.JSON(http.StatusOK, response)
}

// filterByPinned filters items by pinned status.
func (s *APIServer) filterByPinned(items []newsfeed.NewsItem, pinnedParam string) []newsfeed.NewsItem {
	var filtered []newsfeed.NewsItem
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
func (s *APIServer) filterByPublisher(items []newsfeed.NewsItem, publisher string) []newsfeed.NewsItem {
	var filtered []newsfeed.NewsItem
	for _, item := range items {
		if item.Publisher != nil && *item.Publisher == publisher {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterByAuthor filters items by author name (matches any author in array).
func (s *APIServer) filterByAuthor(items []newsfeed.NewsItem, author string) []newsfeed.NewsItem {
	var filtered []newsfeed.NewsItem
	for _, item := range items {
		if slices.Contains(item.Authors, author) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterBySince filters items discovered after the given time.
func (s *APIServer) filterBySince(items []newsfeed.NewsItem, since time.Time) []newsfeed.NewsItem {
	var filtered []newsfeed.NewsItem
	for _, item := range items {
		if item.DiscoveredAt.After(since) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterByUntil filters items discovered before the given time.
func (s *APIServer) filterByUntil(items []newsfeed.NewsItem, until time.Time) []newsfeed.NewsItem {
	var filtered []newsfeed.NewsItem
	for _, item := range items {
		if item.DiscoveredAt.Before(until) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// sortItems sorts items by the given sort parameter.
func (s *APIServer) sortItems(items []newsfeed.NewsItem, sortParam string) {
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
func (s *APIServer) paginate(items []newsfeed.NewsItem, offset, limit int) []newsfeed.NewsItem {
	if offset >= len(items) {
		return []newsfeed.NewsItem{}
	}

	end := min(offset+limit, len(items))

	return items[offset:end]
}

// HandleGetItem handles GET /api/v1/items/{id}. Implements RFC 4 section 3.2.
func (s *APIServer) HandleGetItem(c *gin.Context) {
	// Parse UUID from path
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_id",
				"message": "Invalid item ID: " + err.Error(),
			},
		})
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to get item: " + err.Error(),
			},
		})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "not_found",
				"message": "News item with ID " + id.String() + " not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, item)
}

// HandlePinItem handles POST /api/v1/items/{id}/pin. Implements RFC 4 section
// 3.3.
func (s *APIServer) HandlePinItem(c *gin.Context) {
	// Parse UUID from path
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_id",
				"message": "Invalid item ID: " + err.Error(),
			},
		})
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to get item: " + err.Error(),
			},
		})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "not_found",
				"message": "News item with ID " + id.String() + " not found",
			},
		})
		return
	}

	// Set pinned_at to current time
	now := time.Now()
	item.PinnedAt = &now

	// Update item in feed
	if err := s.feed.Update(*item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to update item: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, item)
}

// HandleUnpinItem handles POST /api/v1/items/{id}/unpin. Implements RFC 4
// section 3.4.
func (s *APIServer) HandleUnpinItem(c *gin.Context) {
	// Parse UUID from path
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "invalid_id",
				"message": "Invalid item ID: " + err.Error(),
			},
		})
		return
	}

	// Get item from feed
	item, err := s.feed.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to get item: " + err.Error(),
			},
		})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "not_found",
				"message": "News item with ID " + id.String() + " not found",
			},
		})
		return
	}

	// Set pinned_at to nil
	item.PinnedAt = nil

	// Update item in feed
	if err := s.feed.Update(*item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to update item: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, item)
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
