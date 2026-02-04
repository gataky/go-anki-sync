# PRD: Greek Audio Generation with Text-to-Speech

## Introduction/Overview

Add automatic Greek pronunciation audio to new vocabulary cards using Google Text-to-Speech API. When creating new Anki cards, the system will:
1. Check if audio file already exists in Anki media collection (by filename: `{greek_word}.mp3`)
2. If not found, generate audio using Google TTS API
3. Attach audio to the "Back" field (Greek word) in the Anki note

Audio generation is optional - failures will be logged but won't block card creation. This feature only applies to newly created cards; existing cards without audio remain unchanged.

## Goals

**Functional Goals:**
1. Automatically generate Greek pronunciation audio for new vocabulary cards
2. Avoid duplicate audio generation by checking existing Anki media files
3. Attach audio to the Greek word field in Anki notes
4. Support dry-run mode to preview audio generation without creating files

**Technical Goals:**
1. Integrate Google Cloud Text-to-Speech API using existing service account credentials
2. Maintain sub-500ms audio generation per card (network dependent)
3. Handle TTS API failures gracefully without blocking card creation
4. Achieve 80%+ unit test coverage with mocked TTS API
5. Add zero performance regression to existing sync operations when audio generation is skipped

## Tech Stack & Architecture

**Languages:**
- Go 1.25.0

**Frameworks & Libraries:**
- **Existing (leverage):**
  - `github.com/atselvan/ankiconnect` v1.1.1 - Anki integration (already has Audio support)
  - `golang.org/x/oauth2` v0.34.0 - OAuth2 for Google APIs
  - `google.golang.org/api` v0.264.0 - Google API client libraries
- **New (add):**
  - `cloud.google.com/go/texttospeech` - Google Cloud TTS client library

**Architectural Pattern:**
- Extend existing layered architecture:
  - `internal/tts/` - New package for TTS operations
  - `internal/anki/client.go` - Extend to handle audio attachment
  - `internal/sync/pusher.go` - Orchestrate audio generation during card creation

**Existing Patterns to Follow:**
- Retry logic with exponential backoff (`internal/anki/retry.go`)
- Error handling: log errors but continue (similar to `pusher.go` partial failure handling)
- Configuration via YAML (`pkg/models/config.go`)
- Service account authentication (same as Google Sheets integration in `internal/sheets/client.go`)

**Data Storage:**
- No local caching - audio files stored directly in Anki media collection
- Configuration stored in existing `config.yaml`

**External Services/APIs:**
- Google Cloud Text-to-Speech API (requires service account with TTS API enabled)
- AnkiConnect API (existing) - for checking/storing media files

## Functional Requirements

1. **Audio Filename Convention**: Audio files must be named exactly as the Greek word with `.mp3` extension (e.g., `γεια.mp3`, `καλημέρα.mp3`). No normalization or transliteration.

2. **Duplicate Detection**: Before generating audio, check if `{greek_word}.mp3` already exists in Anki's media collection using `MediaManager.GetMediaFileNames()`. If found, skip generation.

3. **Audio Generation**: Use Google Cloud TTS API to synthesize Greek text with configurable voice and audio settings (from `config.yaml`).

4. **Audio Attachment**: Attach generated audio to the "Back" field (Greek word) in the Anki note using the `Audio` field in `ankiconnect.Note`.

5. **Scope Limitation**: Only generate audio when creating NEW cards (`AnkiID == 0`). Do NOT generate audio when updating existing cards or during backfill operations.

6. **Error Handling**: If audio generation fails for any reason (API error, network issue, etc.), log the error and continue creating the card WITHOUT audio. Audio is optional and should never block card creation.

7. **Trust Existing Audio**: If an audio file exists in Anki (by filename), trust it completely. Never validate, regenerate, or replace existing audio files.

8. **Dry-Run Support**: When `--dry-run` flag is set, log what audio files WOULD be generated but don't call TTS API or modify Anki.

9. **Sequential Processing**: Process audio generation sequentially (one card at a time) with optional configurable delay between TTS API calls to avoid rate limiting.

10. **Configuration Requirements**: Add new configuration section in `config.yaml`:
    - TTS voice name (e.g., "el-GR-Wavenet-A")
    - Audio encoding (MP3)
    - Speaking rate, pitch, volume gain
    - API request delay

11. **Service Account Authentication**: Use the same service account credentials (`credentials_path` in config.yaml) that are currently used for Google Sheets access. Service account must have `roles/cloudtts.user` permission.

12. **Validation**: Skip audio generation if Greek text is empty or contains only whitespace. Log warning and continue.

## Technical Specifications

### Data Models/Schema

**Add to `pkg/models/config.go`:**
```go
type Config struct {
    // ... existing fields ...

    // TextToSpeech configuration for audio generation
    TextToSpeech *TTSConfig `yaml:"text_to_speech,omitempty"`
}

type TTSConfig struct {
    // Enabled controls whether audio generation is active
    Enabled bool `yaml:"enabled"`

    // VoiceName is the Google TTS voice (e.g., "el-GR-Wavenet-A")
    VoiceName string `yaml:"voice_name"`

    // AudioEncoding is the output format (e.g., "MP3")
    AudioEncoding string `yaml:"audio_encoding"`

    // SpeakingRate is the speed (0.25 to 4.0, default 1.0)
    SpeakingRate float64 `yaml:"speaking_rate,omitempty"`

    // Pitch is the voice pitch (-20.0 to 20.0, default 0.0)
    Pitch float64 `yaml:"pitch,omitempty"`

    // VolumeGainDb is the volume gain in dB (-96.0 to 16.0, default 0.0)
    VolumeGainDb float64 `yaml:"volume_gain_db,omitempty"`

    // RequestDelayMs is the delay between API calls to avoid rate limiting
    RequestDelayMs int `yaml:"request_delay_ms,omitempty"`
}
```

**Default Configuration Values:**
```yaml
text_to_speech:
  enabled: true
  voice_name: "el-GR-Wavenet-A"
  audio_encoding: "MP3"
  speaking_rate: 1.0
  pitch: 0.0
  volume_gain_db: 0.0
  request_delay_ms: 100
```

**No changes to `VocabCard` model** - No tracking of audio state needed.

### Module Structure

**New Package: `internal/tts/`**

**File: `internal/tts/client.go`**
```go
package tts

import (
    texttospeech "cloud.google.com/go/texttospeech/apiv1"
    "github.com/yourusername/sync/pkg/models"
)

// TTSClient handles Google Cloud Text-to-Speech operations
type TTSClient struct {
    client *texttospeech.Client
    config *models.TTSConfig
}

// NewTTSClient creates a TTS client using service account credentials
func NewTTSClient(ctx context.Context, credentialsPath string, config *models.TTSConfig) (*TTSClient, error)

// GenerateAudio synthesizes speech for Greek text and returns MP3 data
// Returns (audioData []byte, error)
func (c *TTSClient) GenerateAudio(greekText string) ([]byte, error)

// Close closes the TTS client connection
func (c *TTSClient) Close() error
```

**Implementation Logic for `GenerateAudio`:**
```
1. Validate input:
   - Return error if greekText is empty or whitespace-only

2. Build SynthesizeSpeechRequest:
   - Input: SynthesisInput{Text: greekText}
   - Voice: VoiceSelectionParams{
       LanguageCode: "el-GR",
       Name: config.VoiceName,
     }
   - AudioConfig: AudioConfig{
       AudioEncoding: MP3,
       SpeakingRate: config.SpeakingRate,
       Pitch: config.Pitch,
       VolumeGainDb: config.VolumeGainDb,
     }

3. Call client.SynthesizeSpeech(ctx, req)

4. Return audio.AudioContent (raw MP3 bytes)

5. Error handling:
   - Wrap errors with context: fmt.Errorf("TTS synthesis failed for '%s': %w", greekText, err)
```

**File: `internal/tts/client_test.go`**
```go
// Unit tests with mocked TTS client
// Test cases:
// - TestNewTTSClient_Success
// - TestNewTTSClient_InvalidCredentials
// - TestGenerateAudio_Success
// - TestGenerateAudio_EmptyText
// - TestGenerateAudio_APIError
// - TestGenerateAudio_ConfigOptions
```

### API Contracts

**AnkiConnect Media API (existing):**
```go
// Check if audio file exists
GetMediaFileNames(pattern string) (*[]string, error)
// Example: GetMediaFileNames("γεια.mp3") returns ["γεια.mp3"] if exists

// Store audio file in Anki media collection
StoreMediaFile(filename string, encodedMediaContent string) (*string, error)
// encodedMediaContent: base64-encoded audio data
// Returns: filename on success
```

**Google Cloud TTS API:**
```
Request: texttospeech.SynthesizeSpeechRequest
{
  input: {text: "γεια"},
  voice: {language_code: "el-GR", name: "el-GR-Wavenet-A"},
  audio_config: {audio_encoding: MP3, speaking_rate: 1.0, pitch: 0.0, volume_gain_db: 0.0}
}

Response: texttospeech.SynthesizeSpeechResponse
{
  audio_content: []byte  // Raw MP3 data
}
```

### Integration Points

**Modify `internal/anki/client.go`:**

**Add method:**
```go
// CheckAudioExists checks if an audio file exists in Anki's media collection
func (c *AnkiClient) CheckAudioExists(filename string) (bool, error) {
    fileNames, err := c.client.Media.GetMediaFileNames(filename)
    if err != nil {
        return false, fmt.Errorf("failed to check audio existence: %w", err)
    }

    if fileNames == nil || len(*fileNames) == 0 {
        return false, nil
    }

    // Check if exact filename match exists
    for _, name := range *fileNames {
        if name == filename {
            return true, nil
        }
    }

    return false, nil
}

// StoreAudioFile stores an audio file in Anki's media collection
func (c *AnkiClient) StoreAudioFile(filename string, audioData []byte) error {
    // Convert to base64
    encodedData := base64.StdEncoding.EncodeToString(audioData)

    // Store via AnkiConnect
    _, err := c.client.Media.StoreMediaFile(filename, encodedData)
    if err != nil {
        return fmt.Errorf("failed to store audio file '%s': %w", filename, err)
    }

    return nil
}
```

**Modify `AddNote` method signature and logic:**
```go
// AddNote creates a single note in Anki and returns the generated note ID.
// If audioData is provided, attaches it to the Back field.
func (c *AnkiClient) AddNote(deckName, modelName string, card *models.VocabCard, audioData []byte) (int64, error) {
    // ... existing duplicate check logic ...

    // Build fields from card
    fields := ankiconnect.Fields{
        "Front":    card.English,
        "Back":     card.Greek,
        "Grammar":  buildGrammarField(card.PartOfSpeech, card.Attributes),
        "Examples": formatExamplesHTML(card.Examples),
    }

    // Build tags
    tags := buildTags(card.Tag, card.SubTag1, card.SubTag2)

    // Build note with optional audio
    note := ankiconnect.Note{
        DeckName:  deckName,
        ModelName: modelName,
        Fields:    fields,
        Tags:      tags,
        Options: &ankiconnect.Options{
            AllowDuplicate: true,
        },
    }

    // Add audio if provided
    if audioData != nil && len(audioData) > 0 {
        filename := fmt.Sprintf("%s.mp3", card.Greek)

        // Convert to base64 for ankiconnect
        encodedData := base64.StdEncoding.EncodeToString(audioData)

        note.Audio = []ankiconnect.Audio{
            {
                Data:     encodedData,
                Filename: filename,
                Fields:   []string{"Back"}, // Attach to Greek word field
            },
        }
    }

    // ... existing note creation logic with retry ...
}
```

**Modify `internal/sync/pusher.go`:**

**Add TTS client initialization in `NewPusher`:**
```go
type Pusher struct {
    sheetsClient SheetsClientInterface
    ankiClient   AnkiClientInterface
    ttsClient    *tts.TTSClient  // New field
    config       *models.Config
    logger       *log.Logger
}

func NewPusher(..., ttsClient *tts.TTSClient) *Pusher {
    return &Pusher{
        // ... existing fields ...
        ttsClient: ttsClient,
    }
}
```

**Modify `createNewCards` method:**
```go
func (p *Pusher) createNewCards(cards []*models.VocabCard, dryRun bool) ([]sheets.CellUpdate, error) {
    // ... existing setup ...

    for _, card := range cards {
        if dryRun {
            p.logger.Printf("DRY RUN: Would create card '%s' (%s)", card.English, card.Greek)

            // Check if audio would be generated
            if p.ttsClient != nil && p.config.TextToSpeech != nil && p.config.TextToSpeech.Enabled {
                audioFilename := fmt.Sprintf("%s.mp3", card.Greek)
                exists, _ := p.ankiClient.CheckAudioExists(audioFilename)
                if !exists {
                    p.logger.Printf("DRY RUN: Would generate audio: %s", audioFilename)
                } else {
                    p.logger.Printf("DRY RUN: Audio already exists: %s", audioFilename)
                }
            }
            continue
        }

        // Calculate checksum before creating note
        mapper.UpdateChecksum(card)

        // Generate audio if TTS is enabled
        var audioData []byte
        if p.ttsClient != nil && p.config.TextToSpeech != nil && p.config.TextToSpeech.Enabled {
            audioData = p.generateAudioForCard(card)

            // Add delay to avoid rate limiting
            if p.config.TextToSpeech.RequestDelayMs > 0 {
                time.Sleep(time.Duration(p.config.TextToSpeech.RequestDelayMs) * time.Millisecond)
            }
        }

        // Create note in Anki (with audio if available)
        noteID, err := p.ankiClient.AddNote(
            p.config.AnkiDeck,
            anki.VocabSyncModelName,
            card,
            audioData,  // Pass audio data
        )

        // ... rest of existing logic ...
    }
}

// generateAudioForCard generates audio for a card, handling errors gracefully
func (p *Pusher) generateAudioForCard(card *models.VocabCard) []byte {
    greekText := strings.TrimSpace(card.Greek)

    // Skip if Greek text is empty
    if greekText == "" {
        p.logger.Printf("WARNING: Skipping audio for card '%s' - Greek text is empty", card.English)
        return nil
    }

    audioFilename := fmt.Sprintf("%s.mp3", greekText)

    // Check if audio already exists in Anki
    exists, err := p.ankiClient.CheckAudioExists(audioFilename)
    if err != nil {
        p.logger.Printf("ERROR: Failed to check audio existence for '%s' (row %d): %v", card.English, card.RowNumber, err)
        return nil
    }

    if exists {
        p.logger.Printf("Audio already exists for '%s', skipping generation", greekText)
        return nil
    }

    // Generate audio via TTS
    audioData, err := p.ttsClient.GenerateAudio(greekText)
    if err != nil {
        p.logger.Printf("ERROR: Failed to generate audio for '%s' (row %d): %v", card.English, card.RowNumber, err)
        return nil
    }

    p.logger.Printf("Generated audio for '%s' (%d bytes)", greekText, len(audioData))
    return audioData
}
```

**Modify `cmd/sync/main.go` or CLI initialization:**
```go
// Initialize TTS client if enabled
var ttsClient *tts.TTSClient
if config.TextToSpeech != nil && config.TextToSpeech.Enabled {
    ttsClient, err = tts.NewTTSClient(ctx, config.CredentialsPath, config.TextToSpeech)
    if err != nil {
        return fmt.Errorf("failed to create TTS client: %w", err)
    }
    defer ttsClient.Close()
}

pusher := sync.NewPusher(sheetsClient, ankiClient, ttsClient, config, logger)
```

### Complex Logic

**Audio Generation Decision Tree:**
```
START: New card creation
  ↓
  Is TTS enabled in config?
  ├─ NO → Create card WITHOUT audio
  └─ YES → Continue
       ↓
       Is Greek text empty/whitespace?
       ├─ YES → Log warning, create card WITHOUT audio
       └─ NO → Continue
            ↓
            Check if "{greek}.mp3" exists in Anki media
            ├─ YES → Log "already exists", create card WITHOUT audio
            └─ NO → Continue
                 ↓
                 Call TTS API to generate audio
                 ├─ ERROR → Log error, create card WITHOUT audio
                 └─ SUCCESS → Continue
                      ↓
                      Store audio in Anki media collection
                      ├─ ERROR → Log error, create card WITHOUT audio
                      └─ SUCCESS → Attach audio to note, create card WITH audio
```

### Security Requirements

1. **Authentication**: Use same service account authentication as Google Sheets
   - Service account JSON key file specified in `credentials_path`
   - Required IAM role: `roles/cloudtts.user` (Cloud Text-to-Speech API User)

2. **API Key Protection**: No additional API keys required - service account handles auth

3. **Input Sanitization**:
   - No sanitization needed - Greek text is user-provided vocabulary
   - Validate text is not empty before API call

4. **Data Privacy**:
   - Audio data is ephemeral (generated and immediately sent to Anki)
   - No local storage or caching
   - No PII in audio content (just Greek vocabulary words)

### Performance Requirements

1. **Audio Generation**: Target <500ms per card (network dependent)
2. **Sequential Processing**: Process one card at a time with configurable delay
3. **No Blocking**: Audio generation must not significantly impact sync time
   - If TTS takes >2s per card, consider adding progress indicator
4. **Memory**: Keep audio data in memory only during transfer (no file system writes)
5. **Rate Limiting**:
   - Default 100ms delay between TTS requests
   - Configurable via `request_delay_ms`
   - Handle API rate limit errors gracefully (log and skip)

## Non-Goals (Out of Scope)

1. ❌ Backfilling audio for existing cards without audio
2. ❌ Updating audio when Greek text changes in existing cards
3. ❌ Audio for English words
4. ❌ Local caching of generated audio files
5. ❌ Validating or regenerating existing audio files in Anki
6. ❌ Support for multiple languages beyond Greek
7. ❌ Custom audio files (user-provided recordings)
8. ❌ Audio playback testing or validation
9. ❌ Batch TTS API requests (process sequentially only)
10. ❌ Audio format conversion (MP3 only)
11. ❌ Google Sheet column for audio tracking
12. ❌ CLI command for standalone audio generation
13. ❌ Audio quality selection UI
14. ❌ Progress bar for audio generation

## Testing Requirements

### Unit Tests (80%+ coverage)

**Package: `internal/tts/`**
```go
// client_test.go
- TestNewTTSClient_Success: Verify client creation with valid credentials
- TestNewTTSClient_InvalidCredentials: Verify error handling
- TestNewTTSClient_NilConfig: Verify default config values
- TestGenerateAudio_Success: Mock TTS API, verify audio data returned
- TestGenerateAudio_EmptyText: Verify error on empty input
- TestGenerateAudio_WhitespaceOnly: Verify error on whitespace
- TestGenerateAudio_APIError: Mock API error, verify error propagation
- TestGenerateAudio_ConfigOptions: Verify voice/encoding settings applied
- TestGenerateAudio_SpecialCharacters: Test Greek characters with accents
- TestClose_Success: Verify clean client shutdown
```

**Package: `internal/anki/`**
```go
// client_test.go (add to existing)
- TestCheckAudioExists_FileExists: Mock GetMediaFileNames returning match
- TestCheckAudioExists_FileNotFound: Mock empty result
- TestCheckAudioExists_APIError: Mock error from AnkiConnect
- TestCheckAudioExists_MultipleFiles: Verify exact filename match
- TestStoreAudioFile_Success: Mock StoreMediaFile success
- TestStoreAudioFile_InvalidData: Test error handling
- TestStoreAudioFile_APIError: Mock error from AnkiConnect
- TestAddNote_WithAudio: Verify Audio field populated correctly
- TestAddNote_WithoutAudio: Verify existing behavior unchanged
- TestAddNote_EmptyAudioData: Verify nil audio handling
```

**Package: `internal/sync/`**
```go
// pusher_test.go (add to existing)
- TestGenerateAudioForCard_Success: Mock TTS client, verify audio generated
- TestGenerateAudioForCard_EmptyGreek: Verify warning logged, nil returned
- TestGenerateAudioForCard_AudioExists: Verify skips generation
- TestGenerateAudioForCard_TTSError: Verify error logged, nil returned
- TestGenerateAudioForCard_CheckExistsError: Verify graceful handling
- TestCreateNewCards_WithAudio: Verify audio passed to AddNote
- TestCreateNewCards_TTSDisabled: Verify audio skipped when disabled
- TestCreateNewCards_DryRunWithAudio: Verify dry-run logs audio plans
- TestCreateNewCards_PartialAudioFailure: Verify continues on audio error
```

**Mocking Strategy:**
- Mock `texttospeech.Client` using interfaces
- Mock `AnkiClient` media methods (already has mock in tests)
- Use `testify/mock` for behavior verification

### Integration Tests

**Not required for initial implementation** (per requirement 14A - unit tests only)

However, recommended manual verification steps:
1. Enable TTS in config with test service account
2. Create 2-3 new cards
3. Verify audio files appear in Anki media collection
4. Verify audio plays correctly in Anki
5. Verify existing audio files are not regenerated
6. Test with empty Greek text
7. Test dry-run mode

## Success Metrics

1. ✅ **Unit Test Coverage**: 80%+ coverage for new code in `internal/tts/` and audio-related changes
2. ✅ **All Tests Pass**: CI pipeline green with new tests
3. ✅ **Zero Regression**: Existing sync operations work unchanged when TTS is disabled
4. ✅ **Configuration Works**: TTS can be enabled/disabled via config.yaml
5. ✅ **Graceful Degradation**: Audio failures don't prevent card creation
6. ✅ **Dry-Run Support**: `--dry-run` correctly previews audio generation
7. ✅ **Audio Deduplication**: Existing audio files are detected and not regenerated
8. ✅ **Correct Attachment**: Generated audio plays in Anki on the Greek word field
9. ✅ **Performance**: Audio generation adds <1s per card overhead (on average)
10. ✅ **Documentation**: README updated with TTS setup instructions

## Open Questions

1. **Service Account Permissions**: Confirm the existing service account has `roles/cloudtts.user` role enabled, or document setup steps for users.

2. **API Quota**: What is the expected volume of cards? Google TTS free tier is 1M characters/month. A typical Greek word is ~10 characters, so ~100k cards/month. Should we add quota monitoring?

3. **Voice Quality**: Should we provide recommended voice options in documentation, or let users experiment?

4. **Error Recovery**: If TTS quota is exceeded mid-sync, should we provide a way to resume audio generation later, or require manual retry?

5. **Logging Level**: Should audio generation logs be INFO or DEBUG level? Current plan is INFO for visibility, but could be verbose.

6. **Config Validation**: Should we validate TTS config on startup (test API connection), or fail lazily on first use?
