package config

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ConfigAPIServer represents the HTTP API server for configuration
// management.
type ConfigAPIServer struct {
	store *ConfigStore
}

// NewConfigAPIServer creates a new config API server.
func NewConfigAPIServer(store *ConfigStore) *ConfigAPIServer {
	return &ConfigAPIServer{
		store: store,
	}
}

// SetupRouter configures the Gin router with config API routes.
func (c *ConfigAPIServer) SetupRouter() *gin.Engine {
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusOK)
			return
		}

		ctx.Next()
	})

	api := router.Group("/api/v1/meta")
	api.GET("/config", c.HandleGetConfig)
	api.PUT("/config", c.HandleUpdateConfig)

	return router
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

// HandleGetConfig handles GET /api/v1/meta/config.
func (c *ConfigAPIServer) HandleGetConfig(ctx *gin.Context) {
	config, err := c.store.GetConfig()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse("internal_error", "Failed to retrieve configuration"))
		return
	}

	ctx.JSON(http.StatusOK, config)
}

// HandleUpdateConfig handles PUT /api/v1/meta/config.
func (c *ConfigAPIServer) HandleUpdateConfig(ctx *gin.Context) {
	var updates Config
	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse("bad_request", err.Error()))
		return
	}

	// If no fields provided (empty body), return current config
	if updates.DefaultPollingInterval == "" {
		config, err := c.store.GetConfig()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse("internal_error", "Failed to retrieve configuration"))
			return
		}
		ctx.JSON(http.StatusOK, config)
		return
	}

	// Validate default_polling_interval
	if err := validatePollingInterval(updates.DefaultPollingInterval); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse("validation_error", err.Error()))
		return
	}

	// Update config
	if err := c.store.UpdateConfig(&updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse("internal_error", "Failed to update configuration"))
		return
	}

	ctx.JSON(http.StatusOK, updates)
}

// validatePollingInterval validates that a polling interval is a valid
// duration.
func validatePollingInterval(interval string) error {
	if _, err := time.ParseDuration(interval); err != nil {
		return errors.New("invalid default_polling_interval: must be a valid duration (e.g., 1h, 30m)")
	}
	return nil
}
