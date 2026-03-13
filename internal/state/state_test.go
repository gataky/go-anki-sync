package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gataky/sync/pkg/models"
)

func TestGetDefaultStatePath(t *testing.T) {
	statePath, err := GetDefaultStatePath()
	require.NoError(t, err)
	assert.NotEmpty(t, statePath)
	assert.Contains(t, statePath, ".sync")
	assert.Contains(t, statePath, "state.json")
}

func TestLoadState_ValidState(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	// Create a valid state file
	validJSON := `{
  "last_pull_timestamp": "2024-02-03T10:30:00Z",
  "last_push_timestamp": "2024-02-03T10:35:00Z",
  "config_hash": "abc123def456"
}`

	err := os.WriteFile(statePath, []byte(validJSON), 0644)
	require.NoError(t, err)

	// Load the state
	state, err := LoadState(statePath)
	require.NoError(t, err)
	assert.NotNil(t, state)

	expectedPullTime, _ := time.Parse(time.RFC3339, "2024-02-03T10:30:00Z")
	expectedPushTime, _ := time.Parse(time.RFC3339, "2024-02-03T10:35:00Z")

	assert.Equal(t, expectedPullTime, state.LastPullTimestamp)
	assert.Equal(t, expectedPushTime, state.LastPushTimestamp)
	assert.Equal(t, "abc123def456", state.ConfigHash)
}

func TestLoadState_MissingFile(t *testing.T) {
	// LoadState should return empty state, not an error
	state, err := LoadState("/nonexistent/path/state.json")
	require.NoError(t, err, "Missing state file should not be an error")
	assert.NotNil(t, state)

	// Check that timestamps are zero values
	assert.True(t, state.LastPullTimestamp.IsZero())
	assert.True(t, state.LastPushTimestamp.IsZero())
	assert.Empty(t, state.ConfigHash)
}

func TestLoadState_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	// Create invalid JSON
	invalidJSON := `{
  "last_pull_timestamp": "invalid",
  "config_hash": [broken json
}`

	err := os.WriteFile(statePath, []byte(invalidJSON), 0644)
	require.NoError(t, err)

	state, err := LoadState(statePath)
	assert.Error(t, err)
	assert.Nil(t, state)
	assert.Contains(t, err.Error(), "failed to parse state file")
}

func TestSaveState(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	pullTime := time.Date(2024, 2, 3, 10, 30, 0, 0, time.UTC)
	pushTime := time.Date(2024, 2, 3, 10, 35, 0, 0, time.UTC)

	state := &models.SyncState{
		LastPullTimestamp: pullTime,
		LastPushTimestamp: pushTime,
		ConfigHash:        "test-hash-123",
	}

	err := SaveState(state, statePath)
	require.NoError(t, err)

	// Verify file was created
	info, err := os.Stat(statePath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())

	// Read file content
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	// Verify JSON is formatted
	assert.Contains(t, string(data), "last_pull_timestamp")
	assert.Contains(t, string(data), "last_push_timestamp")
	assert.Contains(t, string(data), "config_hash")
	assert.Contains(t, string(data), "test-hash-123")
}

func TestRoundTripStateSaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	pullTime := time.Date(2024, 2, 3, 10, 30, 0, 0, time.UTC)
	pushTime := time.Date(2024, 2, 3, 10, 35, 0, 0, time.UTC)

	original := &models.SyncState{
		LastPullTimestamp: pullTime,
		LastPushTimestamp: pushTime,
		ConfigHash:        "abc123def456789",
	}

	// Save
	err := SaveState(original, statePath)
	require.NoError(t, err)

	// Load
	loaded, err := LoadState(statePath)
	require.NoError(t, err)

	// Compare - timestamps might have nano precision differences, so compare to second
	assert.Equal(t, original.LastPullTimestamp.Unix(), loaded.LastPullTimestamp.Unix())
	assert.Equal(t, original.LastPushTimestamp.Unix(), loaded.LastPushTimestamp.Unix())
	assert.Equal(t, original.ConfigHash, loaded.ConfigHash)
}

func TestCalculateConfigHash_SameConfig(t *testing.T) {
	config := &models.Config{
		GoogleSheetID:   "test-sheet-id",
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	// Calculate hash twice
	hash1, err := CalculateConfigHash(config)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	hash2, err := CalculateConfigHash(config)
	require.NoError(t, err)
	assert.NotEmpty(t, hash2)

	// Hashes should be identical for same config
	assert.Equal(t, hash1, hash2)
}

func TestCalculateConfigHash_DifferentConfigs(t *testing.T) {
	config1 := &models.Config{
		GoogleSheetID:   "test-sheet-id-1",
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	config2 := &models.Config{
		GoogleSheetID:   "test-sheet-id-2", // Different
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	hash1, err := CalculateConfigHash(config1)
	require.NoError(t, err)

	hash2, err := CalculateConfigHash(config2)
	require.NoError(t, err)

	// Hashes should be different for different configs
	assert.NotEqual(t, hash1, hash2)
}

func TestCalculateConfigHash_MinorConfigChange(t *testing.T) {
	config1 := &models.Config{
		GoogleSheetID:   "test-sheet-id",
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	config2 := &models.Config{
		GoogleSheetID:   "test-sheet-id",
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "debug", // Changed log level
	}

	hash1, err := CalculateConfigHash(config1)
	require.NoError(t, err)

	hash2, err := CalculateConfigHash(config2)
	require.NoError(t, err)

	// Even minor changes should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestCalculateConfigHash_ConsistentOrdering(t *testing.T) {
	// Create two configs with same values but potentially different field ordering
	config1 := &models.Config{
		GoogleSheetID:   "test-id",
		SheetName:       "Sheet1",
		AnkiDeck:        "Deck1",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	config2 := &models.Config{
		LogLevel:        "info",
		AnkiDeck:        "Deck1",
		GoogleSheetID:   "test-id",
		SheetName:       "Sheet1",
		GoogleTokenPath: "~/.sync/token.json",
		AnkiConnectURL:  "http://localhost:8765",
	}

	hash1, err := CalculateConfigHash(config1)
	require.NoError(t, err)

	hash2, err := CalculateConfigHash(config2)
	require.NoError(t, err)

	// Hashes should be identical - YAML marshaling ensures consistent ordering
	assert.Equal(t, hash1, hash2, "Hash should be consistent regardless of struct field order")
}

func TestStateFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	state := &models.SyncState{
		LastPullTimestamp: time.Now(),
		LastPushTimestamp: time.Now(),
		ConfigHash:        "test",
	}

	err := SaveState(state, statePath)
	require.NoError(t, err)

	// Check file permissions
	info, err := os.Stat(statePath)
	require.NoError(t, err)

	// File should be readable by owner and group (0644)
	mode := info.Mode()
	assert.Equal(t, os.FileMode(0644), mode.Perm(), "State file should have 0644 permissions")
}
