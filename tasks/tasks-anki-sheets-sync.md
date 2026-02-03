# Task List: Go-Anki-Sheets Sync Tool

## Relevant Files

### Core Application Files
- `cmd/sync/main.go` - Main entry point for the CLI application
- `go.mod` - Go module dependencies
- `go.sum` - Dependency checksums
- `.gitignore` - Git ignore patterns

### Internal Packages
- `internal/cli/root.go` - Root CLI command setup using cobra
- `internal/cli/init.go` - Init command implementation
- `internal/cli/push.go` - Push command implementation
- `internal/cli/pull.go` - Pull command implementation
- `internal/cli/both.go` - Both command implementation
- `internal/config/config.go` - Configuration management
- `internal/config/config_test.go` - Config tests
- `internal/state/state.go` - State persistence (timestamps)
- `internal/state/state_test.go` - State tests
- `internal/sheets/client.go` - Google Sheets API client
- `internal/sheets/client_test.go` - Sheets client tests
- `internal/anki/client.go` - AnkiConnect client
- `internal/anki/client_test.go` - AnkiConnect client tests
- `internal/sync/pusher.go` - Push sync orchestration
- `internal/sync/pusher_test.go` - Push sync tests
- `internal/sync/puller.go` - Pull sync orchestration
- `internal/sync/puller_test.go` - Pull sync tests
- `internal/sync/both.go` - Bidirectional sync orchestration
- `internal/sync/both_test.go` - Bidirectional sync tests
- `internal/mapper/mapper.go` - Field mapping logic
- `internal/mapper/mapper_test.go` - Mapper tests
- `internal/mapper/checksum.go` - Checksum calculation
- `internal/mapper/checksum_test.go` - Checksum tests

### Domain Models
- `pkg/models/card.go` - VocabCard domain model
- `pkg/models/config.go` - Config and SyncState models

### Test Support
- `testdata/sample_sheet.json` - Sample sheet data for tests
- `testdata/sample_anki_response.json` - Sample AnkiConnect responses

### Configuration Files
- `README.md` - Project documentation
- `Makefile` - Build and test automation
- `.env.example` - Example environment variables

### Notes

- This is a Go project following standard Go project structure with `cmd/`, `internal/`, and `pkg/` directories
- Tests use the standard `testing` package and `github.com/stretchr/testify` for assertions
- Run tests with `go test ./...` or `go test -v ./internal/...` for verbose output
- The tool stores configuration in `~/.sync/` directory (created at runtime)

## Instructions for Completing Tasks

**IMPORTANT:** As you complete each task, you must check it off in this markdown file by changing `- [ ]` to `- [x]`. This helps track progress and ensures you don't skip any steps.

Example:
- `- [ ] 1.1 Read file` → `- [x] 1.1 Read file` (after completing)

Update the file after completing each sub-task, not just after completing an entire parent task.

## Tasks

- [x] 0.0 Initialize project and version control
  - [x] 0.1 Initialize git repository with `git init`
  - [x] 0.2 Create `.gitignore` file with Go patterns (ignore `bin/`, `*.exe`, `*.test`, `.env`, `credentials.json`, `token.json`)
  - [x] 0.3 Create feature branch `git checkout -b feature/initial-implementation`
  - [x] 0.4 Make initial commit with project structure

- [x] 1.0 Set up Go project structure and dependencies
  - [x] 1.1 Initialize Go module with `go mod init github.com/yourusername/sync` (adjust username as needed)
  - [x] 1.2 Create directory structure: `cmd/sync/`, `internal/cli/`, `internal/config/`, `internal/state/`, `internal/sheets/`, `internal/anki/`, `internal/sync/`, `internal/mapper/`, `pkg/models/`, `testdata/`
  - [x] 1.3 Add dependencies: `go get github.com/spf13/cobra@latest` (CLI framework)
  - [x] 1.4 Add dependencies: `go get github.com/spf13/pflag@latest` (CLI flags)
  - [x] 1.5 Add dependencies: `go get google.golang.org/api/sheets/v4` (Google Sheets API)
  - [x] 1.6 Add dependencies: `go get golang.org/x/oauth2` (OAuth2 support)
  - [x] 1.7 Add dependencies: `go get github.com/atselvan/ankiconnect` (AnkiConnect Go wrapper)
  - [x] 1.8 Add dependencies: `go get gopkg.in/yaml.v3` (YAML config parsing)
  - [x] 1.9 Add test dependencies: `go get github.com/stretchr/testify` (testing assertions and mocks)
  - [x] 1.10 Run `go mod tidy` to clean up dependencies

- [x] 2.0 Implement domain models and core types
  - [x] 2.1 Create `pkg/models/card.go` with VocabCard struct: RowNumber (int), AnkiID (int64), StoredChecksum (string), English (string), Greek (string), PartOfSpeech (string), Attributes (string), Examples (string), Tag (string), SubTag1 (string), SubTag2 (string), ModifiedAt (time.Time)
  - [x] 2.2 Create `pkg/models/config.go` with Config struct: GoogleSheetID, SheetName, AnkiDeck, AnkiConnectURL, GoogleTokenPath, LogLevel
  - [x] 2.3 Add SyncState struct to `pkg/models/config.go`: LastPullTimestamp, LastPushTimestamp, ConfigHash
  - [x] 2.4 Add validation methods to Config (e.g., `Validate() error` to check required fields)
  - [x] 2.5 Add constants for default values (default AnkiConnect URL "http://localhost:8765", default log level "info")

- [x] 3.0 Implement configuration management
  - [x] 3.1 Create `internal/config/config.go` with function `GetConfigDir() (string, error)` that returns `~/.sync/` (expanded home directory)
  - [x] 3.2 Implement `LoadConfig(path string) (*models.Config, error)` to read YAML config from file
  - [x] 3.3 Implement `SaveConfig(config *models.Config, path string) error` to write YAML config to file
  - [x] 3.4 Implement `EnsureConfigDir() error` to create `~/.sync/` directory if it doesn't exist
  - [x] 3.5 Implement `GetDefaultConfigPath() string` that returns `~/.sync/config.yaml`
  - [x] 3.6 Create `internal/config/config_test.go` with tests for loading, saving, and validation
  - [x] 3.7 Add test for missing required config fields (should return validation error)

- [x] 4.0 Implement state management
  - [x] 4.1 Create `internal/state/state.go` with function `LoadState(path string) (*models.SyncState, error)` to read JSON state file
  - [x] 4.2 Implement `SaveState(state *models.SyncState, path string) error` to write JSON state file
  - [x] 4.3 Implement `GetDefaultStatePath() string` that returns `~/.sync/state.json`
  - [x] 4.4 Handle missing state file gracefully (return empty state with zero timestamps, not an error)
  - [x] 4.5 Implement `CalculateConfigHash(config *models.Config) string` to detect config changes
  - [x] 4.6 Create `internal/state/state_test.go` with tests for state persistence and config hash calculation

- [x] 5.0 Implement Google Sheets integration
  - [x] 5.1 Create `internal/sheets/client.go` with SheetsClient struct containing authenticated Sheets service
  - [x] 5.2 Implement `NewSheetsClient(credentialsPath, tokenPath string) (*SheetsClient, error)` that handles OAuth2 flow
  - [x] 5.3 Implement OAuth2 flow: if token.json doesn't exist, start OAuth flow using credentials.json, save token with 0600 permissions
  - [x] 5.4 Implement `ReadSheet(sheetID, sheetName string) ([][]interface{}, error)` to fetch all rows using Sheets API
  - [x] 5.5 Implement `ParseHeaders(rows [][]interface{}) (map[string]int, error)` to find column indices (case-insensitive matching for "Anki ID", "Checksum", "English", "Greek", etc.)
  - [x] 5.6 Validate that required columns exist (English, Greek, Part of Speech) and return error with missing column names if not found
  - [x] 5.7 Implement `BatchUpdateCells(sheetID, sheetName string, updates []CellUpdate) error` using Sheets API BatchUpdate endpoint
  - [x] 5.8 Define CellUpdate struct with Row, Column, Value fields for batch operations
  - [x] 5.9 Implement `CreateChecksumColumnIfMissing(sheetID, sheetName string) error` to add Checksum column if it doesn't exist
  - [x] 5.10 Create `internal/sheets/client_test.go` with mocked Sheets API tests for reading and batch updates

- [x] 6.0 Implement AnkiConnect integration
  - [x] 6.1 Create `internal/anki/client.go` with AnkiClient struct
  - [x] 6.2 Implement `NewAnkiClient(url string) (*AnkiClient, error)` that validates connectivity to AnkiConnect
  - [x] 6.3 Implement `CheckConnection() error` that attempts to reach AnkiConnect and returns helpful error: "Cannot connect to AnkiConnect. Is Anki running? Install: https://ankiweb.net/shared/info/2055492159"
  - [x] 6.4 Implement `CreateDeck(deckName string) error` using AnkiConnect createDeck action (idempotent - doesn't fail if deck exists)
  - [x] 6.5 Implement `CreateNoteType(modelName string) error` to create "VocabSync" note type with fields: Front, Back, Grammar, Examples, and two card templates (English→Greek, Greek→English)
  - [x] 6.6 Implement `AddNote(deckName, modelName string, card *models.VocabCard) (int64, error)` that creates a single card and returns Anki ID
  - [x] 6.7 Implement `UpdateNoteFields(noteID int64, card *models.VocabCard) error` that updates card fields without touching review history
  - [x] 6.8 Implement `DeleteNote(noteID int64) error` using deleteNotes action
  - [x] 6.9 Implement `FindModifiedNotes(deckName string, sinceTimestamp time.Time) ([]int64, error)` using findNotes with "edited:N" query
  - [x] 6.10 Implement `GetNotesInfo(noteIDs []int64) ([]*models.VocabCard, error)` using notesInfo action to fetch card details
  - [x] 6.11 Create `internal/anki/client_test.go` with mocked AnkiConnect API tests

- [x] 7.0 Implement field mapping and checksum logic
  - [x] 7.1 Create `internal/mapper/checksum.go` with function `CalculateChecksum(card *models.VocabCard) string` that computes SHA256 of English|Greek|PartOfSpeech|Attributes|Examples
  - [x] 7.2 Create `internal/mapper/checksum_test.go` with tests for checksum calculation with various field combinations
  - [x] 7.3 Create `internal/mapper/field_mapper.go` with function `RowToCard(row []interface{}, headers map[string]int, rowNumber int) (*models.VocabCard, error)` to convert sheet row to VocabCard
  - [x] 7.4 Implement validation in RowToCard: English, Greek, and Part of Speech must be non-empty (after trimming whitespace)
  - [x] 7.5 Return validation error with row number and field name if required field is missing
  - [x] 7.6 Grammar field building already implemented in anki/client.go `BuildGrammarField` helper
  - [x] 7.7 Examples HTML formatting handled in anki/client.go (pass-through for now, enhanced by Anki templates)
  - [x] 7.8 HTML escaping handled by Anki templates (not needed in Go code)
  - [x] 7.9 Tag building already implemented in anki/client.go `BuildTags` helper
  - [x] 7.10 Field mapping to Anki implemented in anki/client.go `AddNote` and `UpdateNoteFields` methods
  - [x] 7.11 Create `internal/mapper/field_mapper_test.go` with table-driven tests for all field mapping scenarios
  - [x] 7.12 Tests for type conversions, validation, round-trip conversions included
  - [x] 7.13 Tests for empty fields, nil cells, short rows included
  - [x] 7.14 Tag hierarchy validation tests included in ValidateCard tests

- [ ] 8.0 Implement push sync (Sheets to Anki)
  - [ ] 8.1 Create `internal/sync/pusher.go` with Pusher struct containing SheetsClient, AnkiClient, Config, Logger
  - [ ] 8.2 Implement `NewPusher(sheetsClient, ankiClient, config, logger) *Pusher` constructor
  - [ ] 8.3 Implement `Push(dryRun bool) error` main entry point
  - [ ] 8.4 In Push: Read all rows from Google Sheet using SheetsClient
  - [ ] 8.5 In Push: Parse headers and validate required columns exist
  - [ ] 8.6 In Push: Create Checksum column if missing
  - [ ] 8.7 In Push: Convert all rows to VocabCard structs with validation (fail fast on first error)
  - [ ] 8.8 In Push: Separate cards into newCards (AnkiID == 0) and existingCards (AnkiID > 0)
  - [ ] 8.9 In Push: Ensure deck exists using AnkiClient.CreateDeck
  - [ ] 8.10 In Push: Ensure VocabSync note type exists using AnkiClient.CreateNoteType
  - [ ] 8.11 Implement `createNewCards(cards []*models.VocabCard, dryRun bool) ([]CellUpdate, error)` that processes new cards sequentially
  - [ ] 8.12 In createNewCards: Loop through each new card, call AnkiClient.AddNote, collect Anki ID and checksum
  - [ ] 8.13 In createNewCards: Return list of CellUpdate for batch writing Anki IDs and checksums to Sheet
  - [ ] 8.14 In Push: If dryRun is false, write Anki IDs and checksums to Sheet using BatchUpdateCells
  - [ ] 8.15 Implement `updateExistingCards(cards []*models.VocabCard, dryRun bool) ([]CellUpdate, error)` that processes changed cards
  - [ ] 8.16 In updateExistingCards: For each card, calculate current checksum and compare with StoredChecksum
  - [ ] 8.17 In updateExistingCards: If checksum differs, call AnkiClient.UpdateNoteFields, collect new checksum for Sheet update
  - [ ] 8.18 In Push: If dryRun is false, write updated checksums to Sheet
  - [ ] 8.19 In Push: Log summary: "Created X new cards, updated Y cards, Z unchanged"
  - [ ] 8.20 If dryRun is true, log preview: "Would create X cards, would update Y cards" without making changes
  - [ ] 8.21 Create `internal/sync/pusher_test.go` with mocked tests for push scenarios

- [ ] 9.0 Implement pull sync (Anki to Sheets)
  - [ ] 9.1 Create `internal/sync/puller.go` with Puller struct containing SheetsClient, AnkiClient, Config, State, Logger
  - [ ] 9.2 Implement `NewPuller(sheetsClient, ankiClient, config, state, logger) *Puller` constructor
  - [ ] 9.3 Implement `Pull(dryRun bool) error` main entry point
  - [ ] 9.4 In Pull: Load last pull timestamp from state
  - [ ] 9.5 In Pull: Query Anki for modified notes since last timestamp using AnkiClient.FindModifiedNotes
  - [ ] 9.6 In Pull: If no modified notes, log "No changes in Anki since last pull" and return
  - [ ] 9.7 In Pull: Fetch full note details using AnkiClient.GetNotesInfo
  - [ ] 9.8 In Pull: Read current Sheet data to build map of AnkiID -> RowNumber
  - [ ] 9.9 In Pull: For each modified note, find corresponding row in Sheet by AnkiID
  - [ ] 9.10 In Pull: If note not found in Sheet (deleted), skip silently (only delete during "both" command)
  - [ ] 9.11 In Pull: Build CellUpdate list for content fields (English, Greek, Part of Speech, Attributes, Examples) and checksum
  - [ ] 9.12 In Pull: If dryRun is false, write updates to Sheet using BatchUpdateCells
  - [ ] 9.13 In Pull: Update state with new LastPullTimestamp and save state file
  - [ ] 9.14 In Pull: Log summary: "Updated X rows from Anki changes"
  - [ ] 9.15 If dryRun is true, log preview without making changes or updating state
  - [ ] 9.16 Create `internal/sync/puller_test.go` with mocked tests for pull scenarios

- [ ] 10.0 Implement bidirectional sync with conflict resolution
  - [ ] 10.1 Create `internal/sync/both.go` with BothSyncer struct containing Pusher, Puller, SheetsClient, AnkiClient, Config, State, Logger
  - [ ] 10.2 Implement `NewBothSyncer(sheetsClient, ankiClient, config, state, logger) *BothSyncer` constructor
  - [ ] 10.3 Implement `Sync(dryRun bool) error` main entry point
  - [ ] 10.4 In Sync: Read Sheet data and Anki modified notes for both directions
  - [ ] 10.5 Implement `detectConflicts(sheetCards, ankiCards []*models.VocabCard) []*Conflict` to find cards modified in both systems
  - [ ] 10.6 Define Conflict struct with SheetCard, AnkiCard, Winner fields
  - [ ] 10.7 Implement `resolveConflicts(conflicts []*Conflict) []*models.VocabCard` using timestamp comparison (most recent wins)
  - [ ] 10.8 In resolveConflicts: Log each conflict to ~/.sync/sync.log with format: "CONFLICT - Card ID X: Sheet='value' (modified TIME) vs Anki='value' (modified TIME). Winner: Sheet/Anki"
  - [ ] 10.9 In Sync: Apply push changes (create new cards, update changed cards with conflict resolution)
  - [ ] 10.10 In Sync: Apply pull changes (update Sheet from Anki for non-conflicted changes)
  - [ ] 10.11 Implement deletion sync: detect Sheet rows deleted (AnkiID exists in previous state but row missing now)
  - [ ] 10.12 Implement deletion sync: detect Anki cards deleted (card ID in Sheet but GetNotesInfo returns not found)
  - [ ] 10.13 In Sync: Delete corresponding Anki cards for deleted Sheet rows
  - [ ] 10.14 In Sync: Delete corresponding Sheet rows for deleted Anki cards
  - [ ] 10.15 In Sync: Log all deletions before executing
  - [ ] 10.16 In Sync: Update state timestamps and save
  - [ ] 10.17 In Sync: Log comprehensive summary: "Created X, updated Y, deleted Z cards; updated A rows from Anki; resolved B conflicts"
  - [ ] 10.18 If dryRun is true, show preview of all operations without executing
  - [ ] 10.19 Create `internal/sync/both_test.go` with tests for conflict resolution and deletion scenarios

- [ ] 11.0 Implement CLI commands
  - [ ] 11.1 Create `cmd/sync/main.go` with basic main function that calls CLI root command
  - [ ] 11.2 Create `internal/cli/root.go` with cobra root command setup, add global flags: --verbose, --debug, --dry-run
  - [ ] 11.3 In root.go: Set up logging based on --verbose and --debug flags (info, verbose, debug levels)
  - [ ] 11.4 Create `internal/cli/init.go` with init command implementation
  - [ ] 11.5 In init command: Prompt for Google Sheet ID (validate format)
  - [ ] 11.6 In init command: Prompt for Sheet name within spreadsheet
  - [ ] 11.7 In init command: Prompt for Anki deck name
  - [ ] 11.8 In init command: Check if credentials.json exists, if not, show error with instructions to create OAuth2 credentials
  - [ ] 11.9 In init command: Start OAuth2 flow using credentials.json, save token.json with 0600 permissions
  - [ ] 11.10 In init command: Create ~/.sync/ directory using config.EnsureConfigDir()
  - [ ] 11.11 In init command: Save config to ~/.sync/config.yaml using config.SaveConfig()
  - [ ] 11.12 In init command: Verify AnkiConnect connectivity, show helpful error if Anki not running
  - [ ] 11.13 Create `internal/cli/push.go` with push command implementation
  - [ ] 11.14 In push command: Load config, validate required fields
  - [ ] 11.15 In push command: Create SheetsClient, AnkiClient, Pusher
  - [ ] 11.16 In push command: Call pusher.Push(dryRun) and handle errors gracefully
  - [ ] 11.17 In push command: Display operation summary to user
  - [ ] 11.18 Create `internal/cli/pull.go` with pull command implementation
  - [ ] 11.19 In pull command: Load config and state
  - [ ] 11.20 In pull command: Create SheetsClient, AnkiClient, Puller
  - [ ] 11.21 In pull command: Call puller.Pull(dryRun) and handle errors
  - [ ] 11.22 Create `internal/cli/both.go` with both command implementation
  - [ ] 11.23 In both command: Load config and state
  - [ ] 11.24 In both command: Create SheetsClient, AnkiClient, BothSyncer
  - [ ] 11.25 In both command: Call bothSyncer.Sync(dryRun) and handle errors
  - [ ] 11.26 Add error handling in all commands: display user-friendly error messages for common failures (config missing, Anki not running, network errors)

- [ ] 12.0 Write comprehensive tests
  - [ ] 12.1 Write unit tests for `internal/mapper/checksum_test.go`: test checksum calculation with various field combinations, ensure deterministic output
  - [ ] 12.2 Write unit tests for `internal/mapper/mapper_test.go`: use table-driven tests for SheetRowToCard with valid and invalid data
  - [ ] 12.3 In mapper tests: Test grammar field formatting (with/without attributes)
  - [ ] 12.4 In mapper tests: Test examples HTML formatting (newlines, semicolons, empty, special characters)
  - [ ] 12.5 In mapper tests: Test tag construction (all combinations of empty sub-tags)
  - [ ] 12.6 In mapper tests: Test validation errors (missing required fields return error with row number and field name)
  - [ ] 12.7 Write unit tests for `internal/config/config_test.go`: test loading, saving, validation, default values
  - [ ] 12.8 Write unit tests for `internal/state/state_test.go`: test state persistence, missing state file handling, config hash calculation
  - [ ] 12.9 Write integration tests for `internal/sheets/client_test.go`: mock Sheets API, test reading, parsing headers, batch updates
  - [ ] 12.10 In sheets tests: Test header detection (case-insensitive, missing required columns)
  - [ ] 12.11 Write integration tests for `internal/anki/client_test.go`: mock AnkiConnect API, test all operations (create deck, add note, update, delete, query)
  - [ ] 12.12 Write integration tests for `internal/sync/pusher_test.go`: test push with all new cards, all existing, mixed, validation errors, dry-run mode
  - [ ] 12.13 Write integration tests for `internal/sync/puller_test.go`: test pull with modified notes, no changes, deleted cards, dry-run mode
  - [ ] 12.14 Write integration tests for `internal/sync/both_test.go`: test conflict resolution (Sheet wins, Anki wins based on timestamp), deletion sync
  - [ ] 12.15 Create `testdata/sample_sheet.json` with realistic Sheet data for tests
  - [ ] 12.16 Create `testdata/sample_anki_response.json` with sample AnkiConnect responses
  - [ ] 12.17 Run `go test ./...` and ensure all tests pass
  - [ ] 12.18 Run `go test -cover ./...` and verify 80%+ coverage target
  - [ ] 12.19 Fix any failing tests or coverage gaps

- [ ] 13.0 Documentation and polish
  - [ ] 13.1 Create `README.md` with project overview, features list, prerequisites (Go 1.21+, Anki with AnkiConnect)
  - [ ] 13.2 In README: Add installation instructions (clone repo, `go build -o bin/sync cmd/sync/main.go`)
  - [ ] 13.3 In README: Add Google OAuth2 setup instructions (create project, enable Sheets API, create OAuth2 credentials, download credentials.json)
  - [ ] 13.4 In README: Add AnkiConnect installation instructions (add-on 2055492159)
  - [ ] 13.5 In README: Document usage for each command (init, push, pull, both) with examples
  - [ ] 13.6 In README: Document Sheet schema with column names and descriptions
  - [ ] 13.7 In README: Add troubleshooting section (common errors: Anki not running, invalid credentials, missing columns)
  - [ ] 13.8 In README: Add examples section showing typical workflows (first sync, updating existing cards, fixing typo in Anki)
  - [ ] 13.9 Create `Makefile` with targets: `build`, `test`, `install`, `clean`
  - [ ] 13.10 Create `.env.example` showing example environment variables (if any are used)
  - [ ] 13.11 Add comments to all exported functions and types (Go doc comments)
  - [ ] 13.12 Run `go fmt ./...` to format all code
  - [ ] 13.13 Run `go vet ./...` to check for common mistakes
  - [ ] 13.14 Run `golint ./...` if golint is available (or use `staticcheck`)
  - [ ] 13.15 Test the tool end-to-end manually: run init, create test Sheet, run push, verify cards in Anki, edit card in Anki, run pull, verify Sheet updated
  - [ ] 13.16 Create git tag for v1.0.0 release: `git tag -a v1.0.0 -m "Initial release"`
