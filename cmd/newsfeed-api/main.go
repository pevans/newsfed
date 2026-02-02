package main

import (
	"log"
	"net/http"

	"github.com/pevans/newsfed"
)

func main() {
	// Create news feed with .news directory storage (RFC 1)
	feed, err := newsfed.New(".news")
	if err != nil {
		log.Fatalf("Failed to create news feed: %v", err)
	}

	// Create API server (RFC 4)
	server := newsfed.NewAPIServer(feed)

	// Create HTTP multiplexer and register routes
	mux := http.NewServeMux()

	// News Feed API routes - /api/v1/items
	mux.HandleFunc("/api/v1/items", server.RouteItems)
	mux.HandleFunc("/api/v1/items/", server.RouteItems)

	// Apply CORS middleware (RFC 4 section 6)
	handler := server.CORSMiddleware(mux)

	// Start server
	addr := "localhost:8080"
	log.Printf("Starting News Feed API server on http://%s/api/v1/items", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
