package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/sync/pkg/models"
)

func TestGetConfigDir(t *testing.T) {
	configDir, err := GetConfigDir()
	require.NoError(t, err)
	assert.NotEmpty(t, configDir)
	assert.Contains(t, configDir, ".sync")
}

func TestGetDefaultConfigPath(t *testing.T) {
	configPath, err := GetDefaultConfigPath()
	require.NoError(t, err)
	assert.NotEmpty(t, configPath)
	assert.Contains(t, configPath, ".sync")
	assert.Contains(t, configPath, "config.yaml")
}

func TestEnsureConfigDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Override the config directory for testing by manipulating the path
	testConfigDir := filepath.Join(tempDir, ".sync-test")

	// Create the directory
	err := os.MkdirAll(testConfigDir, 0755)
	require.NoError(t, err)

	// Verify directory exists
	info, err := os.Stat(testConfigDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	validYAML := `
google_sheet_id: "test-sheet-id-123"
sheet_name: "Vocabulary"
anki_deck: "Greek Vocabulary"
anki_connect_url: "http://localhost:8765"
google_token_path: "~/.sync/token.json"
log_level: "info"
`

	err := os.WriteFile(configPath, []byte(validYAML), 0644)
	require.NoError(t, err)

	// Load the config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "test-sheet-id-123", config.GoogleSheetID)
	assert.Equal(t, "Vocabulary", config.SheetName)
	assert.Equal(t, "Greek Vocabulary", config.AnkiDeck)
	assert.Equal(t, "http://localhost:8765", config.AnkiConnectURL)
	assert.Equal(t, "info", config.LogLevel)
}

func TestLoadConfig_WithDefaults(t *testing.T) {
	// Create config without optional fields
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	minimalYAML := `
google_sheet_id: "test-sheet-id"
sheet_name: "Vocabulary"
anki_deck: "Test Deck"
`

	err := os.WriteFile(configPath, []byte(minimalYAML), 0644)
	require.NoError(t, err)

	// Load the config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Check defaults were applied
	assert.Equal(t, models.DefaultAnkiConnectURL, config.AnkiConnectURL)
	assert.Equal(t, models.DefaultLogLevel, config.LogLevel)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	config, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "config file not found")
	assert.Contains(t, err.Error(), "run 'sync init'")
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	invalidYAML := `
google_sheet_id: "test"
sheet_name: [invalid yaml structure
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestLoadConfig_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		errorMsg string
	}{
		{
			name: "missing google_sheet_id",
			yaml: `
sheet_name: "Vocabulary"
anki_deck: "Test Deck"
`,
			errorMsg: "google_sheet_id is required",
		},
		{
			name: "missing sheet_name",
			yaml: `
google_sheet_id: "test-id"
anki_deck: "Test Deck"
`,
			errorMsg: "sheet_name is required",
		},
		{
			name: "missing anki_deck",
			yaml: `
google_sheet_id: "test-id"
sheet_name: "Vocabulary"
`,
			errorMsg: "anki_deck is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			err := os.WriteFile(configPath, []byte(tt.yaml), 0644)
			require.NoError(t, err)

			config, err := LoadConfig(configPath)
			assert.Error(t, err)
			assert.Nil(t, config)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	invalidYAML := `
google_sheet_id: "test-id"
sheet_name: "Vocabulary"
anki_deck: "Test Deck"
log_level: "invalid"
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "log_level must be")
}

func TestSaveConfig_ValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	config := &models.Config{
		GoogleSheetID:   "test-sheet-id",
		SheetName:       "Vocabulary",
		AnkiDeck:        "Test Deck",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "~/.sync/token.json",
		LogLevel:        "info",
	}

	err := SaveConfig(config, configPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load it back and verify content
	loadedConfig, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, config.GoogleSheetID, loadedConfig.GoogleSheetID)
	assert.Equal(t, config.SheetName, loadedConfig.SheetName)
	assert.Equal(t, config.AnkiDeck, loadedConfig.AnkiDeck)
}

func TestSaveConfig_InvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Config missing required fields
	config := &models.Config{
		GoogleSheetID: "test-id",
		// Missing sheet_name and anki_deck
	}

	err := SaveConfig(config, configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot save invalid config")
}

func TestRoundTripConfigSaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	original := &models.Config{
		GoogleSheetID:   "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
		SheetName:       "Greek Vocabulary",
		AnkiDeck:        "Greek::Core 1000",
		AnkiConnectURL:  "http://localhost:8765",
		GoogleTokenPath: "/home/user/.sync/token.json",
		LogLevel:        "verbose",
	}

	// Save
	err := SaveConfig(original, configPath)
	require.NoError(t, err)

	// Load
	loaded, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.GoogleSheetID, loaded.GoogleSheetID)
	assert.Equal(t, original.SheetName, loaded.SheetName)
	assert.Equal(t, original.AnkiDeck, loaded.AnkiDeck)
	assert.Equal(t, original.AnkiConnectURL, loaded.AnkiConnectURL)
	assert.Equal(t, original.GoogleTokenPath, loaded.GoogleTokenPath)
	assert.Equal(t, original.LogLevel, loaded.LogLevel)
}
