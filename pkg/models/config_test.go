package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTTSConfig_ElevenLabsFields(t *testing.T) {
	// Create a TTSConfig with all ElevenLabs fields populated
	config := &TTSConfig{
		Provider:                 "elevenlabs",
		Enabled:                  true,
		ElevenLabsAPIKey:         "test-api-key-12345",
		ElevenLabsVoiceID:        "test-voice-id-67890",
		ElevenLabsModel:          "eleven_multilingual_v2",
		ElevenLabsStability:      0.75,
		ElevenLabsSimilarity:     0.80,
		RequestDelayMs:           1000,
	}

	// Assert all fields are set correctly
	assert.Equal(t, "elevenlabs", config.Provider)
	assert.True(t, config.Enabled)
	assert.Equal(t, "test-api-key-12345", config.ElevenLabsAPIKey)
	assert.Equal(t, "test-voice-id-67890", config.ElevenLabsVoiceID)
	assert.Equal(t, "eleven_multilingual_v2", config.ElevenLabsModel)
	assert.Equal(t, 0.75, config.ElevenLabsStability)
	assert.Equal(t, 0.80, config.ElevenLabsSimilarity)
	assert.Equal(t, 1000, config.RequestDelayMs)
}
