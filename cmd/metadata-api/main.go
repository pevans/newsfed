package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/pevans/newsfed/config"
	"github.com/pevans/newsfed/sources"
)

func main() {
	dbPath := ".newsfed/metadata.db"

	// Create source store
	sourceStore, err := sources.NewSourceStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to create source store: %v", err)
	}
	defer sourceStore.Close()

	// Create config store
	configStore, err := config.NewConfigStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to create config store: %v", err)
	}
	defer configStore.Close()

	// Create router with CORS middleware
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	})

	// Mount source API routes
	sourceServer := sources.NewSourceAPIServer(sourceStore)
	sourceGroup := router.Group("/api/v1/meta")
	sourceGroup.GET("/sources", sourceServer.HandleListSources)
	sourceGroup.GET("/sources/:id", sourceServer.HandleGetSource)
	sourceGroup.POST("/sources", sourceServer.HandleCreateSource)
	sourceGroup.PUT("/sources/:id", sourceServer.HandleUpdateSource)
	sourceGroup.DELETE("/sources/:id", sourceServer.HandleDeleteSource)

	// Mount config API routes
	configServer := config.NewConfigAPIServer(configStore)
	configGroup := router.Group("/api/v1/meta")
	configGroup.GET("/config", configServer.HandleGetConfig)
	configGroup.PUT("/config", configServer.HandleUpdateConfig)

	// Start server
	addr := "localhost:8081"
	log.Printf("Starting Metadata API server on http://%s/api/v1/meta", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
