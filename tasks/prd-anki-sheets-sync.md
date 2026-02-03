# Product Requirements Document: Go-Anki-Sheets Sync Tool

## 1. Introduction/Overview

A CLI tool written in Go that provides two-way synchronization between Google Sheets (vocabulary database) and Anki (spaced repetition study app). The tool maintains data integrity by preserving Anki card review history while allowing bulk updates and corrections in either system. The sync is controlled through explicit commands (`push`, `pull`, `both`) giving users precise control over sync direction.

The tool solves the problem of maintaining vocabulary lists in a spreadsheet for bulk editing while leveraging Anki's spaced repetition algorithm for effective learning. Users can add hundreds of words in a Sheet, sync them to Anki for study, and correct typos in either location without losing review progress.

## 2. Goals

### Functional Goals
1. **Bulk Vocabulary Sync:** Push hundreds of new vocabulary words from Sheets to Anki in a single sync operation
2. **Bidirectional Correction:** Edit a typo during an Anki study session and sync it back to the Sheet automatically
3. **History Preservation:** Update card content without resetting Anki's spaced repetition intervals or review statistics
4. **Automated Organization:** Support hierarchical tag structure (e.g., `City::Buildings::Museums`) derived from Sheet columns
5. **Data Integrity:** Fail-fast validation ensures only complete, valid vocabulary entries are synced
6. **Zero Manual Mapping:** Auto-create Anki note type with correct fields on first run; no manual Anki setup required

### Technical Goals
- Reliable sync for typical vocabulary lists (100-500 words)
- Idempotent operations (running sync multiple times produces same result)
- 80%+ test coverage with unit and integration tests
- Clear error messages with actionable remediation steps
- Simplicity and correctness prioritized over raw performance

## 3. Tech Stack & Architecture

### Language
- Go 1.21+

### External Dependencies
- **Google Sheets API:** `google.golang.org/api/sheets/v4` - Official Google API client for Go
- **Google Auth:** `golang.org/x/oauth2` - OAuth2 flow and token management
- **AnkiConnect Integration:** `github.com/atselvan/ankiconnect` - Go wrapper for AnkiConnect JSON-RPC API
- **Testing:** `github.com/stretchr/testify` - Assertions and mocking framework
- **YAML Config:** `gopkg.in/yaml.v3` - Configuration file parsing

### Architectural Pattern

**Layered architecture** with clear separation of concerns:

1. **CLI Layer:** Command parsing, flags, user interaction (cobra/pflag)
2. **Service Layer:** Sync orchestration, conflict resolution, change detection
3. **Repository Layer:** Google Sheets client, AnkiConnect client
4. **Domain Layer:** Core data models (VocabCard, SyncState, FieldMapping)

### Key Architectural Decisions

- **Anki ID as Primary Key:** 13-digit Anki Note ID stored in Sheet is the single source of truth for matching cards to rows
- **Checksum-Based Change Detection:** SHA256 hash of content fields (English + Greek + Part of Speech + Attributes + Examples) stored in Sheet column determines if update is needed
- **Checksum Storage:** Checksums stored directly in Google Sheet as additional column for durability and simplicity
- **Stateful Sync:** Last sync timestamp persisted in `~/.sync/state.json` for efficient pull operations
- **Sequential AnkiConnect Operations:** Process Anki cards one at a time (create/update) for simplicity and reliability
- **Batch Google Sheets Operations:** Use BatchUpdate for all Sheet write operations (Anki IDs, checksums, content updates)

### External Services/APIs

- **AnkiConnect:** Requires AnkiConnect add-on installed in Anki desktop. Must be running on `localhost:8765`
- **Google Sheets API:** Requires OAuth2 credentials and user authorization

### Data Storage

- **Configuration:** YAML file at `~/.sync/config.yaml`
- **OAuth Tokens:** JSON file at `~/.sync/token.json` (0600 permissions)
- **State:** JSON file at `~/.sync/state.json` for sync timestamps

### Existing Patterns to Follow

This is a new project with no existing codebase. Follow standard Go project structure:
```
sync/
├── cmd/
│   └── sync/
│       └── main.go
├── internal/
│   ├── cli/          # Command definitions
│   ├── config/       # Config management
│   ├── sync/         # Sync orchestration
│   ├── sheets/       # Google Sheets client
│   ├── anki/         # AnkiConnect client
│   ├── mapper/       # Field mapping logic
│   └── state/        # State persistence
├── pkg/
│   └── models/       # Shared data models
└── testdata/         # Test fixtures
```

## 4. Functional Requirements

### FR1. CLI Commands

The tool provides the following commands:

- `sync init` - Interactive setup wizard: authenticate with Google, specify Sheet ID and sheet name, specify Anki deck name
- `sync push` - Sync from Google Sheets to Anki (create new cards, update existing cards)
- `sync pull` - Sync from Anki to Google Sheets (update Sheet rows with Anki changes)
- `sync both` - Bidirectional sync with timestamp-based conflict resolution

All commands support the following flags:
- `--dry-run` - Preview changes without applying them
- `--verbose` - Show detailed operation logs
- `--debug` - Show API requests, responses, and internal state

### FR2. Data Validation

- **Fail-fast validation:** Stop sync immediately on first validation error
- **Required fields:** English, Greek, Part of Speech (must be non-empty after trimming whitespace)
- **Optional fields:** Attributes, Examples, Tag, Sub-Tag 1, Sub-Tag 2, Anki ID
- **Error messages:** Display row number and field name when validation fails
- **No partial syncs:** Either all valid data syncs or nothing syncs

### FR3. Card Creation (Push - New Cards)

When Anki ID column is empty:
1. Create new Anki note in configured deck (one card at a time)
2. AnkiConnect returns generated Anki ID
3. Calculate and store checksum for the card
4. Collect all generated IDs and checksums
5. Write all IDs and checksums back to Sheet in single BatchUpdate operation
6. Use auto-generated "VocabSync" note type
7. Map Sheet columns to Anki fields per FR8
8. Apply hierarchical tags per FR9

### FR4. Card Updates (Push - Existing Cards)

When Anki ID exists:
1. Calculate checksum of content fields (English, Greek, Part of Speech, Attributes, Examples)
2. Compare with stored checksum in Sheet column
3. If checksum differs, update card in Anki using AnkiConnect `updateNoteFields` (one card at a time)
4. Write new checksum back to Sheet using BatchUpdate
5. **Never modify:** Anki's review history, intervals, ease factor, or due dates
6. Update tags if tag columns changed

### FR5. Sheet Updates (Pull)

1. Query Anki for notes modified since last successful pull timestamp
2. Match notes to Sheet rows using Anki ID
3. Update Sheet cells with modified content from Anki using BatchUpdate
4. Update fields: English, Greek, Part of Speech, Attributes, Examples
5. **Do not sync tags back:** Sheet is source of truth for tags

### FR6. Conflict Resolution (Both)

When the same card is modified in both systems:
1. Compare modification timestamps
2. Most recent change wins automatically
3. Log all conflicts to `~/.sync/sync.log` with details: card ID, field name, both values, winner
4. Continue processing remaining cards after conflict resolution

### FR7. Deletion Handling

Applies only during `sync both` command:
- **Sheet row deleted** (Anki ID exists but row missing): delete corresponding Anki card
- **Anki card deleted** (card ID in Sheet but not found in Anki): delete Sheet row
- Log all deletions with card/row details before executing

### FR8. Field Mapping

**Grammar Field Construction:**
```
if Attributes is empty:
    Grammar = PartOfSpeech
else:
    Grammar = PartOfSpeech + " (" + Attributes + ")"

Examples: "Noun (Masculine)", "Verb (Class 1)", "Adjective"
```

**Examples HTML Formatting:**
```
Split by newline or semicolon
Wrap each example in <div> tag
Result: "<div>Example 1</div><div>Example 2</div>"
Empty Examples field remains empty string
```

**Empty Field Handling:**
- Empty optional fields remain empty (no default values or placeholders)
- Grammar field with empty Attributes: just "Noun" (no parentheses)
- Tags with empty sub-levels: collapse hierarchy (e.g., "City::Transportation" not "City::::Transportation")

### FR9. Tag Construction

Build hierarchical tags from Tag, Sub-Tag 1, Sub-Tag 2 columns:
1. Skip empty tag levels
2. Join non-empty levels with `::`
3. Examples:
   - Tag="City", Sub-Tag 1="Buildings", Sub-Tag 2="" → `City::Buildings`
   - Tag="Food", Sub-Tag 1="", Sub-Tag 2="" → `Food`
   - Tag="City", Sub-Tag 1="", Sub-Tag 2="Transportation" → `City::Transportation`

### FR10. Initial Sync Behavior

When user first runs `sync push` with a Sheet full of vocabulary but no Anki IDs:
1. Treat all rows without Anki ID as new cards
2. Create cards sequentially (one at a time) in Anki
3. Collect all generated Anki IDs and calculated checksums
4. Write all IDs and checksums back to Sheet in single BatchUpdate
5. Log: "Initial sync: Created N new cards"

## 5. Technical Specifications

### 5.1 Data Models

```go
// Core domain model
type VocabCard struct {
    RowNumber      int       // Sheet row number (for updates)
    AnkiID         int64     // 13-digit Anki note ID (0 if not yet created)
    StoredChecksum string    // Checksum from Sheet column (for comparison)
    English        string    // Required
    Greek          string    // Required
    PartOfSpeech   string    // Required (Noun, Verb, Adjective, etc.)
    Attributes     string    // Optional (Gender for nouns, Class for verbs)
    Examples       string    // Optional (raw text, converted to HTML)
    Tag            string    // Optional (top-level category)
    SubTag1        string    // Optional (second-level category)
    SubTag2        string    // Optional (third-level category)
    ModifiedAt     time.Time // Last modification timestamp
}

// Sync state persistence
type SyncState struct {
    LastPullTimestamp time.Time
    LastPushTimestamp time.Time
    ConfigHash        string // Detect config changes
}

// Configuration
type Config struct {
    GoogleSheetID    string
    SheetName        string
    AnkiDeck         string
    AnkiConnectURL   string // Default: "http://localhost:8765"
    GoogleTokenPath  string // Default: "~/.sync/token.json"
    LogLevel         string // "info", "verbose", "debug"
}
```

### 5.2 Google Sheet Schema

Expected column headers (row 1, case-insensitive matching):

| Column | Type | Required | Description |
|--------|------|----------|-------------|
| Anki ID | integer | No | 13-digit Anki note ID (auto-filled by tool) |
| Checksum | string | No | SHA256 hash of content fields (auto-filled by tool) |
| English | string | Yes | Primary English word(s) |
| Greek | string | Yes | Primary Greek word(s) |
| Part of Speech | string | Yes | Noun, Verb, Adjective, etc. |
| Attributes | string | No | Gender (Masculine/Feminine/Neuter) or Verb Class |
| Examples | string | No | Usage examples (one per line or semicolon-separated) |
| Tag | string | No | Top-level category |
| Sub-Tag 1 | string | No | Second-level category |
| Sub-Tag 2 | string | No | Third-level category |

**Header Detection:**
- Tool expects these exact column names in row 1
- Case-insensitive matching ("english" = "English" = "ENGLISH")
- Fail with error if required columns missing
- Checksum column is optional; tool will create it if missing

### 5.3 Anki Note Type: "VocabSync"

Auto-created on first `sync push` if it doesn't exist.

**Fields:**
- **Front:** English word
- **Back:** Greek translation
- **Grammar:** Combined Part of Speech + Attributes
- **Examples:** HTML-formatted examples

**Card Templates (two cards per note):**

1. **Card 1 (English → Greek):**
   - Front: `{{Front}}`
   - Back: `{{Back}}<br><br>{{Grammar}}<br><br>{{Examples}}`

2. **Card 2 (Greek → English):**
   - Front: `{{Back}}`
   - Back: `{{Front}}<br><br>{{Grammar}}<br><br>{{Examples}}`

Both cards allow bidirectional practice: English→Greek and Greek→English.

### 5.4 Checksum Calculation

```go
func CalculateChecksum(card VocabCard) string {
    content := card.English + "|" +
               card.Greek + "|" +
               card.PartOfSpeech + "|" +
               card.Attributes + "|" +
               card.Examples
    hash := sha256.Sum256([]byte(content))
    return hex.EncodeToString(hash[:])
}
```

**Included in checksum:** English, Greek, Part of Speech, Attributes, Examples

**Excluded from checksum:** Anki ID, Tags (tags can change independently without triggering content update)

### 5.5 API Contracts

#### Google Sheets API

**Read Operation:**
```http
GET /v4/spreadsheets/{sheetId}/values/{sheetName}!A1:Z
```

**Batch Write (Anki IDs):**
```http
POST /v4/spreadsheets/{sheetId}/values:batchUpdate

Body:
{
  "valueInputOption": "USER_ENTERED",
  "data": [
    {
      "range": "Vocabulary!A2:A100",
      "values": [[ankiID1], [ankiID2], ...]
    }
  ]
}
```

**Batch Update (Pull changes):**
```http
POST /v4/spreadsheets/{sheetId}/values:batchUpdate

Body:
{
  "valueInputOption": "USER_ENTERED",
  "data": [
    {
      "range": "Vocabulary!B5:F5",
      "values": [[english, greek, pos, attr, examples]]
    }
  ]
}
```

#### AnkiConnect API

**Create Note:**
```json
{
  "action": "addNote",
  "params": {
    "note": {
      "deckName": "Greek Vocabulary",
      "modelName": "VocabSync",
      "fields": {
        "Front": "hello",
        "Back": "γεια σου",
        "Grammar": "Interjection",
        "Examples": "<div>Hello, how are you?</div>"
      },
      "tags": ["Greetings", "City::Basics"]
    }
  }
}

Response: { "result": 1234567890123, "error": null }
```

**Update Note:**
```json
{
  "action": "updateNoteFields",
  "params": {
    "note": {
      "id": 1234567890123,
      "fields": {
        "Front": "hello",
        "Back": "γεια σου"
      }
    }
  }
}
```

**Query Modified Notes:**
```json
// Step 1: Find modified note IDs
{
  "action": "findNotes",
  "params": {
    "query": "deck:\"Greek Vocabulary\" edited:7"
  }
}
Response: { "result": [1234567890123, 1234567890124], "error": null }

// Step 2: Fetch note details
{
  "action": "notesInfo",
  "params": {
    "notes": [1234567890123, 1234567890124]
  }
}
```

**Create Note Type:**
```json
{
  "action": "createModel",
  "params": {
    "modelName": "VocabSync",
    "inOrderFields": ["Front", "Back", "Grammar", "Examples"],
    "css": ".card { font-family: arial; font-size: 20px; text-align: center; }",
    "cardTemplates": [
      {
        "Name": "English to Greek",
        "Front": "{{Front}}",
        "Back": "{{Back}}<br><br>{{Grammar}}<br><br>{{Examples}}"
      },
      {
        "Name": "Greek to English",
        "Front": "{{Back}}",
        "Back": "{{Front}}<br><br>{{Grammar}}<br><br>{{Examples}}"
      }
    ]
  }
}
```

**Delete Note:**
```json
{
  "action": "deleteNotes",
  "params": {
    "notes": [1234567890123]
  }
}
```

### 5.6 Security Requirements

- OAuth2 tokens stored in `~/.sync/token.json` with 0600 permissions (user read/write only)
- `credentials.json` (OAuth client secret) should not be committed to version control
- AnkiConnect operates on localhost only (no network exposure)
- No sensitive data logged even in debug mode (tokens, credentials redacted)
- Config file can contain deck names and Sheet IDs (not sensitive)

### 5.7 Performance Requirements

- Initial sync of 500 cards: No strict time requirement (sequential processing is acceptable)
- Incremental sync (10 changed cards): < 10 seconds
- Sheet read operation: single API call (batch read entire sheet)
- Sheet write operations: batched using BatchUpdate (typically 1-2 API calls per sync)
- AnkiConnect operations: sequential, one card at a time for both create and update
- Checksum calculation: O(n) where n = number of rows, negligible overhead
- HTTP timeout: 30 seconds for all API calls
- Performance is not a primary concern; correctness and simplicity are prioritized

### 5.8 Complex Logic: Conflict Resolution

```go
func ResolveConflict(sheetCard, ankiCard VocabCard) VocabCard {
    // Compare modification timestamps
    if ankiCard.ModifiedAt.After(sheetCard.ModifiedAt) {
        // Anki wins
        LogConflict(sheetCard, ankiCard, "Anki")
        return ankiCard
    } else {
        // Sheet wins (including equal timestamps)
        LogConflict(sheetCard, ankiCard, "Sheet")
        return sheetCard
    }
}

func LogConflict(sheet, anki VocabCard, winner string) {
    log.Printf("CONFLICT - Card ID %d: Sheet='%s' (modified %s) vs Anki='%s' (modified %s). Winner: %s",
        sheet.AnkiID,
        sheet.English,
        sheet.ModifiedAt.Format(time.RFC3339),
        anki.English,
        anki.ModifiedAt.Format(time.RFC3339),
        winner)
}
```

### 5.9 Complex Logic: Push Sync with Sequential Card Processing

```go
func PushToAnki(cards []VocabCard, ankiClient AnkiClient, sheetsClient SheetsClient) error {
    var newCards []VocabCard
    var existingCards []VocabCard

    // Separate new vs existing
    for _, card := range cards {
        if card.AnkiID == 0 {
            newCards = append(newCards, card)
        } else {
            existingCards = append(existingCards, card)
        }
    }

    // Create new cards sequentially
    var sheetUpdates []SheetUpdate
    if len(newCards) > 0 {
        for _, card := range newCards {
            // Create card in Anki (one at a time)
            ankiID, err := ankiClient.AddNote(card)
            if err != nil {
                return fmt.Errorf("failed to create card at row %d: %w", card.RowNumber, err)
            }

            // Calculate checksum
            card.AnkiID = ankiID
            checksum := CalculateChecksum(card)

            // Collect updates for batch write
            sheetUpdates = append(sheetUpdates, SheetUpdate{
                Row:    card.RowNumber,
                Column: "A", // Anki ID column
                Value:  ankiID,
            })
            sheetUpdates = append(sheetUpdates, SheetUpdate{
                Row:    card.RowNumber,
                Column: "B", // Checksum column
                Value:  checksum,
            })
        }

        // Write all Anki IDs and checksums to Sheet in one batch
        err := sheetsClient.BatchUpdateCells(sheetUpdates)
        if err != nil {
            return fmt.Errorf("failed to write Anki IDs and checksums to Sheet: %w", err)
        }
    }

    // Update existing cards sequentially (only if checksum changed)
    var checksumUpdates []SheetUpdate
    for _, card := range existingCards {
        currentChecksum := CalculateChecksum(card)

        // Compare with stored checksum from Sheet
        if currentChecksum != card.StoredChecksum {
            // Update card in Anki
            err := ankiClient.UpdateNoteFields(card.AnkiID, card)
            if err != nil {
                return fmt.Errorf("failed to update card %d: %w", card.AnkiID, err)
            }

            // Collect checksum update
            checksumUpdates = append(checksumUpdates, SheetUpdate{
                Row:    card.RowNumber,
                Column: "B", // Checksum column
                Value:  currentChecksum,
            })
        }
    }

    // Write updated checksums to Sheet
    if len(checksumUpdates) > 0 {
        err := sheetsClient.BatchUpdateCells(checksumUpdates)
        if err != nil {
            return fmt.Errorf("failed to write checksums to Sheet: %w", err)
        }
    }

    return nil
}
```

## 6. Non-Goals (Out of Scope)

The following features are explicitly **not** included in the initial version:

1. **Media/Image Support** - No support for images, audio, or other media files in cards. Only text-based vocabulary entries.

2. **Multiple Sheet Sources** - Tool syncs one Sheet to one Anki deck per config. No support for merging multiple Sheets.

3. **Anki Card Suspension State** - Tool does not sync suspended/unsuspended state. Users manage study state manually in Anki.

4. **Advanced Conflict Resolution** - No interactive conflict resolution UI. No field-level conflict detection. Timestamp-based resolution only.

5. **Duplicate Detection by Content** - No fuzzy matching to detect potential duplicates. Anki ID is sole matching criterion.

6. **Offline Mode** - Tool requires internet connection for Google Sheets API and Anki desktop running locally. No local caching of Sheet data.

7. **Mobile Anki Sync** - Tool only syncs with Anki desktop via AnkiConnect. Users must use Anki's built-in sync to get changes to mobile.

8. **Rich Text Formatting** - No support for bold, italic, colors in Sheet cells. No Markdown parsing. Plain text only (except simple HTML in Examples field).

9. **Undo/Rollback Operations** - No built-in undo mechanism. Users rely on Sheet version history and Anki backups.

10. **Statistics & Analytics** - No reporting on vocabulary learning progress. Focus is sync only, not study tracking.

11. **Collaborative Editing Conflict Detection** - No detection of concurrent Sheet edits by multiple users. Assumes single user or coordinated editing.

12. **Custom Field Mappings** - Column names and structure are fixed. No user-configurable field mapping.

## 7. Testing Requirements

### 7.1 Unit Tests

Test coverage for isolated functions/packages:

**Sync Logic (`internal/sync/`):**
- Checksum calculation for various field combinations
- Grammar field formatting (with/without attributes)
- Examples HTML formatting (newlines, semicolons, empty)
- Tag construction (all combinations of empty sub-tags)
- Conflict resolution logic (timestamp comparison)
- Change detection (checksum matching)

**Field Mapping (`internal/mapper/`):**
- Sheet row to VocabCard conversion
- VocabCard to Anki note fields conversion
- Validation logic for required fields
- Header detection (case-insensitive matching)

**State Management (`internal/state/`):**
- State file read/write
- Timestamp persistence
- Config hash calculation

**Target:** 80%+ code coverage

### 7.2 Integration Tests (Mocked APIs)

Test end-to-end flows with mocked Google Sheets API and AnkiConnect:

**Push Command Tests:**
- Push with all new cards (empty Anki IDs) → verify addNote calls, verify IDs written back
- Push with all existing cards (checksums match) → verify no API calls
- Push with mixed new/existing cards → verify correct operations
- Push with validation error → verify fail-fast behavior
- Push --dry-run → verify no API calls made

**Pull Command Tests:**
- Pull with modified cards → verify Sheet update calls
- Pull with no modified cards → verify no changes
- Pull with deleted cards (ID not found) → verify graceful skip

**Both Command Tests:**
- Both with changes in Sheet only → acts like push
- Both with changes in Anki only → acts like pull
- Both with conflicts → verify timestamp-based resolution
- Both with deletions → verify bidirectional deletion

**Init Command Tests:**
- Init creates config directory structure
- Init handles OAuth flow (mocked)
- Init validates Sheet ID and name

### 7.3 Testing Tools

- **Mocking:** `github.com/stretchr/testify/mock` for mocking Google Sheets and AnkiConnect clients
- **Assertions:** `github.com/stretchr/testify/assert` for test assertions
- **Test Fixtures:** Sample Sheet data in `testdata/` directory
- **Table-Driven Tests:** Use Go table-driven test pattern for field mapping variations

### 7.4 Manual Test Scenarios

Document these manual test cases for pre-release validation:

1. **Fresh setup:** Run `sync init` → authenticate → first `sync push` with 10 cards
2. **Update existing:** Change 3 cards in Sheet → `sync push` → verify only 3 updated in Anki
3. **Edit in Anki:** Change 2 cards during review → `sync pull` → verify Sheet updated
4. **Conflict:** Edit same card in both → `sync both` → verify most recent wins
5. **Delete row:** Remove row from Sheet → `sync both` → verify Anki card deleted
6. **Dry run:** Make changes → `sync push --dry-run` → verify preview shown, no changes applied

## 8. Success Metrics

The feature is considered successfully implemented when:

### Functional Success Criteria

1. User can run `sync init` and complete Google OAuth flow successfully
2. User can run `sync push` and create 100+ cards in Anki with all fields correctly mapped
3. Anki IDs are written back to Sheet after card creation
4. User can edit card in Anki, run `sync pull`, and see changes reflected in Sheet
5. User can edit row in Sheet, run `sync push`, and Anki card updates without losing review history
6. `sync both` resolves conflicts using timestamp-based logic
7. Deleting Sheet row removes Anki card (and vice versa) during `sync both`
8. `--dry-run` shows accurate preview without making changes
9. All validation errors display helpful messages with row numbers

### Technical Success Criteria

1. All unit tests pass with 80%+ coverage
2. All integration tests pass with mocked APIs
3. Manual test scenarios (6 scenarios in Section 7.4) all pass
4. Sync operations complete successfully for typical vocabulary lists (100-500 cards)
5. No sensitive data (tokens, credentials) logged at any log level
6. Tool fails gracefully with clear errors when Anki not running or network down
7. Config files created with correct permissions (0600 for token.json)
8. Checksums correctly stored and retrieved from Sheet column

### Quality Metrics

1. Zero data loss during normal operations
2. Zero duplicate card creation during incremental syncs
3. Anki review history preserved after content updates
4. Idempotent: running same sync command multiple times produces identical results

## 9. Design Decisions (Resolved)

The following design questions were resolved during PRD development:

### D1. AnkiConnect Operations Approach
**Decision:** Process cards sequentially, one at a time
- Simpler implementation and error handling
- No need to worry about batch operation support
- Performance is acceptable for typical use cases (hundreds of cards)
- Easier to log progress and identify failures

### D2. Google Sheets API Quota Management
**Decision:** No special quota handling initially
- Use BatchUpdate for Sheet writes (minimize API calls)
- No exponential backoff or retry logic needed initially
- Can add if quota issues arise in practice

### D3. Checksum Storage Location
**Decision:** Store checksums in Google Sheet as additional column
- More robust (persists even if local state lost)
- Enables change detection across different machines
- Makes debugging easier (checksums visible to user)
- Trade-off: Adds one column to user's Sheet

### D4. Initial Sync Content Matching
**Decision:** No content-based matching
- Keep implementation simple
- Anki ID is sole source of truth for matching
- Users should start with empty Anki deck or use manual import
- Avoids complexity of fuzzy matching and potential false positives

## 10. Error Handling

### 10.1 Error Categories

**Configuration Errors (Exit immediately):**
- Config file missing or malformed YAML
- Required config fields missing (sheet_id, sheet_name, anki_deck)
- Google credentials.json not found
- OAuth token expired and cannot refresh

**Connectivity Errors (Fail fast):**
- Cannot reach AnkiConnect at localhost:8765
  - Error: "Cannot connect to AnkiConnect. Is Anki running? Install: https://ankiweb.net/shared/info/2055492159"
- Cannot reach Google Sheets API
  - Error: "Cannot connect to Google Sheets API. Check internet connection."
- HTTP timeout (30 second timeout for all API calls)

**Validation Errors (Fail fast with row details):**
- Required field empty: "Validation error at row 15: 'Greek' field is required"
- Invalid Part of Speech: "Validation error at row 23: 'Part of Speech' must be non-empty"
- Malformed Sheet structure: "Error: Required column 'English' not found in sheet headers"

**Data Integrity Errors (Fail fast):**
- Anki ID in Sheet doesn't match any card in Anki: "Error: Row 42 references Anki card 1234567890123 which doesn't exist"
- Duplicate Anki IDs in Sheet: "Error: Anki ID 1234567890123 appears in multiple rows (5, 17)"
- Sheet row count changed during sync: "Error: Sheet was modified during sync. Please retry."

### 10.2 Edge Cases

**Empty Sheet:**
- If no data rows (only header), show: "No vocabulary entries found. Add rows to Sheet and retry."
- Exit successfully (not an error)

**All Cards Up-to-Date:**
- If checksums match for all rows, show: "Everything is up to date. No changes needed."
- Exit successfully

**Anki Deck Doesn't Exist:**
- Auto-create deck on first push
- Log: "Created Anki deck: Greek Vocabulary"

**VocabSync Note Type Doesn't Exist:**
- Auto-create note type on first push
- Log: "Created Anki note type: VocabSync with 2 card templates"

**First Run (No Anki IDs):**
- Treat all rows as new cards
- Create cards sequentially in Anki
- Calculate checksums for all cards
- Write all Anki IDs and checksums back to Sheet in BatchUpdate
- If Checksum column doesn't exist, tool creates it automatically
- Log: "Initial sync: Created N new cards"

**Modified Card Deleted in Anki:**
- During pull, if card in "modified" query results is not found: skip silently (was deleted)
- Only delete Sheet row during `sync both` command

**Network Interruption Mid-Sync:**
- If Sheet write fails after Anki cards created: cards exist but IDs not recorded
- Current behavior: fail with error, user must manually delete duplicate cards or re-run sync
- Future enhancement: detect duplicates via content matching

**Special Characters in Fields:**
- Properly escape HTML in Examples field (use Go's `html.EscapeString`)
- Handle Unicode Greek characters correctly (UTF-8 throughout)
- Handle quotes, commas in Sheet context

## 11. Configuration Reference

### 11.1 Config File Format

Location: `~/.sync/config.yaml`

```yaml
# Google Sheets configuration
google_sheet_id: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
sheet_name: "Vocabulary"

# Anki configuration
anki_deck: "Greek Vocabulary"
anki_connect_url: "http://localhost:8765"

# Authentication
google_token_path: "~/.sync/token.json"

# Logging
log_level: "info"  # Options: info, verbose, debug
```

### 11.2 State File Format

Location: `~/.sync/state.json`

```json
{
  "last_pull_timestamp": "2024-02-03T10:30:00Z",
  "last_push_timestamp": "2024-02-03T10:30:00Z",
  "config_hash": "a3f5d8c7e9b2..."
}
```

### 11.3 Token File

Location: `~/.sync/token.json`

Contains Google OAuth2 access and refresh tokens. Generated during `sync init` OAuth flow.
Permissions: 0600 (readable/writable by user only)

## 12. Implementation Phases

### Phase 1: Authentication & Setup
- Configure Google Cloud Console Project and OAuth2 credentials
- Implement `sync init` command with OAuth flow
- Implement token.json caching with proper permissions
- Verify AnkiConnect connectivity on localhost:8765
- Milestone: Successfully authenticate and validate connections

### Phase 2: One-Way Sync (Sheets to Anki)
- Develop VocabCard domain model with checksum support
- Implement Sheet reading with header detection and validation (including Checksum column)
- Implement sequential addNote logic for new rows (one at a time)
- Calculate checksums and write back to Sheet column
- Auto-create VocabSync note type if missing
- Auto-create deck if missing
- Milestone: Successfully push 10 words from Sheet to new Anki deck with checksums stored

### Phase 3: Update & History Preservation
- Implement checksum comparison logic (current vs stored in Sheet)
- Implement sequential updateNoteFields logic for changed cards
- Write updated checksums back to Sheet column
- Test that changing card content does not reset review intervals
- Milestone: Update existing cards without losing review history, checksums stay in sync

### Phase 4: Reverse Sync (Anki to Sheets)
- Implement "Modified Notes" query using Anki timestamps
- Develop logic to find and update specific rows in Sheet by Anki ID
- Implement BatchUpdate for Sheet modifications
- Milestone: Edit card in Anki and see change reflected in Sheet

### Phase 5: Bidirectional Sync & Conflict Resolution
- Implement `sync both` command
- Implement timestamp-based conflict resolution
- Implement conflict logging
- Implement deletion sync
- Milestone: Full bidirectional sync with conflict handling

### Phase 6: Polish & Testing
- Implement --dry-run flag
- Implement --verbose and --debug logging
- Write comprehensive unit tests
- Write integration tests with mocked APIs
- Perform manual test scenarios
- Milestone: 80%+ test coverage, all tests passing
