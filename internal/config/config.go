package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/sync/pkg/models"
	"gopkg.in/yaml.v3"
)

const (
	configDirName  = ".sync"
	configFileName = "config.yaml"
)

// GetConfigDir returns the configuration directory path (~/.sync/).
// The home directory is expanded to an absolute path.
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, configDirName), nil
}

// GetDefaultConfigPath returns the default configuration file path (~/.sync/config.yaml).
func GetDefaultConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, configFileName), nil
}

// GetDefaultCredentialsPath returns the default path to the service account key file.
func GetDefaultCredentialsPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "service-account.json"), nil
}

// EnsureConfigDir creates the configuration directory (~/.sync/) if it doesn't exist.
// The directory is created with 0755 permissions (rwxr-xr-x).
func EnsureConfigDir() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Create directory with permissions 0755
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	return nil
}

// LoadConfig reads and parses a YAML configuration file.
// Returns an error if the file doesn't exist or contains invalid YAML.
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s (run 'sync init' to create)", path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply default values for optional fields
	config.SetDefaults()

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig writes a configuration to a YAML file.
// The file is created with 0644 permissions (rw-r--r--).
// The parent directory must exist before calling this function.
func SaveConfig(config *models.Config, path string) error {
	// Validate before saving
	if err := config.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid config: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file with 0644 permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
