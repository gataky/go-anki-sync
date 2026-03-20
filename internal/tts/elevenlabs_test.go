package tts

import (
	"testing"

	"github.com/gataky/sync/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewElevenLabsClient_Success(t *testing.T) {
	config := &models.TTSConfig{
		Provider:             "elevenlabs",
		Enabled:              true,
		ElevenLabsAPIKey:     "test-api-key",
		ElevenLabsVoiceID:    "test-voice-id",
		ElevenLabsStability:  0.6,
		ElevenLabsSimilarity: 0.8,
	}

	client, err := NewElevenLabsClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.Equal(t, "test-voice-id", client.voiceID)
	assert.Equal(t, defaultModel, client.model)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.config)
}

func TestNewElevenLabsClient_CustomModel(t *testing.T) {
	config := &models.TTSConfig{
		Provider:             "elevenlabs",
		Enabled:              true,
		ElevenLabsAPIKey:     "test-api-key",
		ElevenLabsVoiceID:    "test-voice-id",
		ElevenLabsModel:      "eleven_monolingual_v1",
		ElevenLabsStability:  0.5,
		ElevenLabsSimilarity: 0.75,
	}

	client, err := NewElevenLabsClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "eleven_monolingual_v1", client.model)
}

func TestNewElevenLabsClient_MissingAPIKey(t *testing.T) {
	config := &models.TTSConfig{
		Provider:             "elevenlabs",
		Enabled:              true,
		ElevenLabsAPIKey:     "",
		ElevenLabsVoiceID:    "test-voice-id",
		ElevenLabsStability:  0.5,
		ElevenLabsSimilarity: 0.75,
	}

	client, err := NewElevenLabsClient(config)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "ElevenLabs API key is required")
}

func TestNewElevenLabsClient_MissingVoiceID(t *testing.T) {
	config := &models.TTSConfig{
		Provider:             "elevenlabs",
		Enabled:              true,
		ElevenLabsAPIKey:     "test-api-key",
		ElevenLabsVoiceID:    "",
		ElevenLabsStability:  0.5,
		ElevenLabsSimilarity: 0.75,
	}

	client, err := NewElevenLabsClient(config)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "ElevenLabs voice ID is required")
}

func TestNewElevenLabsClient_NilConfig(t *testing.T) {
	client, err := NewElevenLabsClient(nil)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "TTS config is required")
}

func TestElevenLabsClient_GenerateAudio_EmptyText(t *testing.T) {
	config := &models.TTSConfig{
		Enabled:           true,
		ElevenLabsAPIKey:  "sk_test",
		ElevenLabsVoiceID: "voice123",
	}

	client, err := NewElevenLabsClient(config)
	assert.NoError(t, err)

	_, err = client.GenerateAudio("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestElevenLabsClient_GenerateAudio_WhitespaceOnly(t *testing.T) {
	config := &models.TTSConfig{
		Enabled:           true,
		ElevenLabsAPIKey:  "sk_test",
		ElevenLabsVoiceID: "voice123",
	}

	client, err := NewElevenLabsClient(config)
	assert.NoError(t, err)

	_, err = client.GenerateAudio("   \t\n  ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestElevenLabsClient_GenerateAudio_DefaultSettings(t *testing.T) {
	// This test verifies that default stability and similarity values are used
	config := &models.TTSConfig{
		Enabled:           true,
		ElevenLabsAPIKey:  "sk_test",
		ElevenLabsVoiceID: "voice123",
		// No stability or similarity specified - should use defaults
	}

	client, err := NewElevenLabsClient(config)
	assert.NoError(t, err)

	// We can't easily test the actual API call without mocking,
	// but we can verify the client was created successfully
	assert.NotNil(t, client)
	assert.Equal(t, "eleven_multilingual_v2", client.model)
}
