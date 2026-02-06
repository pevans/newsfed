package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: create a test config store
func createTestConfigStore(t *testing.T) *ConfigStore {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewConfigStore(dbPath)
	require.NoError(t, err, "should create config store")
	t.Cleanup(func() { store.Close() })
	return store
}

// TestGetConfig_Default verifies default config is returned when not set
func TestGetConfig_Default(t *testing.T) {
	store := createTestConfigStore(t)

	config, err := store.GetConfig()
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, "1h", config.DefaultPollingInterval, "should have default polling interval")
}

// TestUpdateConfig_Success verifies updating config
func TestUpdateConfig_Success(t *testing.T) {
	store := createTestConfigStore(t)

	newConfig := &Config{
		DefaultPollingInterval: "30m",
	}

	err := store.UpdateConfig(newConfig)
	require.NoError(t, err)

	retrieved, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, "30m", retrieved.DefaultPollingInterval)
}

// TestUpdateConfig_Overwrites verifies updating config replaces old values
func TestUpdateConfig_Overwrites(t *testing.T) {
	store := createTestConfigStore(t)

	// Set initial value
	config1 := &Config{DefaultPollingInterval: "1h"}
	err := store.UpdateConfig(config1)
	require.NoError(t, err)

	// Overwrite with new value
	config2 := &Config{DefaultPollingInterval: "2h"}
	err = store.UpdateConfig(config2)
	require.NoError(t, err)

	// Verify new value
	retrieved, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, "2h", retrieved.DefaultPollingInterval)
}
