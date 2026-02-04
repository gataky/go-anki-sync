package tts

import (
	"context"
	"fmt"
	"strings"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"google.golang.org/api/option"

	"github.com/yourusername/sync/pkg/models"
)

// TTSClient handles Google Cloud Text-to-Speech operations
type TTSClient struct {
	client *texttospeech.Client
	config *models.TTSConfig
}

// NewTTSClient creates a TTS client using service account credentials
func NewTTSClient(ctx context.Context, credentialsPath string, config *models.TTSConfig) (*TTSClient, error) {
	if config == nil {
		return nil, fmt.Errorf("TTS config is required")
	}

	if strings.TrimSpace(credentialsPath) == "" {
		return nil, fmt.Errorf("credentials path is required")
	}

	// Create TTS client with service account credentials
	client, err := texttospeech.NewClient(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS client: %w", err)
	}

	return &TTSClient{
		client: client,
		config: config,
	}, nil
}

// GenerateAudio synthesizes speech for Greek text and returns MP3 data
func (c *TTSClient) GenerateAudio(greekText string) ([]byte, error) {
	// Validate input
	if err := validateGreekText(greekText); err != nil {
		return nil, err
	}

	// Set default values if not configured
	speakingRate := c.config.SpeakingRate
	if speakingRate == 0 {
		speakingRate = 1.0
	}

	// Build synthesis request
	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{
				Text: greekText,
			},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "el-GR",
			Name:         c.config.VoiceName,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
			SpeakingRate:  speakingRate,
			Pitch:         c.config.Pitch,
			VolumeGainDb:  c.config.VolumeGainDb,
		},
	}

	// Call TTS API
	ctx := context.Background()
	resp, err := c.client.SynthesizeSpeech(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("TTS synthesis failed for '%s': %w", greekText, err)
	}

	return resp.AudioContent, nil
}

// Close closes the TTS client connection
func (c *TTSClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// validateGreekText checks if the Greek text is valid for TTS
func validateGreekText(text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("Greek text cannot be empty or whitespace-only")
	}
	return nil
}
