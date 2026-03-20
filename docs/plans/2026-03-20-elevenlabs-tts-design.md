# ElevenLabs TTS Integration Design

**Date:** 2026-03-20
**Status:** Approved

## Overview

Add ElevenLabs as an alternative TTS provider alongside Google Cloud TTS, with provider selection via configuration. ElevenLabs will become the default TTS provider going forward. The existing interface-based architecture makes this straightforward - we'll create a new ElevenLabs client implementing the same `TTSClientInterface`.

## Goals

1. Add ElevenLabs text-to-speech support for Greek audio generation
2. Allow users to choose between Google TTS and ElevenLabs via configuration
3. Make ElevenLabs the default provider when both are configured
4. Support voice selection via voice ID
5. Support optional ElevenLabs voice settings (stability, similarity, style)
6. Maintain backward compatibility with existing Google TTS configurations

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                       Bootstrap                              │
│  ┌────────────────────────────────────────────────────┐     │
│  │  TTS Factory                                       │     │
│  │  - Check provider field in config                 │     │
│  │  - Instantiate Google or ElevenLabs client        │     │
│  │  - Return TTSClientInterface                      │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                            │
           ┌────────────────┴────────────────┐
           │                                  │
           ▼                                  ▼
┌────────────────────────┐      ┌────────────────────────┐
│  Google TTS Client     │      │  ElevenLabs Client     │
│  (existing)            │      │  (new)                 │
│                        │      │                        │
│  - NewTTSClient()      │      │  - NewElevenLabsClient()│
│  - GenerateAudio()     │      │  - GenerateAudio()     │
│  - Close()             │      │  - Close()             │
└────────────────────────┘      └────────────────────────┘
           │                                  │
           └────────────────┬─────────────────┘
                            │
                            ▼
                  TTSClientInterface
                  - GenerateAudio(greekText) ([]byte, error)
```

### Interface Stability

The existing `TTSClientInterface` in `internal/sync/interfaces.go` remains unchanged:

```go
type TTSClientInterface interface {
    GenerateAudio(greekText string) ([]byte, error)
}
```

This ensures all existing code using TTS (pusher, tests) continues to work without modification.

## Detailed Design

### 1. Configuration Model Changes

**File:** `pkg/models/config.go`

Add provider selection and ElevenLabs fields to `TTSConfig`:

```go
type TTSConfig struct {
    // Provider selection
    Provider string `yaml:"provider"` // "google" or "elevenlabs", default "elevenlabs"
    Enabled  bool   `yaml:"enabled"`

    // Google Cloud TTS fields (existing)
    VoiceName      string  `yaml:"voice_name,omitempty"`
    AudioEncoding  string  `yaml:"audio_encoding,omitempty"`
    SpeakingRate   float64 `yaml:"speaking_rate,omitempty"`
    Pitch          float64 `yaml:"pitch,omitempty"`
    VolumeGainDb   float64 `yaml:"volume_gain_db,omitempty"`

    // ElevenLabs fields (new)
    ElevenLabsAPIKey        string  `yaml:"elevenlabs_api_key,omitempty"`
    ElevenLabsVoiceID       string  `yaml:"elevenlabs_voice_id,omitempty"`
    ElevenLabsModel         string  `yaml:"elevenlabs_model,omitempty"`        // default: "eleven_multilingual_v2"
    ElevenLabsStability     float64 `yaml:"elevenlabs_stability,omitempty"`     // 0-1, default: 0.5
    ElevenLabsSimilarity    float64 `yaml:"elevenlabs_similarity_boost,omitempty"` // 0-1, default: 0.75

    // Shared fields
    RequestDelayMs int `yaml:"request_delay_ms,omitempty"` // Used by both providers
}
```

**Validation rules:**
- If `Provider` is empty, default to "elevenlabs"
- If `Provider == "google"`: require `VoiceName`
- If `Provider == "elevenlabs"`: require `ElevenLabsAPIKey` and `ElevenLabsVoiceID`
- Unknown provider values should error

### 2. ElevenLabs Client Implementation

**File:** `internal/tts/elevenlabs.go` (new)

```go
package tts

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"

    "github.com/gataky/sync/pkg/models"
)

const (
    elevenLabsBaseURL = "https://api.elevenlabs.io/v1"
    defaultModel      = "eleven_multilingual_v2"
    defaultStability  = 0.5
    defaultSimilarity = 0.75
)

// ElevenLabsClient handles ElevenLabs Text-to-Speech operations
type ElevenLabsClient struct {
    apiKey     string
    voiceID    string
    model      string
    httpClient *http.Client
    config     *models.TTSConfig
}

// NewElevenLabsClient creates an ElevenLabs TTS client
func NewElevenLabsClient(ctx context.Context, config *models.TTSConfig) (*ElevenLabsClient, error) {
    if config == nil {
        return nil, fmt.Errorf("TTS config is required")
    }

    if strings.TrimSpace(config.ElevenLabsAPIKey) == "" {
        return nil, fmt.Errorf("ElevenLabs API key is required")
    }

    if strings.TrimSpace(config.ElevenLabsVoiceID) == "" {
        return nil, fmt.Errorf("ElevenLabs voice ID is required")
    }

    model := config.ElevenLabsModel
    if model == "" {
        model = defaultModel
    }

    return &ElevenLabsClient{
        apiKey:     config.ElevenLabsAPIKey,
        voiceID:    config.ElevenLabsVoiceID,
        model:      model,
        httpClient: &http.Client{},
        config:     config,
    }, nil
}

// GenerateAudio synthesizes speech for Greek text and returns MP3 data
func (c *ElevenLabsClient) GenerateAudio(greekText string) ([]byte, error) {
    // Validate input
    if err := validateGreekText(greekText); err != nil {
        return nil, err
    }

    // Build request payload
    stability := c.config.ElevenLabsStability
    if stability == 0 {
        stability = defaultStability
    }

    similarity := c.config.ElevenLabsSimilarity
    if similarity == 0 {
        similarity = defaultSimilarity
    }

    payload := map[string]interface{}{
        "text":     greekText,
        "model_id": c.model,
        "voice_settings": map[string]interface{}{
            "stability":        stability,
            "similarity_boost": similarity,
        },
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Make API request
    url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsBaseURL, c.voiceID)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("xi-api-key", c.apiKey)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("ElevenLabs API request failed: %w", err)
    }
    defer resp.Body.Close()

    // Read response
    audioData, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // Check status code
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ElevenLabs API error (status %d): %s", resp.StatusCode, string(audioData))
    }

    return audioData, nil
}

// Close cleans up resources (no-op for HTTP client)
func (c *ElevenLabsClient) Close() error {
    return nil
}
```

**API Details:**
- Endpoint: `POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}`
- Authentication: `xi-api-key` header
- Returns: MP3 audio stream
- Model: `eleven_multilingual_v2` supports Greek and 28 other languages

### 3. Factory Pattern in Bootstrap

**File:** `internal/cli/bootstrap.go`

Replace direct instantiation with factory function:

```go
// Conditionally initialize TTS
if opts.EnableTTS && cfg.TextToSpeech != nil && cfg.TextToSpeech.Enabled {
    ttsCtx := context.Background()
    ttsClient, err := createTTSClient(ttsCtx, credentialsPath, cfg.TextToSpeech)
    if err != nil {
        return nil, printError("failed to initialize TTS client: %w", err)
    }
    ctx.TTSClient = ttsClient
    logger.Info("TTS client initialized successfully (provider: %s)", cfg.TextToSpeech.Provider)
} else if opts.EnableTTS {
    logger.Info("TTS is disabled, skipping audio generation")
}
```

Add factory function:

```go
// createTTSClient creates the appropriate TTS client based on configuration
func createTTSClient(ctx context.Context, credentialsPath string, config *models.TTSConfig) (TTSClientInterface, error) {
    // Default to elevenlabs if no provider specified
    provider := config.Provider
    if provider == "" {
        provider = "elevenlabs"
    }

    switch strings.ToLower(provider) {
    case "google":
        return tts.NewTTSClient(ctx, credentialsPath, config)
    case "elevenlabs":
        return tts.NewElevenLabsClient(ctx, config)
    default:
        return nil, fmt.Errorf("unknown TTS provider: %s (must be 'google' or 'elevenlabs')", provider)
    }
}
```

Note: Need to adjust import to allow both implementations:
```go
import (
    "github.com/gataky/sync/internal/tts"
    // TTSClientInterface is from internal/sync/interfaces.go
)
```

### 4. Configuration Validation

Add validation method to `TTSConfig` in `pkg/models/config.go`:

```go
// Validate checks TTS configuration
func (t *TTSConfig) Validate() error {
    if !t.Enabled {
        return nil // No validation needed if disabled
    }

    provider := t.Provider
    if provider == "" {
        provider = "elevenlabs" // Default
    }

    switch strings.ToLower(provider) {
    case "google":
        if strings.TrimSpace(t.VoiceName) == "" {
            return fmt.Errorf("voice_name is required for Google TTS provider")
        }
    case "elevenlabs":
        if strings.TrimSpace(t.ElevenLabsAPIKey) == "" {
            return fmt.Errorf("elevenlabs_api_key is required for ElevenLabs provider")
        }
        if strings.TrimSpace(t.ElevenLabsVoiceID) == "" {
            return fmt.Errorf("elevenlabs_voice_id is required for ElevenLabs provider")
        }
    default:
        return fmt.Errorf("unknown TTS provider: %s (must be 'google' or 'elevenlabs')", provider)
    }

    return nil
}
```

Call this from `Config.Validate()`:
```go
func (c *Config) Validate() error {
    // ... existing validation ...

    // Validate TTS config if present
    if c.TextToSpeech != nil {
        if err := c.TextToSpeech.Validate(); err != nil {
            return err
        }
    }

    return nil
}
```

## Testing Strategy

### Unit Tests

**File:** `internal/tts/elevenlabs_test.go` (new)

Test cases:
- `TestNewElevenLabsClient_Success` - successful initialization
- `TestNewElevenLabsClient_MissingAPIKey` - error on missing API key
- `TestNewElevenLabsClient_MissingVoiceID` - error on missing voice ID
- `TestGenerateAudio_Success` - mock successful API response
- `TestGenerateAudio_EmptyText` - error on empty Greek text
- `TestGenerateAudio_APIError` - handle API error responses
- `TestGenerateAudio_DefaultSettings` - verify default model and settings

### Integration Tests

Existing pusher tests use `TTSClientInterface` via mocks, so no changes needed. The interface abstraction ensures compatibility.

### Manual Testing

1. Configure ElevenLabs in `~/.sync/config.yaml`
2. Run `./sync push --dry-run` to verify client initialization
3. Run `./sync push` to generate audio with ElevenLabs
4. Verify audio quality in Anki cards
5. Test switching back to Google TTS provider
6. Test validation errors (missing API key, invalid provider)

## Migration Guide

### For Existing Users (Google TTS)

Existing configurations will continue to work. To explicitly specify Google:

```yaml
text_to_speech:
  provider: "google"  # Add this line
  enabled: true
  voice_name: "el-GR-Wavenet-A"
  # ... rest of existing config
```

### Switching to ElevenLabs

1. Get an ElevenLabs API key from https://elevenlabs.io
2. Choose a voice ID (browse voices at https://elevenlabs.io/voice-library)
3. Update config:

```yaml
text_to_speech:
  provider: "elevenlabs"
  enabled: true
  elevenlabs_api_key: "your-api-key-here"
  elevenlabs_voice_id: "21m00Tcm4TlvDq8ikWAM"  # Example: Rachel voice
  elevenlabs_model: "eleven_multilingual_v2"     # Optional
  request_delay_ms: 100
```

### For New Users

Default to ElevenLabs:

```yaml
text_to_speech:
  enabled: true
  elevenlabs_api_key: "your-api-key-here"
  elevenlabs_voice_id: "your-voice-id"
```

Provider defaults to "elevenlabs" if not specified.

## Documentation Updates

### README.md Changes

1. **Update "Greek Audio Generation" section title** to "Audio Generation (Text-to-Speech)"
2. **Add "Supported Providers" subsection** before requirements
3. **Restructure setup** into provider-specific sections
4. **Update example config** to show both providers
5. **Add ElevenLabs troubleshooting** section

### New README Content Outline

```markdown
## Audio Generation (Text-to-Speech)

The tool automatically generates high-quality audio pronunciation for Greek words.

### Supported Providers

- **ElevenLabs** (recommended, default) - High-quality neural voices with natural pronunciation
- **Google Cloud TTS** - WaveNet and Standard voices

### ElevenLabs Setup (Recommended)

1. Create account at https://elevenlabs.io
2. Get your API key from Settings
3. Choose a voice from the Voice Library
4. Add to `~/.sync/config.yaml`:
   ```yaml
   text_to_speech:
     provider: "elevenlabs"  # or omit, defaults to elevenlabs
     enabled: true
     elevenlabs_api_key: "your-key-here"
     elevenlabs_voice_id: "21m00Tcm4TlvDq8ikWAM"
   ```

**Available voices**: Browse at https://elevenlabs.io/voice-library
**Recommended model**: `eleven_multilingual_v2` (supports Greek + 28 languages)

### Google Cloud TTS Setup

[Keep existing Google TTS setup instructions]

### Configuration Reference

[Show complete config example with both providers]
```

## Example Configurations

### ElevenLabs (Default)

```yaml
text_to_speech:
  provider: "elevenlabs"  # Optional, defaults to elevenlabs
  enabled: true
  elevenlabs_api_key: "sk_abc123..."
  elevenlabs_voice_id: "21m00Tcm4TlvDq8ikWAM"
  elevenlabs_model: "eleven_multilingual_v2"  # Optional
  elevenlabs_stability: 0.5                    # Optional, 0-1
  elevenlabs_similarity_boost: 0.75            # Optional, 0-1
  request_delay_ms: 100                        # Rate limiting
```

### Google Cloud TTS

```yaml
text_to_speech:
  provider: "google"
  enabled: true
  voice_name: "el-GR-Wavenet-A"
  audio_encoding: "MP3"
  speaking_rate: 1.0
  pitch: 0.0
  volume_gain_db: 0.0
  request_delay_ms: 100
```

### Minimal ElevenLabs (uses defaults)

```yaml
text_to_speech:
  enabled: true
  elevenlabs_api_key: "sk_abc123..."
  elevenlabs_voice_id: "21m00Tcm4TlvDq8ikWAM"
```

## Error Handling

### Configuration Errors

- Missing API key → Clear error: "elevenlabs_api_key is required for ElevenLabs provider"
- Missing voice ID → Clear error: "elevenlabs_voice_id is required for ElevenLabs provider"
- Invalid provider → Clear error: "unknown TTS provider: xyz (must be 'google' or 'elevenlabs')"

### Runtime Errors

- API rate limit → Log warning, respect `request_delay_ms`
- Invalid API key → Error with hint to check API key in config
- Network error → Log error, card created without audio (graceful degradation)
- Empty Greek text → Skip audio generation, log debug message

All errors should be logged with context and allow the sync operation to continue when possible.

## Implementation Checklist

- [ ] Add ElevenLabs fields to `TTSConfig` in `pkg/models/config.go`
- [ ] Add `Validate()` method to `TTSConfig`
- [ ] Create `internal/tts/elevenlabs.go` client
- [ ] Create `internal/tts/elevenlabs_test.go` unit tests
- [ ] Add factory function `createTTSClient()` in `internal/cli/bootstrap.go`
- [ ] Update bootstrap to use factory pattern
- [ ] Test with ElevenLabs API (manual)
- [ ] Test backward compatibility with Google TTS
- [ ] Update README.md with ElevenLabs instructions
- [ ] Add migration guide to README
- [ ] Update configuration examples in README
- [ ] Test validation errors
- [ ] Test default provider behavior (should use ElevenLabs)

## Future Enhancements (Out of Scope)

- Voice cloning support
- Streaming audio generation
- Caching audio by voice + text hash
- Support for additional TTS providers (Azure, AWS Polly)
- Per-card voice selection
- Pronunciation lexicons for Greek words
