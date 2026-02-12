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

// ConfigFilePath returns the path to the default config file
// (~/.newsfed/config.yaml).
func ConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".newsfed", "config.yaml"), nil
}

// WriteDefaultConfigFile creates a default config file at
// ~/.newsfed/config.yaml with absolute paths. If the file already exists and
// force is false, it is skipped (returns false, nil). If force is true, the
// file is overwritten. Returns true if a file was created or overwritten.
func WriteDefaultConfigFile(force bool) (bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".newsfed")
	configPath := filepath.Join(configDir, "config.yaml")

	// Skip if file exists and force is false
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return false, nil
		}
	}

	// Create directory if needed
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return false, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Build default config content with absolute paths
	metadataDSN := filepath.Join(homeDir, ".newsfed", "metadata.db")
	feedDSN := filepath.Join(homeDir, ".newsfed", "feed")

	content := fmt.Sprintf(`# Newsfed configuration
# See https://github.com/pevans/newsfed for details

storage:
  # Metadata storage (sources, configuration)
  metadata:
    type: "sqlite"
    dsn: %q

  # News feed storage (news items)
  feed:
    type: "file"
    dsn: %q
`, metadataDSN, feedDSN)

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return false, fmt.Errorf("failed to write config file: %w", err)
	}

	return true, nil
}

// LoadConfigFile loads configuration from ~/.newsfed/config.yaml. Returns nil
// if the file doesn't exist (not an error). Returns error if the file exists
// but cannot be parsed.
func LoadConfigFile() (*FileConfig, error) {
	configPath, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

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
