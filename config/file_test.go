package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFilePath(t *testing.T) {
	path, err := ConfigFilePath()
	require.NoError(t, err)
	assert.Contains(t, path, ".newsfed")
	assert.True(t, filepath.IsAbs(path), "config file path should be absolute")
	assert.True(t, strings.HasSuffix(path, filepath.Join(".newsfed", "config.yaml")))
}

func TestWriteDefaultConfigFile_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	created, err := WriteDefaultConfigFile(false)
	require.NoError(t, err)
	assert.True(t, created)

	// Verify file exists with correct permissions
	configPath := filepath.Join(tmpDir, ".newsfed", "config.yaml")
	stat, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())

	// Verify content is valid YAML that parses into our config struct
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "metadata")
	assert.Contains(t, content, "feed")
	assert.Contains(t, content, filepath.Join(tmpDir, ".newsfed", "metadata.db"))
	assert.Contains(t, content, filepath.Join(tmpDir, ".newsfed", "feed"))

	// Verify the YAML parses correctly
	cfg, err := LoadConfigFile()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "sqlite", cfg.Storage.Metadata.Type)
	assert.Equal(t, filepath.Join(tmpDir, ".newsfed", "metadata.db"), cfg.Storage.Metadata.DSN)
	assert.Equal(t, "file", cfg.Storage.Feed.Type)
	assert.Equal(t, filepath.Join(tmpDir, ".newsfed", "feed"), cfg.Storage.Feed.DSN)
}

func TestWriteDefaultConfigFile_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create config file manually
	configDir := filepath.Join(tmpDir, ".newsfed")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	originalContent := []byte("original content\n")
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), originalContent, 0o600))

	created, err := WriteDefaultConfigFile(false)
	require.NoError(t, err)
	assert.False(t, created, "should not overwrite existing file")

	// Verify original content is preserved
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, originalContent, data)
}

func TestWriteDefaultConfigFile_ForceOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create config file manually
	configDir := filepath.Join(tmpDir, ".newsfed")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("old"), 0o600))

	created, err := WriteDefaultConfigFile(true)
	require.NoError(t, err)
	assert.True(t, created, "force should overwrite existing file")

	// Verify content was overwritten with default config
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "metadata")
}

func TestWriteDefaultConfigFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Ensure .newsfed does not exist
	configDir := filepath.Join(tmpDir, ".newsfed")
	_, err := os.Stat(configDir)
	require.True(t, os.IsNotExist(err))

	created, err := WriteDefaultConfigFile(false)
	require.NoError(t, err)
	assert.True(t, created)

	// Verify directory was created with 0700
	stat, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
	assert.Equal(t, os.FileMode(0o700), stat.Mode().Perm())
}

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
