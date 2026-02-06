package sources

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pevans/newsfed/scraper"
)

// SourceAPIServer represents the HTTP API server for source management.
type SourceAPIServer struct {
	store *SourceStore
}

// NewSourceAPIServer creates a new source API server.
func NewSourceAPIServer(store *SourceStore) *SourceAPIServer {
	return &SourceAPIServer{
		store: store,
	}
}

// SetupRouter configures the Gin router with all source API routes.
func (s *SourceAPIServer) SetupRouter() *gin.Engine {
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})

	api := router.Group("/api/v1/meta")
	api.GET("/sources", s.HandleListSources)
	api.GET("/sources/:id", s.HandleGetSource)
	api.POST("/sources", s.HandleCreateSource)
	api.PUT("/sources/:id", s.HandleUpdateSource)
	api.DELETE("/sources/:id", s.HandleDeleteSource)

	return router
}

// ListSourcesResponse represents the response for GET /api/v1/meta/sources.
type ListSourcesResponse struct {
	Sources []Source `json:"sources"`
	Total   int      `json:"total"`
}

// CreateSourceRequest represents the request for POST /api/v1/meta/sources.
type CreateSourceRequest struct {
	SourceType      string                 `json:"source_type" binding:"required"`
	URL             string                 `json:"url" binding:"required"`
	Name            string                 `json:"name" binding:"required"`
	PollingInterval *string                `json:"polling_interval,omitempty"`
	ScraperConfig   *scraper.ScraperConfig `json:"scraper_config,omitempty"`
	Enabled         *bool                  `json:"enabled,omitempty"` // Default: true
}

// UpdateSourceRequest represents the request for PUT
// /api/v1/meta/sources/{id}.
type UpdateSourceRequest struct {
	Name            *string                `json:"name,omitempty"`
	URL             *string                `json:"url,omitempty"`
	Enabled         *bool                  `json:"enabled,omitempty"`
	PollingInterval *string                `json:"polling_interval,omitempty"`
	ScraperConfig   *scraper.ScraperConfig `json:"scraper_config,omitempty"`
}

// errorResponse creates a standardized error response.
func errorResponse(code, message string) gin.H {
	return gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	}
}

// handleError maps domain errors to HTTP responses.
func (s *SourceAPIServer) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrSourceNotFound):
		c.JSON(http.StatusNotFound, errorResponse("not_found", err.Error()))
	case errors.Is(err, ErrDuplicateURL):
		c.JSON(http.StatusConflict, errorResponse("conflict", err.Error()))
	case errors.Is(err, ErrInvalidSourceType):
		c.JSON(http.StatusBadRequest, errorResponse("validation_error", err.Error()))
	default:
		c.JSON(http.StatusInternalServerError, errorResponse("internal_error", "Failed to process request"))
	}
}

// HandleListSources handles GET /api/v1/meta/sources.
func (s *SourceAPIServer) HandleListSources(c *gin.Context) {
	// Build filter from query parameters
	filter := SourceFilter{}

	if typeParam := c.Query("type"); typeParam != "" {
		filter.Type = &typeParam
	}

	if enabledParam := c.Query("enabled"); enabledParam != "" {
		enabled := enabledParam == "true"
		filter.Enabled = &enabled
	}

	sources, err := s.store.ListSources(filter)
	if err != nil {
		s.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, ListSourcesResponse{
		Sources: sources,
		Total:   len(sources),
	})
}

// HandleGetSource handles GET /api/v1/meta/sources/{id}.
func (s *SourceAPIServer) HandleGetSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", "Invalid source ID"))
		return
	}

	source, err := s.store.GetSource(sourceID)
	if err != nil {
		s.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, source)
}

// HandleCreateSource handles POST /api/v1/meta/sources.
func (s *SourceAPIServer) HandleCreateSource(c *gin.Context) {
	var req CreateSourceRequest

	// Bind JSON -- Gin validates required fields automatically
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("validation_error", err.Error()))
		return
	}

	// Validate source type (will be checked again in CreateSource, but fail
	// fast)
	if err := validateSourceType(req.SourceType); err != nil {
		s.handleError(c, err)
		return
	}

	// Validate scraper_config for website sources
	if req.SourceType == "website" && req.ScraperConfig == nil {
		c.JSON(http.StatusBadRequest, errorResponse("validation_error", "scraper_config is required for website sources"))
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if err := validatePollingInterval(*req.PollingInterval); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("validation_error", err.Error()))
			return
		}
	}

	// Determine enabled_at value from boolean enabled field
	var enabledAt *time.Time
	if req.Enabled == nil || *req.Enabled {
		// Default to enabled or explicitly enabled
		now := time.Now()
		enabledAt = &now
	}
	// If explicitly disabled, enabledAt stays nil

	// Create source
	source, err := s.store.CreateSource(req.SourceType, req.URL, req.Name, req.ScraperConfig, enabledAt)
	if err != nil {
		s.handleError(c, err)
		return
	}

	// Set polling_interval if provided
	if req.PollingInterval != nil {
		update := SourceUpdate{
			PollingInterval: req.PollingInterval,
		}
		if err := s.store.UpdateSource(source.SourceID, update); err != nil {
			s.handleError(c, err)
			return
		}
		source.PollingInterval = req.PollingInterval
	}

	c.JSON(http.StatusCreated, source)
}

// HandleUpdateSource handles PUT /api/v1/meta/sources/{id}.
func (s *SourceAPIServer) HandleUpdateSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", "Invalid source ID"))
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", err.Error()))
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if err := validatePollingInterval(*req.PollingInterval); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("validation_error", err.Error()))
			return
		}
	}

	// Build update struct
	update := SourceUpdate{
		Name:            req.Name,
		URL:             req.URL,
		PollingInterval: req.PollingInterval,
		ScraperConfig:   req.ScraperConfig,
	}

	// Handle enabled -- convert boolean to enabled_at timestamp
	if req.Enabled != nil {
		if *req.Enabled {
			now := time.Now()
			update.EnabledAt = &now
		} else {
			// Clear enabled_at to disable the source
			update.ClearEnabledAt = true
		}
	}

	// Update source
	if err := s.store.UpdateSource(sourceID, update); err != nil {
		s.handleError(c, err)
		return
	}

	// Return updated source
	source, err := s.store.GetSource(sourceID)
	if err != nil {
		s.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, source)
}

// HandleDeleteSource handles DELETE /api/v1/meta/sources/{id}.
func (s *SourceAPIServer) HandleDeleteSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("bad_request", "Invalid source ID"))
		return
	}

	if err := s.store.DeleteSource(sourceID); err != nil {
		s.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// validateSourceType validates that the source type is valid.
func validateSourceType(sourceType string) error {
	if sourceType != "rss" && sourceType != "atom" && sourceType != "website" {
		return ErrInvalidSourceType
	}
	return nil
}

// validatePollingInterval validates that a polling interval is a valid
// duration.
func validatePollingInterval(interval string) error {
	if _, err := time.ParseDuration(interval); err != nil {
		return errors.New("invalid polling_interval: must be a valid duration (e.g., 1h, 30m)")
	}
	return nil
}
