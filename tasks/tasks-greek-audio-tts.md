# Task List: Greek Audio Generation with Text-to-Speech

## Relevant Files

### New Files
- `internal/tts/client.go` - TTS client for Google Cloud Text-to-Speech API integration
- `internal/tts/client_test.go` - Unit tests for TTS client

### Modified Files
- `pkg/models/config.go` - Add TTSConfig struct for TTS configuration
- `internal/anki/client.go` - Add CheckAudioExists and StoreAudioFile methods; modify AddNote signature
- `internal/anki/client_test.go` - Add tests for new audio methods
- `internal/sync/pusher.go` - Add TTS client field; modify createNewCards to generate audio
- `internal/sync/pusher_test.go` - Add tests for audio generation in pusher
- `cmd/sync/main.go` - Initialize TTS client and pass to pusher
- `go.mod` - Add cloud.google.com/go/texttospeech dependency
- `config.yaml.example` - Add text_to_speech configuration example (if example file exists)
- `README.md` - Document TTS setup and configuration

### Notes

- Unit tests should use mocked TTS client to avoid real API calls
- Follow existing retry pattern from `internal/anki/retry.go` for error handling
- Use same service account authentication pattern as Google Sheets integration
- Audio generation failures should be logged but never block card creation

## Instructions for Completing Tasks

**IMPORTANT:** As you complete each task, you must check it off in this markdown file by changing `- [ ]` to `- [x]`. This helps track progress and ensures you don't skip any steps.

Example:
- `- [ ] 1.1 Read file` → `- [x] 1.1 Read file` (after completing)

Update the file after completing each sub-task, not just after completing an entire parent task.

## Tasks

- [x] 0.0 Create feature branch
  - [x] 0.1 Create and checkout a new branch `feature/greek-audio-tts` from main

- [x] 1.0 Add TTS configuration to data models
  - [x] 1.1 Read `pkg/models/config.go` to understand existing Config structure
  - [x] 1.2 Add `TTSConfig` struct with fields: Enabled (bool), VoiceName (string), AudioEncoding (string), SpeakingRate (float64), Pitch (float64), VolumeGainDb (float64), RequestDelayMs (int)
  - [x] 1.3 Add `TextToSpeech *TTSConfig` field to Config struct with yaml tag `text_to_speech,omitempty`
  - [x] 1.4 Verify the code compiles: `go build ./pkg/models/...`

- [x] 2.0 Create TTS client package
  - [x] 2.1 Create directory `internal/tts/`
  - [x] 2.2 Create `internal/tts/client.go` file
  - [x] 2.3 Add package declaration and imports (context, fmt, cloud.google.com/go/texttospeech/apiv1, texttospeechpb, google.golang.org/api/option, models)
  - [x] 2.4 Define `TTSClient` struct with fields: client (*texttospeech.Client), config (*models.TTSConfig)
  - [x] 2.5 Implement `NewTTSClient(ctx context.Context, credentialsPath string, config *models.TTSConfig) (*TTSClient, error)` - creates client using service account, returns error if credentials invalid
  - [x] 2.6 Implement `GenerateAudio(greekText string) ([]byte, error)` - validates input (error if empty/whitespace), builds SynthesizeSpeechRequest with config settings (voice: el-GR + VoiceName, audio config with MP3/speaking rate/pitch/volume), calls client.SynthesizeSpeech, returns audio bytes
  - [x] 2.7 Implement `Close() error` - closes TTS client connection
  - [x] 2.8 Add helper function `validateGreekText(text string) error` to check for empty/whitespace
  - [x] 2.9 Verify the code compiles: `go build ./internal/tts/...`

- [x] 3.0 Extend Anki client for audio operations
  - [x] 3.1 Read `internal/anki/client.go` to understand existing structure and patterns
  - [x] 3.2 Add import for "encoding/base64" at top of file
  - [x] 3.3 Implement `CheckAudioExists(filename string) (bool, error)` method - calls c.client.Media.GetMediaFileNames(filename), handles RestErr type, returns true if exact filename match found in results
  - [x] 3.4 Implement `StoreAudioFile(filename string, audioData []byte) error` method - base64 encodes audioData, calls c.client.Media.StoreMediaFile with encoded data, wraps errors with context
  - [x] 3.5 Modify `AddNote` method signature from `func (c *AnkiClient) AddNote(deckName, modelName string, card *models.VocabCard) (int64, error)` to include audioData parameter: `func (c *AnkiClient) AddNote(deckName, modelName string, card *models.VocabCard, audioData []byte) (int64, error)`
  - [x] 3.6 In `AddNote` method, after building the note struct, add logic: if audioData is not nil and len > 0, create filename as `fmt.Sprintf("%s.mp3", card.Greek)`, base64 encode audioData, populate note.Audio field with []ankiconnect.Audio containing one Audio struct with Data (encoded), Filename, and Fields ([]string{"Back"})
  - [x] 3.7 Verify the code compiles: `go build ./internal/anki/...`

- [x] 4.0 Integrate audio generation into sync workflow
  - [x] 4.1 Read `internal/sync/pusher.go` to understand existing pusher structure
  - [x] 4.2 Import "time" and "strings" packages if not already imported
  - [x] 4.3 Add `ttsClient *tts.TTSClient` field to Pusher struct
  - [x] 4.4 Update `NewPusher` function to accept ttsClient parameter and assign to struct
  - [x] 4.5 Implement helper method `generateAudioForCard(card *models.VocabCard) []byte` that: (a) trims and validates Greek text (return nil if empty with warning log), (b) builds filename as "{greek}.mp3", (c) calls ankiClient.CheckAudioExists (return nil on error or if exists), (d) calls ttsClient.GenerateAudio (return nil on error), (e) logs success and returns audio bytes. All errors should be logged with ERROR prefix but return nil (graceful degradation)
  - [x] 4.6 In `createNewCards` method, before the loop add check: if p.ttsClient is nil or config.TextToSpeech is nil or not enabled, skip all audio logic
  - [x] 4.7 In `createNewCards` method dry-run block, add logic to check if audio would be generated: call CheckAudioExists, log "Would generate audio: {filename}" if not exists, or "Audio already exists: {filename}" if exists
  - [x] 4.8 In `createNewCards` method after checksum update, add: `var audioData []byte`, then if TTS enabled call `audioData = p.generateAudioForCard(card)`, then if RequestDelayMs > 0 sleep for that duration
  - [x] 4.9 Update `ankiClient.AddNote` call to pass audioData as 4th argument
  - [x] 4.10 Read `cmd/sync/main.go` (or wherever CLI initialization happens) to understand initialization pattern
  - [x] 4.11 In CLI initialization, after config is loaded and before pusher creation, add: initialize ttsClient variable as nil, check if config.TextToSpeech is not nil and Enabled is true, if so call tts.NewTTSClient with context and config, handle errors, add defer ttsClient.Close()
  - [x] 4.12 Update `sync.NewPusher` call to pass ttsClient as additional parameter
  - [x] 4.13 Verify the code compiles: `go build ./...`

- [x] 5.0 Add comprehensive unit tests
  - [x] 5.1 Create `internal/tts/client_test.go` with package declaration and imports (testing, testify, mocks if needed)
  - [x] 5.2 Implement `TestNewTTSClient_Success` - test successful client creation with valid config (may need to mock or skip if requires real credentials)
  - [x] 5.3 Implement `TestGenerateAudio_EmptyText` - verify error returned for empty string
  - [x] 5.4 Implement `TestGenerateAudio_WhitespaceOnly` - verify error returned for whitespace-only string
  - [x] 5.5 Implement mock TTS client interface or use test doubles for remaining tests
  - [x] 5.6 Implement `TestGenerateAudio_Success` - mock TTS API, verify audio bytes returned
  - [x] 5.7 Implement `TestGenerateAudio_APIError` - mock API error, verify error propagated
  - [x] 5.8 Run TTS tests: `go test ./internal/tts/... -v`
  - [x] 5.9 Read `internal/anki/client_test.go` to understand existing test patterns and mock setup
  - [x] 5.10 Add `TestCheckAudioExists_FileExists` - mock GetMediaFileNames returning matching filename, verify true returned
  - [x] 5.11 Add `TestCheckAudioExists_FileNotFound` - mock empty result, verify false returned
  - [x] 5.12 Add `TestCheckAudioExists_APIError` - mock error from AnkiConnect, verify error propagated
  - [x] 5.13 Add `TestStoreAudioFile_Success` - mock successful StoreMediaFile call
  - [x] 5.14 Add `TestAddNote_WithAudio` - verify note.Audio field populated correctly when audioData provided
  - [x] 5.15 Add `TestAddNote_WithoutAudio` - verify existing behavior unchanged when audioData is nil
  - [x] 5.16 Run Anki tests: `go test ./internal/anki/... -v`
  - [x] 5.17 Read `internal/sync/pusher_test.go` to understand existing test patterns
  - [x] 5.18 Update mock structs if needed to support TTS client mocking
  - [x] 5.19 Add `TestGenerateAudioForCard_Success` - mock successful audio generation
  - [x] 5.20 Add `TestGenerateAudioForCard_EmptyGreek` - verify nil returned with warning for empty Greek
  - [x] 5.21 Add `TestGenerateAudioForCard_AudioExists` - mock CheckAudioExists returning true, verify generation skipped
  - [x] 5.22 Add `TestGenerateAudioForCard_TTSError` - mock TTS error, verify nil returned (graceful)
  - [x] 5.23 Add `TestCreateNewCards_WithAudio` - verify audio passed to AddNote when TTS enabled
  - [x] 5.24 Add `TestCreateNewCards_TTSDisabled` - verify audio skipped when config.TextToSpeech.Enabled is false
  - [x] 5.25 Add `TestCreateNewCards_DryRunWithAudio` - verify dry-run logs audio generation plans
  - [x] 5.26 Run sync tests: `go test ./internal/sync/... -v`
  - [x] 5.27 Run all tests: `go test ./... -v` and verify all pass

- [x] 6.0 Update documentation and dependencies
  - [x] 6.1 Run `go get cloud.google.com/go/texttospeech/apiv1` to add TTS dependency
  - [x] 6.2 Run `go mod tidy` to clean up dependencies
  - [x] 6.3 Check if `config.yaml.example` exists, if so add text_to_speech section with default values (enabled: true, voice_name: "el-GR-Wavenet-A", audio_encoding: "MP3", speaking_rate: 1.0, pitch: 0.0, volume_gain_db: 0.0, request_delay_ms: 100)
  - [x] 6.4 Read `README.md` to understand documentation structure
  - [x] 6.5 Add new section "Greek Audio Generation (Text-to-Speech)" in README with: (a) Overview of feature, (b) Requirements (service account needs roles/cloudtts.user), (c) Configuration example from config.yaml, (d) How to enable/disable TTS, (e) Supported voices and settings, (f) Troubleshooting common issues
  - [x] 6.6 Verify documentation is clear and includes setup steps for enabling TTS API in Google Cloud Console
  - [x] 6.7 Final verification: run `go build ./cmd/sync` to ensure binary compiles
  - [x] 6.8 Final test run: `go test ./...` to ensure all tests pass
