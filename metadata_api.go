package newsfed

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// SetupRouter configures the Gin router with all metadata API routes
func (s *MetadataAPIServer) SetupRouter() *gin.Engine {
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
	{
		api.GET("/sources", s.HandleListSources)
		api.GET("/sources/:id", s.HandleGetSource)
		api.POST("/sources", s.HandleCreateSource)
		api.PUT("/sources/:id", s.HandleUpdateSource)
		api.DELETE("/sources/:id", s.HandleDeleteSource)
		api.GET("/config", s.HandleGetConfig)
		api.PUT("/config", s.HandleUpdateConfig)
	}

	return router
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
	SourceType      string         `json:"source_type" binding:"required"`
	URL             string         `json:"url" binding:"required"`
	Name            string         `json:"name" binding:"required"`
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
func (s *MetadataAPIServer) HandleListSources(c *gin.Context) {
	sources, err := s.store.ListSources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to list sources",
			},
		})
		return
	}

	// Apply filters
	typeFilter := c.Query("type")
	enabledFilter := c.Query("enabled")

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

	c.JSON(http.StatusOK, ListSourcesResponse{
		Sources: filtered,
		Total:   len(filtered),
	})
}

// HandleGetSource handles GET /api/v1/meta/sources/{id}. Implements RFC 6
// section 3.2.
func (s *MetadataAPIServer) HandleGetSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "bad_request",
				"message": "Invalid source ID",
			},
		})
		return
	}

	source, err := s.store.GetSource(sourceID)
	if err != nil {
		if err.Error() == "source not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "not_found",
					"message": "Source with ID " + sourceID.String() + " not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to retrieve source",
			},
		})
		return
	}

	c.JSON(http.StatusOK, source)
}

// HandleCreateSource handles POST /api/v1/meta/sources. Implements RFC 6
// section 3.3.
func (s *MetadataAPIServer) HandleCreateSource(c *gin.Context) {
	var req CreateSourceRequest

	// Bind JSON - Gin validates required fields automatically
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "validation_error",
				"message": err.Error(),
			},
		})
		return
	}

	// Validate source_type
	if req.SourceType != "rss" && req.SourceType != "atom" && req.SourceType != "website" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "validation_error",
				"message": "source_type must be one of: rss, atom, website",
			},
		})
		return
	}

	// Validate scraper_config for website sources
	if req.SourceType == "website" && req.ScraperConfig == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "validation_error",
				"message": "scraper_config is required for website sources",
			},
		})
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if _, err := time.ParseDuration(*req.PollingInterval); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "validation_error",
					"message": "Invalid polling_interval: must be a valid duration (e.g., 1h, 30m)",
				},
			})
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
		// Check for duplicate URL
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "conflict",
					"message": "Source with this URL already exists",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to create source",
			},
		})
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

	c.JSON(http.StatusCreated, source)
}

// HandleUpdateSource handles PUT /api/v1/meta/sources/{id}. Implements RFC 6
// section 3.4.
func (s *MetadataAPIServer) HandleUpdateSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "bad_request",
				"message": "Invalid source ID",
			},
		})
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "bad_request",
				"message": err.Error(),
			},
		})
		return
	}

	// Validate polling_interval if provided
	if req.PollingInterval != nil {
		if _, err := time.ParseDuration(*req.PollingInterval); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "validation_error",
					"message": "Invalid polling_interval: must be a valid duration (e.g., 1h, 30m)",
				},
			})
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
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "not_found",
					"message": "Source with ID " + sourceID.String() + " not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to update source",
			},
		})
		return
	}

	// Return updated source
	source, err := s.store.GetSource(sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to retrieve updated source",
			},
		})
		return
	}

	c.JSON(http.StatusOK, source)
}

// HandleDeleteSource handles DELETE /api/v1/meta/sources/{id}. Implements RFC
// 6 section 3.5.
func (s *MetadataAPIServer) HandleDeleteSource(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "bad_request",
				"message": "Invalid source ID",
			},
		})
		return
	}

	err = s.store.DeleteSource(sourceID)
	if err != nil {
		if err.Error() == "source not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "not_found",
					"message": "Source with ID " + sourceID.String() + " not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to delete source",
			},
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleGetConfig handles GET /api/v1/meta/config. Implements RFC 6 section
// 3.6.
func (s *MetadataAPIServer) HandleGetConfig(c *gin.Context) {
	config, err := s.store.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to retrieve configuration",
			},
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleUpdateConfig handles PUT /api/v1/meta/config. Implements RFC 6
// section 3.7.
func (s *MetadataAPIServer) HandleUpdateConfig(c *gin.Context) {
	var updates Config
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "bad_request",
				"message": err.Error(),
			},
		})
		return
	}

	// If no fields provided (empty body), return current config
	if updates.DefaultPollingInterval == "" {
		config, err := s.store.GetConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "internal_error",
					"message": "Failed to retrieve configuration",
				},
			})
			return
		}
		c.JSON(http.StatusOK, config)
		return
	}

	// Validate default_polling_interval
	if _, err := time.ParseDuration(updates.DefaultPollingInterval); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "validation_error",
				"message": "Invalid default_polling_interval: must be a valid duration (e.g., 1h, 30m)",
			},
		})
		return
	}

	// Update config
	if err := s.store.UpdateConfig(&updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Failed to update configuration",
			},
		})
		return
	}

	c.JSON(http.StatusOK, updates)
}
