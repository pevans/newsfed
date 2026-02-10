package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// StorageConfig represents storage configuration from config file.
type StorageConfig struct {
	Metadata struct {
		Type string `yaml:"type"`
		DSN  string `yaml:"dsn"`
	} `yaml:"metadata"`
	Feed struct {
		Type string `yaml:"type"`
		DSN  string `yaml:"dsn"`
	} `yaml:"feed"`
}

// FileConfig represents the structure of ~/.newsfed/config.yaml.
type FileConfig struct {
	Storage StorageConfig `yaml:"storage"`
}

// LoadConfigFile loads configuration from ~/.newsfed/config.yaml. Returns nil
// if the file doesn't exist (not an error). Returns error if the file exists
// but cannot be parsed.
func LoadConfigFile() (*FileConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".newsfed", "config.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // File doesn't exist -- not an error
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
