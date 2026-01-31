package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// NewsFeed represents a collection of news items stored in a directory
type NewsFeed struct {
	storageDir string
}

// New creates a new news feed with the specified storage directory
func New(storageDir string) (*NewsFeed, error) {
	// Create the storage directory if it doesn't exist
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &NewsFeed{
		storageDir: storageDir,
	}, nil
}

// Add saves a news item to the feed
func (nf *NewsFeed) Add(item NewsItem) error {
	// Use the item's UUID as the filename
	filename := filepath.Join(nf.storageDir, item.ID.String()+".json")

	// Marshal the item to JSON
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal news item: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("failed to write news item: %w", err)
	}

	return nil
}

// List returns all news items in the feed. Corrupted or invalid files are
// logged and skipped rather than causing the entire operation to fail.
func (nf *NewsFeed) List() ([]NewsItem, error) {
	entries, err := os.ReadDir(nf.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var items []NewsItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Read the file
		filename := filepath.Join(nf.storageDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read file %s: %v\n", entry.Name(), err)
			continue
		}

		// Unmarshal the news item
		var item NewsItem
		if err := json.Unmarshal(data, &item); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal file %s: %v\n", entry.Name(), err)
			continue
		}

		items = append(items, item)
	}

	return items, nil
}
