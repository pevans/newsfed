package main

import (
	"log"
)

func main() {
	// Create news feed with .news directory storage
	feed, err := New(".news")
	if err != nil {
		log.Fatalf("Failed to create news feed: %v", err)
	}

	// Create and start API server per RFC 4
	server := NewAPIServer(feed)
	addr := "localhost:8080"
	log.Printf("Starting newsfed API server on http://%s/api/v1", addr)

	if err := server.Start(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
