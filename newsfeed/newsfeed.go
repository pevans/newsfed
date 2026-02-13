package newsfeed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// NewsFeed represents a collection of news items stored in a directory
type NewsFeed struct {
	storageDir string
}

// ReadError describes a failure to read a single news item file.
type ReadError struct {
	Filename string
	Err      error
}

func (e *ReadError) Error() string {
	return fmt.Sprintf("%s: %v", e.Filename, e.Err)
}

// ListResult contains the results of listing news items, including
// any per-file errors that occurred during the operation.
type ListResult struct {
	Items  []NewsItem
	Errors []ReadError
}

// NewNewsFeed creates a new news feed with the specified storage directory
func NewNewsFeed(storageDir string) (*NewsFeed, error) {
	// Create the storage directory if it doesn't exist (0700: owner-only access)
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
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

	// Write to file (0600: owner-only read/write)
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return fmt.Errorf("failed to write news item: %w", err)
	}

	return nil
}

// List returns all news items in the feed. Corrupted or invalid files are
// collected in the result's Errors slice rather than causing the entire
// operation to fail. A non-nil error return indicates a total failure
// (e.g., the storage directory is unreadable).
func (nf *NewsFeed) List() (*ListResult, error) {
	entries, err := os.ReadDir(nf.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	result := &ListResult{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Read the file
		filename := filepath.Join(nf.storageDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			result.Errors = append(result.Errors, ReadError{
				Filename: entry.Name(),
				Err:      err,
			})
			continue
		}

		// Unmarshal the news item
		var item NewsItem
		if err := json.Unmarshal(data, &item); err != nil {
			result.Errors = append(result.Errors, ReadError{
				Filename: entry.Name(),
				Err:      err,
			})
			continue
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

// Get retrieves a news item by its ID.
func (nf *NewsFeed) Get(id uuid.UUID) (*NewsItem, error) {
	filename := filepath.Join(nf.storageDir, id.String()+".json")

	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Item not found (not an error)
		}
		return nil, fmt.Errorf("failed to read news item: %w", err)
	}

	// Unmarshal the news item
	var item NewsItem
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal news item: %w", err)
	}

	return &item, nil
}

// Delete removes a news item from the feed by its ID.
func (nf *NewsFeed) Delete(id uuid.UUID) error {
	filename := filepath.Join(nf.storageDir, id.String()+".json")
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("failed to delete news item: %w", err)
	}
	return nil
}

// Update updates an existing news item in the feed.
func (nf *NewsFeed) Update(item NewsItem) error {
	// Check if the item exists
	filename := filepath.Join(nf.storageDir, item.ID.String()+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("news item not found")
	}

	// Marshal the item to JSON
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal news item: %w", err)
	}

	// Write to file (0600: owner-only read/write)
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return fmt.Errorf("failed to write news item: %w", err)
	}

	return nil
}
