package main

import (
	"log"
	"net/http"

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

	// Create HTTP multiplexer and register routes
	mux := http.NewServeMux()

	// Metadata Management API routes - /api/v1/meta/*
	mux.HandleFunc("/api/v1/meta/sources", server.RouteSources)
	mux.HandleFunc("/api/v1/meta/sources/", server.RouteSources)
	mux.HandleFunc("/api/v1/meta/config", server.RouteConfig)
	mux.HandleFunc("/api/v1/meta/config/", server.RouteConfig)

	// Apply CORS middleware (RFC 6 section 6)
	handler := newsfed.CORSMiddleware(mux)

	// Start server
	addr := "localhost:8081"
	log.Printf("Starting Metadata API server on http://%s/api/v1/meta", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
