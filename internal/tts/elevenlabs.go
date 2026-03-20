package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gataky/sync/pkg/models"
)

const (
	elevenLabsBaseURL  = "https://api.elevenlabs.io/v1"
	defaultModel       = "eleven_multilingual_v2"
	defaultStability   = 0.5
	defaultSimilarity  = 0.75
)

// ElevenLabsClient handles ElevenLabs Text-to-Speech operations
type ElevenLabsClient struct {
	apiKey     string
	voiceID    string
	model      string
	httpClient *http.Client
	config     *models.TTSConfig
}

// voiceSettings represents the ElevenLabs voice settings
type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}

// elevenLabsRequest represents the request payload for ElevenLabs API
type elevenLabsRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings voiceSettings `json:"voice_settings"`
}

// NewElevenLabsClient creates an ElevenLabs TTS client
func NewElevenLabsClient(config *models.TTSConfig) (*ElevenLabsClient, error) {
	if config == nil {
		return nil, fmt.Errorf("TTS config is required")
	}

	if strings.TrimSpace(config.ElevenLabsAPIKey) == "" {
		return nil, fmt.Errorf("ElevenLabs API key is required")
	}

	if strings.TrimSpace(config.ElevenLabsVoiceID) == "" {
		return nil, fmt.Errorf("ElevenLabs voice ID is required")
	}

	// Set default model if not specified
	model := config.ElevenLabsModel
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}

	return &ElevenLabsClient{
		apiKey:  config.ElevenLabsAPIKey,
		voiceID: config.ElevenLabsVoiceID,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
	}, nil
}

// GenerateAudio synthesizes speech for Greek text using ElevenLabs and returns audio data
func (c *ElevenLabsClient) GenerateAudio(greekText string) ([]byte, error) {
	// Validate input
	if err := validateGreekText(greekText); err != nil {
		return nil, err
	}

	// Set default values for voice settings if not configured
	stability := c.config.ElevenLabsStability
	if stability == 0 {
		stability = defaultStability
	}

	similarity := c.config.ElevenLabsSimilarity
	if similarity == 0 {
		similarity = defaultSimilarity
	}

	// Build request payload
	payload := elevenLabsRequest{
		Text:    greekText,
		ModelID: c.model,
		VoiceSettings: voiceSettings{
			Stability:       stability,
			SimilarityBoost: similarity,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsBaseURL, c.voiceID)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.apiKey)

	// Make API call
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs API request failed for '%s': %w", greekText, err)
	}
	defer resp.Body.Close()

	// Read response body
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs API returned status %d: %s", resp.StatusCode, string(audioData))
	}

	return audioData, nil
}

// Close closes the ElevenLabs client (no-op for HTTP client)
func (c *ElevenLabsClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
