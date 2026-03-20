package models

import (
	"fmt"
	"strings"
	"time"
)

// Default configuration values
const (
	DefaultAnkiConnectURL = "http://localhost:8765"
	DefaultLogLevel       = "info"
)

// Config represents the application configuration stored in ~/.sync/config.yaml
type Config struct {
	// GoogleSheetID is the unique identifier for the Google Spreadsheet
	GoogleSheetID string `yaml:"google_sheet_id"`

	// SheetName is the name of the specific sheet/tab within the spreadsheet
	SheetName string `yaml:"sheet_name"`

	// AnkiDeck is the name of the Anki deck where cards will be synced
	AnkiDeck string `yaml:"anki_deck"`

	// AnkiConnectURL is the URL for the AnkiConnect API (default: http://localhost:8765)
	AnkiConnectURL string `yaml:"anki_connect_url"`

	// GoogleTokenPath is the path to the OAuth2 token file (default: ~/.sync/token.json)
	GoogleTokenPath string `yaml:"google_token_path"`

	// LogLevel controls logging verbosity: "info", "verbose", or "debug"
	LogLevel string `yaml:"log_level"`

	// AnkiProfile is the Anki profile name (default: "User 1")
	AnkiProfile string `yaml:"anki_profile,omitempty"`

	// TextToSpeech configuration for Greek audio generation
	TextToSpeech *TTSConfig `yaml:"text_to_speech,omitempty"`
}

// TTSConfig holds configuration for text-to-speech providers.
type TTSConfig struct {
	// Provider selection
	Provider string `yaml:"provider"` // "google" or "elevenlabs", default "elevenlabs"
	Enabled  bool   `yaml:"enabled"`

	// Google Cloud TTS fields
	VoiceName      string  `yaml:"voice_name,omitempty"`
	AudioEncoding  string  `yaml:"audio_encoding,omitempty"`
	SpeakingRate   float64 `yaml:"speaking_rate,omitempty"`
	Pitch          float64 `yaml:"pitch,omitempty"`
	VolumeGainDb   float64 `yaml:"volume_gain_db,omitempty"`

	// ElevenLabs fields
	ElevenLabsAPIKey     string  `yaml:"elevenlabs_api_key,omitempty"`
	ElevenLabsVoiceID    string  `yaml:"elevenlabs_voice_id,omitempty"`
	ElevenLabsModel      string  `yaml:"elevenlabs_model,omitempty"`
	ElevenLabsStability  float64 `yaml:"elevenlabs_stability,omitempty"`
	ElevenLabsSimilarity float64 `yaml:"elevenlabs_similarity_boost,omitempty"`

	// Shared fields
	RequestDelayMs int `yaml:"request_delay_ms,omitempty"`
}

// Validate checks that all required configuration fields are present and valid.
// Returns an error describing any validation failures.
func (c *Config) Validate() error {
	var errors []string

	if strings.TrimSpace(c.GoogleSheetID) == "" {
		errors = append(errors, "google_sheet_id is required")
	}

	if strings.TrimSpace(c.SheetName) == "" {
		errors = append(errors, "sheet_name is required")
	}

	if strings.TrimSpace(c.AnkiDeck) == "" {
		errors = append(errors, "anki_deck is required")
	}

	// Validate log level if specified
	if c.LogLevel != "" {
		validLogLevels := map[string]bool{"info": true, "verbose": true, "debug": true}
		if !validLogLevels[strings.ToLower(c.LogLevel)] {
			errors = append(errors, "log_level must be 'info', 'verbose', or 'debug'")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// SetDefaults applies default values to optional configuration fields if they are empty.
func (c *Config) SetDefaults() {
	if c.AnkiConnectURL == "" {
		c.AnkiConnectURL = DefaultAnkiConnectURL
	}

	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}

	if c.AnkiProfile == "" {
		c.AnkiProfile = "User 1"
	}
}

// SyncState represents the persistent state of synchronization operations.
// Stored in ~/.sync/state.json to track when syncs last occurred.
type SyncState struct {
	// LastPullTimestamp is when the last successful pull from Anki to Sheets occurred
	LastPullTimestamp time.Time `json:"last_pull_timestamp"`

	// LastPushTimestamp is when the last successful push from Sheets to Anki occurred
	LastPushTimestamp time.Time `json:"last_push_timestamp"`

	// ConfigHash is a SHA256 hash of the config to detect configuration changes
	ConfigHash string `json:"config_hash"`
}
