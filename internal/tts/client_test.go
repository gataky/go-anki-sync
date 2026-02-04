package tts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/sync/pkg/models"
)

// TestNewTTSClient_Success tests successful client creation.
// Note: This test requires valid credentials to run, so it's skipped in CI.
func TestNewTTSClient_Success(t *testing.T) {
	t.Skip("Skipping test that requires real Google Cloud credentials")

	config := &models.TTSConfig{
		Enabled:       true,
		VoiceName:     "el-GR-Wavenet-A",
		AudioEncoding: "MP3",
		SpeakingRate:  1.0,
		Pitch:         0.0,
		VolumeGainDb:  0.0,
	}

	ctx := context.Background()
	client, err := NewTTSClient(ctx, "credentials.json", config)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	if client != nil {
		defer client.Close()
	}
}

// TestGenerateAudio_EmptyText tests that empty text returns an error.
func TestGenerateAudio_EmptyText(t *testing.T) {
	config := &models.TTSConfig{
		Enabled:       true,
		VoiceName:     "el-GR-Wavenet-A",
		AudioEncoding: "MP3",
	}

	// Create a client with nil underlying client for testing validation logic
	client := &TTSClient{
		client: nil,
		config: config,
	}

	_, err := client.GenerateAudio("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestGenerateAudio_WhitespaceOnly tests that whitespace-only text returns an error.
func TestGenerateAudio_WhitespaceOnly(t *testing.T) {
	config := &models.TTSConfig{
		Enabled:       true,
		VoiceName:     "el-GR-Wavenet-A",
		AudioEncoding: "MP3",
	}

	// Create a client with nil underlying client for testing validation logic
	client := &TTSClient{
		client: nil,
		config: config,
	}

	_, err := client.GenerateAudio("   \t\n  ")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestValidateGreekText tests the validation helper function.
func TestValidateGreekText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid text", "γεια", false},
		{"Valid with spaces", "γεια σου", false},
		{"Empty string", "", true},
		{"Whitespace only", "   ", true},
		{"Tab only", "\t", true},
		{"Newline only", "\n", true},
		{"Mixed whitespace", " \t\n ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGreekText(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
