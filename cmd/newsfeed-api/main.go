package main

import (
	"log"

	"github.com/pevans/newsfed"
)

func main() {
	// Create news feed with .news directory storage (RFC 1)
	feed, err := newsfed.NewNewsFeed(".news")
	if err != nil {
		log.Fatalf("Failed to create news feed: %v", err)
	}

	// Create API server (RFC 4)
	server := newsfed.NewAPIServer(feed)

	// Setup Gin router with all routes
	router := server.SetupRouter()

	// Start server
	addr := "localhost:8080"
	log.Printf("Starting News Feed API server on http://%s/api/v1/items", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
