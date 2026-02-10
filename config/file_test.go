package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFile_NoFile(t *testing.T) {
	// Create a temporary directory that definitely doesn't have a config file
	tmpDir := t.TempDir()

	// Temporarily change HOME to point to tmpDir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := LoadConfigFile()
	require.NoError(t, err)
	assert.Nil(t, cfg, "Should return nil when config file doesn't exist")
}

func TestLoadConfigFile_ValidConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create .newsfed directory
	newsfedDir := filepath.Join(tmpDir, ".newsfed")
	require.NoError(t, os.MkdirAll(newsfedDir, 0o700))

	// Write a valid config file
	configPath := filepath.Join(newsfedDir, "config.yaml")
	configContent := `storage:
  metadata:
    type: "sqlite"
    dsn: "/path/to/metadata.db"
  feed:
    type: "file"
    dsn: "/path/to/feed"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	// Temporarily change HOME to point to tmpDir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := LoadConfigFile()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "sqlite", cfg.Storage.Metadata.Type)
	assert.Equal(t, "/path/to/metadata.db", cfg.Storage.Metadata.DSN)
	assert.Equal(t, "file", cfg.Storage.Feed.Type)
	assert.Equal(t, "/path/to/feed", cfg.Storage.Feed.DSN)
}

func TestLoadConfigFile_InvalidYAML(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create .newsfed directory
	newsfedDir := filepath.Join(tmpDir, ".newsfed")
	require.NoError(t, os.MkdirAll(newsfedDir, 0o700))

	// Write an invalid config file
	configPath := filepath.Join(newsfedDir, "config.yaml")
	invalidContent := `storage:
  metadata:
    type: "sqlite"
    dsn: "/path/to/metadata.db"
  feed:
    - this is invalid yaml because feed should be an object not a list
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidContent), 0o600))

	// Temporarily change HOME to point to tmpDir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := LoadConfigFile()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestLoadConfigFile_PartialConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create .newsfed directory
	newsfedDir := filepath.Join(tmpDir, ".newsfed")
	require.NoError(t, os.MkdirAll(newsfedDir, 0o700))

	// Write a partial config file (only metadata, no feed)
	configPath := filepath.Join(newsfedDir, "config.yaml")
	configContent := `storage:
  metadata:
    type: "postgres"
    dsn: "postgres://localhost/db"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	// Temporarily change HOME to point to tmpDir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := LoadConfigFile()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres", cfg.Storage.Metadata.Type)
	assert.Equal(t, "postgres://localhost/db", cfg.Storage.Metadata.DSN)
	assert.Equal(t, "", cfg.Storage.Feed.Type, "Unspecified feed type should be empty string")
	assert.Equal(t, "", cfg.Storage.Feed.DSN, "Unspecified feed DSN should be empty string")
}
