package main

import (
	"log"

	"github.com/pevans/newsfed"
)

func main() {
	// Create metadata store (RFC 5)
	store, err := newsfed.NewMetadataStore(".newsfed/metadata.db")
	if err != nil {
		log.Fatalf("Failed to create metadata store: %v", err)
	}
	defer store.Close()

	// Create API server (RFC 6)
	server := newsfed.NewMetadataAPIServer(store)

	// Setup Gin router with all routes
	router := server.SetupRouter()

	// Start server
	addr := "localhost:8081"
	log.Printf("Starting Metadata API server on http://%s/api/v1/meta", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
