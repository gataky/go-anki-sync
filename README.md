# Anki-Sheets Sync

A bidirectional sync tool for managing vocabulary flashcards between Google Sheets and Anki.

**🚀 New to this tool? Start with [SETUP.md](SETUP.md) for a 5-minute quick start guide!**

## Features

- **Bidirectional Sync**: Keep your Google Sheets and Anki deck in perfect sync
- **Checksum-based Change Detection**: Only syncs cards that have actually changed
- **Conflict Resolution**: Timestamp-based last-write-wins strategy
- **Dry-run Mode**: Preview changes before applying them
- **Greek Audio Generation**: Automatic text-to-speech audio for Greek words using Google Cloud TTS
- **Three Sync Modes**:
  - `push`: Google Sheets → Anki
  - `pull`: Anki → Google Sheets
  - `both`: Bidirectional with conflict resolution

## Greek Audio Generation (Text-to-Speech)

The tool can automatically generate high-quality audio pronunciation for Greek words using Google Cloud Text-to-Speech.

### Requirements

1. **Google Cloud Project** with Text-to-Speech API enabled
2. **Service Account** with `roles/cloudtts.user` role
3. Same service account JSON key file used for Sheets access

### Setup

1. Enable the Cloud Text-to-Speech API in your Google Cloud project:
   ```bash
   # In Google Cloud Console
   https://console.cloud.google.com/apis/library/texttospeech.googleapis.com
   ```

2. Add the `Cloud Text-to-Speech User` role to your service account:
   - Go to IAM & Admin → IAM
   - Find your service account
   - Click Edit → Add Another Role
   - Select `Cloud Text-to-Speech User`

3. Add TTS configuration to `~/.sync/config.yaml`:
   ```yaml
   text_to_speech:
     enabled: true
     voice_name: "el-GR-Wavenet-A"  # Greek voice
     audio_encoding: "MP3"
     speaking_rate: 1.0              # 0.25 to 4.0
     pitch: 0.0                      # -20.0 to 20.0
     volume_gain_db: 0.0             # -96.0 to 16.0
     request_delay_ms: 100           # Delay between TTS requests
   ```

### How It Works

- **Audio for new and updated cards**: Audio is generated when:
  - Creating new cards during `push`
  - Updating existing cards during `push` (if Greek text has changed)
- **Auto-play on Greek side**: When the Greek side of the card is shown, the audio automatically plays
- **Text + Audio**: The Greek text is displayed along with the audio (`γεια[sound:γεια.mp3]`)
- **Smart audio generation**: Checks Anki's media folder before generating
  - If audio exists: Links the existing file to the card (no TTS API call, no file upload)
  - If audio doesn't exist: Generates fresh audio using Google Cloud TTS and uploads it
- **Single audio attachment**: Each card gets exactly one audio file, no duplicates
- **Graceful degradation**: If audio generation fails, the card is still created/updated without audio
- **Sequential processing**: Cards are processed one at a time with configurable delay

### Supported Voices

Google Cloud TTS supports multiple Greek voices:
- `el-GR-Wavenet-A` (recommended) - High quality WaveNet voice
- `el-GR-Standard-A` - Standard quality voice

### Troubleshooting

**"Failed to initialize TTS client"**
- Verify Text-to-Speech API is enabled in Google Cloud Console
- Check that your service account has the `roles/cloudtts.user` role
- Ensure your service account JSON key file is valid

**"Audio generation failed"**
- Check your Google Cloud quota for TTS requests
- Verify the Greek text is not empty
- Try increasing `request_delay_ms` if hitting rate limits

**Disabling Audio Generation**
Set `enabled: false` in the `text_to_speech` config or remove the section entirely.

## Installation

### Prerequisites

1. **Go 1.25+**: Install from [golang.org](https://golang.org/dl/)
2. **Anki with AnkiConnect**:
   - Install Anki from [apps.ankiweb.net](https://apps.ankiweb.net/)
   - Install AnkiConnect add-on: Code `2055492159`
3. **Google Service Account** (much simpler than OAuth2!):
   - Create a project at [console.cloud.google.com](https://console.cloud.google.com/)
   - Enable the Google Sheets API
   - Create a Service Account (IAM & Admin → Service Accounts)
   - Download the JSON key file
   - **Share your Google Sheet with the service account email**

### Build from Source

```bash
git clone https://github.com/yourusername/sync.git
cd sync
go build -o sync ./cmd/sync
```

## Quick Start

### 1. Set Up Google Service Account

1. Go to [console.cloud.google.com](https://console.cloud.google.com/)
2. Create/select a project
3. Enable Google Sheets API
4. Go to **IAM & Admin → Service Accounts**
5. Click **Create Service Account** (any name works)
6. Click on the service account → **Keys** → **Add Key** → **Create New Key** → **JSON**
7. Download the JSON key file
8. Move it to `~/.sync/service-account.json`
9. **CRITICAL**: Open your Google Sheet and click **Share**
   - Add the service account email (looks like `your-service@project.iam.gserviceaccount.com`)
   - Give it **Editor** permissions

### 2. Initialize Configuration

```bash
./sync init
```

You'll be prompted for:
- **Google Sheet ID**: Found in the URL of your spreadsheet
  Example: `1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms`
- **Sheet Name**: The tab name within your spreadsheet (default: "Sheet1")
- **Anki Deck Name**: The deck you want to sync to

### 3. First Push (Sheets → Anki)

```bash
# Preview what will be synced
./sync push --dry-run

# Actually sync
./sync push
```

No browser authorization needed - service accounts work automatically!

### 4. Pull Changes (Anki → Sheets)

```bash
# Preview changes from Anki
./sync pull --dry-run

# Sync changes from Anki to Sheet
./sync pull
```

### 5. Bidirectional Sync

```bash
# Sync in both directions with conflict resolution
./sync both
```

## Google Sheet Format

Your spreadsheet should have these columns (order doesn't matter):

| Column | Required | Description |
|--------|----------|-------------|
| Anki ID | No | Auto-populated by the tool |
| Checksum | No | Auto-populated for change detection |
| English | Yes | English word/phrase |
| Greek | Yes | Greek translation |
| Part of Speech | Yes | Noun, Verb, Adjective, etc. |
| Attributes | No | Gender, verb class, etc. |
| Examples | No | Usage examples (multiple lines become numbered list) |
| Tag | No | Top-level tag |
| Sub-Tag 1 | No | Second-level tag |
| Sub-Tag 2 | No | Third-level tag |

**Examples Field Formatting:**
- Single example: Displays as plain text
- Multiple examples: Use newlines to separate; displays as a numbered list in Anki
- Example in sheet cell:
  ```
  This is the first example
  This is the second example
  This is the third example
  ```
- Result in Anki:
  1. This is the first example
  2. This is the second example
  3. This is the third example

### Example

| Anki ID | Checksum | English | Greek | Part of Speech | Attributes | Examples | Tag | Sub-Tag 1 | Sub-Tag 2 |
|---------|----------|---------|-------|----------------|------------|----------|-----|-----------|-----------|
|  | | hello | γεια | Interjection | Informal | Hello, how are you?<br>Γεια σου! | Greetings | Basic | |
|  | | house | σπίτι | Noun | Neuter | This is my house.<br>Where is your house? | City | Buildings | Residential |

## Command Reference

### Global Flags

- `-v, --verbose`: Enable verbose logging with timestamps
- `--debug`: Enable debug logging with file/line info
- `-n, --dry-run`: Preview changes without applying them

### Commands

#### `init`
Initialize configuration by prompting for Sheet ID, sheet name, and Anki deck name.

```bash
./sync init
```

#### `push`
Push new and modified cards from Google Sheets to Anki.

```bash
./sync push [--dry-run] [--verbose]
```

- Creates new Anki cards for rows without an Anki ID
- Updates existing cards if content has changed (checksum mismatch)
- Writes Anki IDs and checksums back to the Sheet

#### `pull`
Pull modified cards from Anki to Google Sheets.

```bash
./sync pull [--dry-run] [--verbose]
```

- Queries Anki for cards modified since last pull
- Updates corresponding Sheet rows
- Tracks last pull timestamp in `~/.sync/state.json`

#### `both`
Bidirectional sync with conflict resolution.

```bash
./sync both [--dry-run] [--verbose]
```

- Pushes Sheet changes to Anki
- Pulls Anki changes to Sheet
- Resolves conflicts using timestamp-based strategy (most recent wins)
- Logs all conflicts with resolution details

## Configuration

Configuration is stored in `~/.sync/config.yaml`:

```yaml
google_sheet_id: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
sheet_name: "Sheet1"
anki_deck: "Greek"
anki_connect_url: "http://localhost:8765"
anki_profile: "User 1"  # Your Anki profile name (default: "User 1")
log_level: "info"

# Optional: Greek audio generation with Google Cloud TTS
text_to_speech:
  enabled: true
  voice_name: "el-GR-Wavenet-A"
  audio_encoding: "MP3"
  speaking_rate: 1.0
  pitch: 0.0
  volume_gain_db: 0.0
  request_delay_ms: 100
```

Service account credentials: `~/.sync/service-account.json`

State (timestamps) is stored in `~/.sync/state.json`:

```json
{
  "last_pull_timestamp": "2026-02-03T10:30:00Z",
  "last_push_timestamp": "2026-02-03T10:30:00Z",
  "config_hash": "abc123..."
}
```

## How It Works

### Change Detection

The tool uses SHA256 checksums of content fields to detect changes:

- **Content fields**: English, Greek, Part of Speech, Attributes, Examples, Tag, Sub-Tag 1, Sub-Tag 2
- **Excluded from checksum**: Anki ID, Checksum (itself), Row Number, Modified At

### Duplicate Detection

**Duplicates are determined by the English + Greek combination:**

- ✅ Allowed: Same English, different Greek
  - Example: "when" (όταν) - conjunction
  - Example: "when" (πότε) - adverb
- ❌ Rejected: Same English + Greek (true duplicate)
  - Example: "hello" (γεια) appears twice

This allows multiple translations/meanings of the same English word.

### Conflict Resolution

When a card is modified in both Sheets and Anki:

1. The tool detects the conflict
2. Compares modification timestamps
3. **Anki wins** if it has a modification timestamp
4. **Sheet wins** if Anki has no timestamp
5. Logs the conflict with details
6. Applies the winner's changes to both systems

### AnkiConnect Note Type

The tool automatically creates a "VocabSync" note type with:

**Fields:**
- Front (English)
- Back (Greek)
- Grammar (Part of Speech + Attributes)
- Examples

**Cards:**
1. **English → Greek**: Shows English, reveals Greek with grammar and examples
2. **Greek → English**: Shows Greek, reveals English with grammar and examples

### Tags

Tags are hierarchical using Anki's `::` separator:

- Single tag: `City`
- Two levels: `City::Buildings`
- Three levels: `City::Buildings::Museums`

## Troubleshooting

### "Cannot connect to AnkiConnect"

- Ensure Anki is running
- Verify AnkiConnect is installed (Tools → Add-ons → Code: 2055492159)
- Check AnkiConnect is listening on http://localhost:8765

### "Config file not found"

Run `./sync init` to create the configuration.

### "Service account key file not found"

1. Go to [console.cloud.google.com](https://console.cloud.google.com/)
2. Go to IAM & Admin → Service Accounts
3. Create Service Account
4. Download JSON key
5. Move to `~/.sync/service-account.json`

### "Failed to read sheet" or "Permission denied"

**Make sure you shared the Google Sheet with your service account email!**

1. Open your Google Sheet
2. Click **Share**
3. Add the service account email (from the JSON key file: `client_email` field)
4. Give it **Editor** permissions

This is the most common mistake - the service account needs explicit access.

### "Required columns missing"

Your Google Sheet must have at minimum: English, Greek, and Part of Speech columns.

## Development

### Project Structure

```
sync/
├── cmd/sync/           # Main entry point
├── internal/
│   ├── anki/          # AnkiConnect client
│   ├── cli/           # Cobra CLI commands
│   ├── config/        # Configuration management
│   ├── mapper/        # Field mapping and checksums
│   ├── sheets/        # Google Sheets client
│   ├── state/         # State persistence
│   └── sync/          # Sync orchestration
├── pkg/models/        # Domain models
└── testdata/          # Test fixtures
```

### Running Tests

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/sync/...
```

### Building

```bash
# Development build
go build -o sync ./cmd/sync

# Production build
go build -ldflags="-s -w" -o sync ./cmd/sync
```

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Acknowledgments

- [AnkiConnect](https://github.com/FooSoft/anki-connect) for the Anki API
- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [Google Sheets API](https://developers.google.com/sheets/api) for sheet access
