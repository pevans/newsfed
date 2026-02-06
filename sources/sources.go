package sources

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pevans/newsfed/scraper"
)

// Custom errors for source operations
var (
	ErrSourceNotFound    = errors.New("source not found")
	ErrDuplicateURL      = errors.New("source with this URL already exists")
	ErrInvalidSourceType = errors.New("source_type must be rss, atom, or website")
)

// SourceStore manages source configurations using SQLite.
type SourceStore struct {
	db *sql.DB
}

// Source represents a news source configuration.
type Source struct {
	SourceID        uuid.UUID              `json:"source_id"`
	SourceType      string                 `json:"source_type"` // "rss", "atom", "website"
	URL             string                 `json:"url"`
	Name            string                 `json:"name"`
	EnabledAt       *time.Time             `json:"enabled_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	PollingInterval *string                `json:"polling_interval,omitempty"`
	LastFetchedAt   *time.Time             `json:"last_fetched_at,omitempty"`
	LastModified    *string                `json:"last_modified,omitempty"`
	ETag            *string                `json:"etag,omitempty"`
	FetchErrorCount int                    `json:"fetch_error_count"`
	LastError       *string                `json:"last_error,omitempty"`
	ScraperConfig   *scraper.ScraperConfig `json:"scraper_config,omitempty"`
}

// IsEnabled returns true if the source is currently enabled.
func (s *Source) IsEnabled() bool {
	return s.EnabledAt != nil
}

// SourceUpdate represents fields that can be updated on a source.
type SourceUpdate struct {
	Name            *string
	URL             *string
	EnabledAt       *time.Time
	ClearEnabledAt  bool // Set to true to set enabled_at to NULL
	PollingInterval *string
	ScraperConfig   *scraper.ScraperConfig
	LastFetchedAt   *time.Time
	LastModified    *string
	ETag            *string
	FetchErrorCount *int
	LastError       *string
}

// SourceFilter represents filtering options for listing sources.
type SourceFilter struct {
	Type    *string // Filter by source_type
	Enabled *bool   // Filter by enabled status
	Limit   int     // Pagination limit
	Offset  int     // Pagination offset
}

// NewSourceStore creates a new source store with the given database path.
func NewSourceStore(dbPath string) (*SourceStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SourceStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the sources table if it doesn't exist.
func (s *SourceStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sources (
		source_id TEXT PRIMARY KEY,
		source_type TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		enabled_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		polling_interval TEXT,
		last_fetched_at TEXT,
		last_modified TEXT,
		etag TEXT,
		fetch_error_count INTEGER DEFAULT 0,
		last_error TEXT,
		scraper_config TEXT
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *SourceStore) Close() error {
	return s.db.Close()
}

// CreateSource creates a new source.
func (s *SourceStore) CreateSource(
	sourceType, url, name string,
	config *scraper.ScraperConfig,
	enabledAt *time.Time,
) (*Source, error) {
	// Validate source type
	if sourceType != "rss" && sourceType != "atom" && sourceType != "website" {
		return nil, ErrInvalidSourceType
	}

	now := time.Now()

	source := &Source{
		SourceID:        uuid.New(),
		SourceType:      sourceType,
		URL:             url,
		Name:            name,
		EnabledAt:       enabledAt,
		CreatedAt:       now,
		UpdatedAt:       now,
		FetchErrorCount: 0,
		ScraperConfig:   config,
	}

	// Serialize scraper_config to JSON if present
	var scraperConfigJSON *string
	if config != nil {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal scraper_config: %w", err)
		}
		jsonStr := string(data)
		scraperConfigJSON = &jsonStr
	}

	query := `
		INSERT INTO sources (
			source_id, source_type, url, name, enabled_at,
			created_at, updated_at, scraper_config
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		source.SourceID.String(),
		source.SourceType,
		source.URL,
		source.Name,
		formatTime(source.EnabledAt),
		formatTime(&source.CreatedAt),
		formatTime(&source.UpdatedAt),
		scraperConfigJSON,
	)
	if err != nil {
		// Check for duplicate URL constraint violation
		if strings.Contains(err.Error(), "UNIQUE constraint") ||
			strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrDuplicateURL
		}
		return nil, fmt.Errorf("failed to insert source: %w", err)
	}

	return source, nil
}

// GetSource retrieves a source by ID.
func (s *SourceStore) GetSource(sourceID uuid.UUID) (*Source, error) {
	query := `
		SELECT source_id, source_type, url, name, enabled_at,
		       created_at, updated_at, polling_interval, last_fetched_at,
		       last_modified, etag, fetch_error_count, last_error, scraper_config
		FROM sources
		WHERE source_id = ?
	`

	var sourceIDStr, sourceType, url, name, createdAtStr, updatedAtStr string
	var enabledAtStr, pollingInterval, lastFetchedAtStr, lastModified, etag, lastError, scraperConfigJSON sql.NullString
	var fetchErrorCount int

	err := s.db.QueryRow(query, sourceID.String()).Scan(
		&sourceIDStr, &sourceType, &url, &name,
		&enabledAtStr, &createdAtStr, &updatedAtStr,
		&pollingInterval, &lastFetchedAtStr, &lastModified,
		&etag, &fetchErrorCount, &lastError, &scraperConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query source: %w", err)
	}

	return scanSource(
		sourceIDStr, sourceType, url, name,
		createdAtStr, updatedAtStr,
		enabledAtStr, pollingInterval, lastFetchedAtStr,
		lastModified, etag, fetchErrorCount, lastError, scraperConfigJSON,
	)
}

// ListSources lists sources with optional filtering.
func (s *SourceStore) ListSources(filter SourceFilter) ([]Source, error) {
	// Build query with WHERE clause based on filter
	query := `
		SELECT source_id, source_type, url, name, enabled_at,
		       created_at, updated_at, polling_interval, last_fetched_at,
		       last_modified, etag, fetch_error_count, last_error, scraper_config
		FROM sources
	`

	var whereClauses []string
	var args []any

	if filter.Type != nil {
		whereClauses = append(whereClauses, "source_type = ?")
		args = append(args, *filter.Type)
	}

	if filter.Enabled != nil {
		if *filter.Enabled {
			whereClauses = append(whereClauses, "enabled_at IS NOT NULL")
		} else {
			whereClauses = append(whereClauses, "enabled_at IS NULL")
		}
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var sourceIDStr, sourceType, url, name, createdAtStr, updatedAtStr string
		var enabledAtStr, pollingInterval, lastFetchedAtStr, lastModified, etag, lastError, scraperConfigJSON sql.NullString
		var fetchErrorCount int

		err := rows.Scan(
			&sourceIDStr, &sourceType, &url, &name,
			&enabledAtStr, &createdAtStr, &updatedAtStr,
			&pollingInterval, &lastFetchedAtStr, &lastModified,
			&etag, &fetchErrorCount, &lastError, &scraperConfigJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}

		source, err := scanSource(
			sourceIDStr, sourceType, url, name,
			createdAtStr, updatedAtStr,
			enabledAtStr, pollingInterval, lastFetchedAtStr,
			lastModified, etag, fetchErrorCount, lastError, scraperConfigJSON,
		)
		if err != nil {
			return nil, err
		}

		sources = append(sources, *source)
	}

	return sources, nil
}

// UpdateSource updates a source with the provided fields.
func (s *SourceStore) UpdateSource(sourceID uuid.UUID, update SourceUpdate) error {
	// Build dynamic UPDATE query based on provided fields
	setClauses := []string{"updated_at = ?"}
	now := time.Now()
	args := []any{formatTime(&now)}

	if update.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *update.Name)
	}
	if update.URL != nil {
		setClauses = append(setClauses, "url = ?")
		args = append(args, *update.URL)
	}
	if update.ClearEnabledAt {
		setClauses = append(setClauses, "enabled_at = ?")
		args = append(args, nil)
	} else if update.EnabledAt != nil {
		setClauses = append(setClauses, "enabled_at = ?")
		args = append(args, formatTime(update.EnabledAt))
	}
	if update.PollingInterval != nil {
		setClauses = append(setClauses, "polling_interval = ?")
		args = append(args, *update.PollingInterval)
	}
	if update.ScraperConfig != nil {
		data, err := json.Marshal(update.ScraperConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal scraper_config: %w", err)
		}
		setClauses = append(setClauses, "scraper_config = ?")
		args = append(args, string(data))
	}
	if update.LastFetchedAt != nil {
		setClauses = append(setClauses, "last_fetched_at = ?")
		args = append(args, formatTime(update.LastFetchedAt))
	}
	if update.LastModified != nil {
		setClauses = append(setClauses, "last_modified = ?")
		args = append(args, *update.LastModified)
	}
	if update.ETag != nil {
		setClauses = append(setClauses, "etag = ?")
		args = append(args, *update.ETag)
	}
	if update.FetchErrorCount != nil {
		setClauses = append(setClauses, "fetch_error_count = ?")
		args = append(args, *update.FetchErrorCount)
	}
	if update.LastError != nil {
		setClauses = append(setClauses, "last_error = ?")
		args = append(args, *update.LastError)
	}

	// Add WHERE clause
	args = append(args, sourceID.String())

	query := fmt.Sprintf("UPDATE sources SET %s WHERE source_id = ?",
		strings.Join(setClauses, ", "))

	result, err := s.db.Exec(query, args...)
	if err != nil {
		// Check for duplicate URL constraint violation
		if strings.Contains(err.Error(), "UNIQUE constraint") ||
			strings.Contains(err.Error(), "unique constraint") {
			return ErrDuplicateURL
		}
		return fmt.Errorf("failed to update source: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrSourceNotFound
	}

	return nil
}

// DeleteSource deletes a source.
func (s *SourceStore) DeleteSource(sourceID uuid.UUID) error {
	result, err := s.db.Exec("DELETE FROM sources WHERE source_id = ?", sourceID.String())
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrSourceNotFound
	}

	return nil
}

// scanSource is a shared helper that parses SQL row data into a Source
// struct. This eliminates duplication between GetSource and ListSources.
func scanSource(
	sourceIDStr, sourceType, url, name string,
	createdAtStr, updatedAtStr string,
	enabledAtStr, pollingInterval, lastFetchedAtStr, lastModified, etag sql.NullString,
	fetchErrorCount int,
	lastError, scraperConfigJSON sql.NullString,
) (*Source, error) {
	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source ID: %w", err)
	}

	source := &Source{
		SourceID:        sourceID,
		SourceType:      sourceType,
		URL:             url,
		Name:            name,
		CreatedAt:       parseTime(createdAtStr),
		UpdatedAt:       parseTime(updatedAtStr),
		FetchErrorCount: fetchErrorCount,
	}

	// Parse optional timestamps
	if enabledAtStr.Valid {
		t := parseTime(enabledAtStr.String)
		source.EnabledAt = &t
	}
	if lastFetchedAtStr.Valid {
		t := parseTime(lastFetchedAtStr.String)
		source.LastFetchedAt = &t
	}

	// Parse optional strings
	if pollingInterval.Valid {
		source.PollingInterval = &pollingInterval.String
	}
	if lastModified.Valid {
		source.LastModified = &lastModified.String
	}
	if etag.Valid {
		source.ETag = &etag.String
	}
	if lastError.Valid {
		source.LastError = &lastError.String
	}

	// Parse scraper_config JSON
	if scraperConfigJSON.Valid {
		var config scraper.ScraperConfig
		if err := json.Unmarshal([]byte(scraperConfigJSON.String), &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal scraper_config: %w", err)
		}
		source.ScraperConfig = &config
	}

	return source, nil
}

// Helper functions for time formatting
func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	// Strip monotonic clock for consistent storage and comparisons
	return t.Truncate(0).Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	// Try RFC3339Nano first, fall back to RFC3339 for compatibility
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	// Strip monotonic clock for consistent comparisons
	return t.Truncate(0)
}
