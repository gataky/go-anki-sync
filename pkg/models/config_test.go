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

func TestTTSConfig_Validate_GoogleProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *TTSConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid google config",
			config: &TTSConfig{
				Provider:  "google",
				Enabled:   true,
				VoiceName: "el-GR-Standard-A",
			},
			wantErr: false,
		},
		{
			name: "missing voice_name error",
			config: &TTSConfig{
				Provider: "google",
				Enabled:  true,
			},
			wantErr: true,
			errMsg:  "voice_name is required for Google TTS provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTTSConfig_Validate_ElevenLabsProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *TTSConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid elevenlabs config",
			config: &TTSConfig{
				Provider:             "elevenlabs",
				Enabled:              true,
				ElevenLabsAPIKey:     "test-key",
				ElevenLabsVoiceID:    "test-voice",
			},
			wantErr: false,
		},
		{
			name: "missing api_key error",
			config: &TTSConfig{
				Provider:          "elevenlabs",
				Enabled:           true,
				ElevenLabsVoiceID: "test-voice",
			},
			wantErr: true,
			errMsg:  "elevenlabs_api_key is required for ElevenLabs provider",
		},
		{
			name: "missing voice_id error",
			config: &TTSConfig{
				Provider:         "elevenlabs",
				Enabled:          true,
				ElevenLabsAPIKey: "test-key",
			},
			wantErr: true,
			errMsg:  "elevenlabs_voice_id is required for ElevenLabs provider",
		},
		{
			name: "default provider (empty string)",
			config: &TTSConfig{
				Provider:          "",
				Enabled:           true,
				ElevenLabsAPIKey:  "test-key",
				ElevenLabsVoiceID: "test-voice",
			},
			wantErr: false,
		},
		{
			name: "unknown provider error",
			config: &TTSConfig{
				Provider: "unknown",
				Enabled:  true,
			},
			wantErr: true,
			errMsg:  "unknown TTS provider: unknown (must be 'google' or 'elevenlabs')",
		},
		{
			name: "disabled tts skips validation",
			config: &TTSConfig{
				Provider: "elevenlabs",
				Enabled:  false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
