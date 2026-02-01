package main

import (
	"time"

	"github.com/google/uuid"
)

// NewsItem represents a single news item as defined in RFC 1, section 2.1
type NewsItem struct {
	ID           uuid.UUID  `json:"id"`
	Title        string     `json:"title"`
	Summary      string     `json:"summary"`
	URL          string     `json:"url"`
	Publisher    *string    `json:"publisher,omitempty"`
	Authors      []string   `json:"authors"`
	PublishedAt  time.Time  `json:"published_at"`
	DiscoveredAt time.Time  `json:"discovered_at"`
	PinnedAt     *time.Time `json:"pinned_at,omitempty"`
}
