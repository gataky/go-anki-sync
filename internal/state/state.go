package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/sync/pkg/models"
	"gopkg.in/yaml.v3"
)

const stateFileName = "state.json"

// GetDefaultStatePath returns the default state file path (~/.sync/state.json).
func GetDefaultStatePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".sync", stateFileName), nil
}

// LoadState reads and parses a JSON state file.
// If the file doesn't exist, returns an empty state with zero timestamps (not an error).
// This allows the tool to work on first run without requiring a state file.
func LoadState(path string) (*models.SyncState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty state on first run - this is not an error
			return &models.SyncState{
				LastPullTimestamp: time.Time{}, // Zero time
				LastPushTimestamp: time.Time{}, // Zero time
				ConfigHash:        "",
			}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state models.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// SaveState writes a state to a JSON file.
// The file is created with 0644 permissions (rw-r--r--).
// The parent directory must exist before calling this function.
func SaveState(state *models.SyncState, path string) error {
	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state to JSON: %w", err)
	}

	// Write to file with 0644 permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// CalculateConfigHash computes a SHA256 hash of the configuration.
// This hash is used to detect when the configuration has changed between syncs.
// Returns the hash as a hex-encoded string.
func CalculateConfigHash(config *models.Config) (string, error) {
	// Marshal config to YAML for consistent ordering
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config for hashing: %w", err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256(data)

	// Return as hex string
	return fmt.Sprintf("%x", hash), nil
}

// Manager implements the StateManager interface for loading and saving sync state.
type Manager struct{}

// LoadState implements StateManager.LoadState
func (m *Manager) LoadState(path string) (*models.SyncState, error) {
	return LoadState(path)
}

// SaveState implements StateManager.SaveState
func (m *Manager) SaveState(state *models.SyncState, path string) error {
	return SaveState(state, path)
}

// GetDefaultStatePath implements StateManager.GetDefaultStatePath
func (m *Manager) GetDefaultStatePath() string {
	path, _ := GetDefaultStatePath()
	return path
}
