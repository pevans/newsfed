package newsfed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// MetadataStore manages source configurations and user preferences using
// SQLite. Implements RFC 5.
type MetadataStore struct {
	db *sql.DB
}

// Source represents a news source configuration. Implements RFC 5 section
// 2.1.
type Source struct {
	SourceID        uuid.UUID      `json:"source_id"`
	SourceType      string         `json:"source_type"` // "rss", "atom", "website"
	URL             string         `json:"url"`
	Name            string         `json:"name"`
	EnabledAt       *time.Time     `json:"enabled_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	PollingInterval *string        `json:"polling_interval,omitempty"`
	LastFetchedAt   *time.Time     `json:"last_fetched_at,omitempty"`
	LastModified    *string        `json:"last_modified,omitempty"`
	ETag            *string        `json:"etag,omitempty"`
	FetchErrorCount int            `json:"fetch_error_count"`
	LastError       *string        `json:"last_error,omitempty"`
	ScraperConfig   *ScraperConfig `json:"scraper_config,omitempty"`
}

// Config represents user configuration. Implements RFC 5 section 2.4.
type Config struct {
	DefaultPollingInterval string `json:"default_polling_interval"`
}

// NewMetadataStore creates a new metadata store with the given database path.
func NewMetadataStore(dbPath string) (*MetadataStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &MetadataStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables if they don't exist. Implements RFC
// 5 section 3.1.1.
func (m *MetadataStore) initSchema() error {
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

	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := m.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (m *MetadataStore) Close() error {
	return m.db.Close()
}

// CreateSource creates a new source. Implements RFC 5 section 4.1.1.
func (m *MetadataStore) CreateSource(
	sourceType, url, name string,
	config *ScraperConfig,
	enabledAt *time.Time,
) (*Source, error) {
	now := time.Now()

	source := &Source{
		SourceID:        uuid.New(),
		SourceType:      sourceType,
		URL:             url,
		Name:            name,
		EnabledAt:       enabledAt, // Use as-is (nil means disabled, &time means enabled at that time)
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

	_, err := m.db.Exec(query,
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
		return nil, fmt.Errorf("failed to insert source: %w", err)
	}

	return source, nil
}

// GetSource retrieves a source by ID. Implements RFC 5 section 4.1.2.
func (m *MetadataStore) GetSource(sourceID uuid.UUID) (*Source, error) {
	query := `
		SELECT source_id, source_type, url, name, enabled_at,
		       created_at, updated_at, polling_interval, last_fetched_at,
		       last_modified, etag, fetch_error_count, last_error, scraper_config
		FROM sources
		WHERE source_id = ?
	`

	var source Source
	var sourceIDStr, createdAtStr, updatedAtStr string
	var enabledAtStr, pollingInterval, lastFetchedAtStr, lastModified, etag, lastError, scraperConfigJSON sql.NullString

	err := m.db.QueryRow(query, sourceID.String()).Scan(
		&sourceIDStr, &source.SourceType, &source.URL, &source.Name,
		&enabledAtStr, &createdAtStr, &updatedAtStr,
		&pollingInterval, &lastFetchedAtStr, &lastModified,
		&etag, &source.FetchErrorCount, &lastError, &scraperConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("source not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query source: %w", err)
	}

	// Parse UUIDs and timestamps
	source.SourceID = sourceID
	source.CreatedAt = parseTime(createdAtStr)
	source.UpdatedAt = parseTime(updatedAtStr)
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
		var config ScraperConfig
		if err := json.Unmarshal([]byte(scraperConfigJSON.String), &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal scraper_config: %w", err)
		}
		source.ScraperConfig = &config
	}

	return &source, nil
}

// ListSources lists all sources. Implements RFC 5 section 4.1.2.
func (m *MetadataStore) ListSources() ([]Source, error) {
	query := `
		SELECT source_id, source_type, url, name, enabled_at,
		       created_at, updated_at, polling_interval, last_fetched_at,
		       last_modified, etag, fetch_error_count, last_error, scraper_config
		FROM sources
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var source Source
		var sourceIDStr, createdAtStr, updatedAtStr string
		var enabledAtStr, pollingInterval, lastFetchedAtStr, lastModified, etag, lastError, scraperConfigJSON sql.NullString

		err := rows.Scan(
			&sourceIDStr, &source.SourceType, &source.URL, &source.Name,
			&enabledAtStr, &createdAtStr, &updatedAtStr,
			&pollingInterval, &lastFetchedAtStr, &lastModified,
			&etag, &source.FetchErrorCount, &lastError, &scraperConfigJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}

		// Parse UUID and timestamps
		source.SourceID, _ = uuid.Parse(sourceIDStr)
		source.CreatedAt = parseTime(createdAtStr)
		source.UpdatedAt = parseTime(updatedAtStr)
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
			var config ScraperConfig
			if err := json.Unmarshal([]byte(scraperConfigJSON.String), &config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal scraper_config: %w", err)
			}
			source.ScraperConfig = &config
		}

		sources = append(sources, source)
	}

	return sources, nil
}

// UpdateSource updates a source. Implements RFC 5 section 4.1.3.
func (m *MetadataStore) UpdateSource(sourceID uuid.UUID, updates map[string]any) error {
	// Build dynamic UPDATE query based on provided fields
	setClauses := []string{"updated_at = ?"}
	args := []any{formatTime(&time.Time{})}

	if name, ok := updates["name"].(string); ok {
		setClauses = append(setClauses, "name = ?")
		args = append(args, name)
	}
	if url, ok := updates["url"].(string); ok {
		setClauses = append(setClauses, "url = ?")
		args = append(args, url)
	}
	if enabledAt, ok := updates["enabled_at"].(*time.Time); ok {
		setClauses = append(setClauses, "enabled_at = ?")
		args = append(args, formatTime(enabledAt))
	}
	if pollingInterval, ok := updates["polling_interval"].(string); ok {
		setClauses = append(setClauses, "polling_interval = ?")
		args = append(args, pollingInterval)
	}
	if scraperConfig, ok := updates["scraper_config"].(*ScraperConfig); ok {
		data, err := json.Marshal(scraperConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal scraper_config: %w", err)
		}
		setClauses = append(setClauses, "scraper_config = ?")
		args = append(args, string(data))
	}
	if lastFetchedAt, ok := updates["last_fetched_at"].(*time.Time); ok {
		setClauses = append(setClauses, "last_fetched_at = ?")
		args = append(args, formatTime(lastFetchedAt))
	}
	if lastModified, ok := updates["last_modified"].(*string); ok {
		setClauses = append(setClauses, "last_modified = ?")
		args = append(args, lastModified)
	}
	if etag, ok := updates["etag"].(*string); ok {
		setClauses = append(setClauses, "etag = ?")
		args = append(args, etag)
	}
	if fetchErrorCount, ok := updates["fetch_error_count"].(int); ok {
		setClauses = append(setClauses, "fetch_error_count = ?")
		args = append(args, fetchErrorCount)
	}
	if lastError, ok := updates["last_error"].(*string); ok {
		setClauses = append(setClauses, "last_error = ?")
		args = append(args, lastError)
	}

	// Set updated_at to now
	now := time.Now()
	args[0] = formatTime(&now)

	// Add WHERE clause
	args = append(args, sourceID.String())

	query := fmt.Sprintf("UPDATE sources SET %s WHERE source_id = ?",
		strings.Join(setClauses, ", "))

	result, err := m.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("source not found")
	}

	return nil
}

// DeleteSource deletes a source. Implements RFC 5 section 4.1.4.
func (m *MetadataStore) DeleteSource(sourceID uuid.UUID) error {
	result, err := m.db.Exec("DELETE FROM sources WHERE source_id = ?", sourceID.String())
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("source not found")
	}

	return nil
}

// GetConfig retrieves user configuration. Implements RFC 5 section 4.2.
func (m *MetadataStore) GetConfig() (*Config, error) {
	query := "SELECT value FROM config WHERE key = ?"

	var defaultPollingInterval string
	err := m.db.QueryRow(query, "default_polling_interval").Scan(&defaultPollingInterval)
	if err == sql.ErrNoRows {
		// Return default config if not set
		return &Config{DefaultPollingInterval: "1h"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %w", err)
	}

	return &Config{DefaultPollingInterval: defaultPollingInterval}, nil
}

// UpdateConfig updates user configuration. Implements RFC 5 section 4.2.
func (m *MetadataStore) UpdateConfig(config *Config) error {
	query := "INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)"
	_, err := m.db.Exec(query, "default_polling_interval", config.DefaultPollingInterval)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}
	return nil
}

// Helper functions for time formatting
func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
