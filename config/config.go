package config

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// ConfigStore manages user configuration using SQLite.
type ConfigStore struct {
	db *sql.DB
}

// Config represents user configuration.
type Config struct {
	DefaultPollingInterval string `json:"default_polling_interval"`
	BrowserCommand         string `json:"browser_command"`
}

// NewConfigStore creates a new config store with the given database path.
func NewConfigStore(dbPath string) (*ConfigStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &ConfigStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the config table if it doesn't exist.
func (c *ConfigStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := c.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (c *ConfigStore) Close() error {
	return c.db.Close()
}

// GetConfig retrieves user configuration.
func (c *ConfigStore) GetConfig() (*Config, error) {
	query := "SELECT value FROM config WHERE key = ?"

	var defaultPollingInterval string
	err := c.db.QueryRow(query, "default_polling_interval").Scan(&defaultPollingInterval)
	if err == sql.ErrNoRows {
		defaultPollingInterval = "1h"
	} else if err != nil {
		return nil, fmt.Errorf("failed to query default_polling_interval: %w", err)
	}

	var browserCommand string
	err = c.db.QueryRow(query, "browser_command").Scan(&browserCommand)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query browser_command: %w", err)
	}
	// If not found, browserCommand will be empty string (default)

	return &Config{
		DefaultPollingInterval: defaultPollingInterval,
		BrowserCommand:         browserCommand,
	}, nil
}

// UpdateConfig updates user configuration.
func (c *ConfigStore) UpdateConfig(cfg *Config) error {
	query := "INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)"

	_, err := c.db.Exec(query, "default_polling_interval", cfg.DefaultPollingInterval)
	if err != nil {
		return fmt.Errorf("failed to update default_polling_interval: %w", err)
	}

	if cfg.BrowserCommand != "" {
		_, err = c.db.Exec(query, "browser_command", cfg.BrowserCommand)
		if err != nil {
			return fmt.Errorf("failed to update browser_command: %w", err)
		}
	}

	return nil
}
