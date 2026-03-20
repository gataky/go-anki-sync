# Audio Regeneration with Versioned Filenames Design

**Date:** 2026-03-20
**Status:** Approved

## Overview

Add a "Regen TTS" column to the Google Sheet that triggers audio regeneration regardless of whether audio already exists. Implement versioned audio filenames to ensure Anki recognizes when audio files change.

## Problem Statement

Currently, audio is only generated when the file doesn't exist in Anki's media directory. This causes issues when:
1. Users want to regenerate audio with different TTS settings
2. Users switch TTS providers (Google TTS ↔ ElevenLabs)
3. Audio quality needs improvement but the file already exists

Additionally, Anki doesn't reliably detect when audio files are replaced because the filename stays the same, leading to caching issues.

## Goals

1. Allow users to force audio regeneration via a sheet column
2. Use versioned filenames so Anki always detects new audio
3. Support multiple TTS providers with distinct versioning
4. Maintain backward compatibility with existing audio files
5. Automatically clean up the regeneration flag after completion

## Audio Filename Format

### New Format

**Pattern**: `{greek-word}-{source}-{version}.mp3`

**Components**:
- `{greek-word}`: The Greek text from the card (e.g., "γεια")
- `{source}`: Provider identifier
  - `etts` = ElevenLabs TTS
  - `gtts` = Google Cloud TTS
- `{version}`: Auto-incremented number starting at 1

**Examples**:
- `γεια-etts-1.mp3` - First ElevenLabs generation
- `γεια-etts-2.mp3` - Regenerated with ElevenLabs
- `γεια-gtts-1.mp3` - First Google TTS generation (independent versioning)
- `Καλημέρα-etts-3.mp3` - Third version with ElevenLabs

### Legacy Format

**Pattern**: `{greek-word}.mp3`

**Examples**:
- `γεια.mp3` - Old format (still valid, will be linked to cards)

**Behavior**: Legacy files remain functional. When regeneration is triggered or new audio is generated, the new versioned format is used.

## Version Number Determination

### Algorithm

For a given Greek word and current provider:

1. Determine provider source code:
   - ElevenLabs → `etts`
   - Google TTS → `gtts`

2. Scan Anki media directory for files matching: `{greek-word}-{source}-*.mp3`

3. Parse version numbers from matching filenames

4. Return `max(versions) + 1`, or `1` if no matching files found

### Example Scenarios

| Existing Files | Current Provider | Next Filename |
|----------------|------------------|---------------|
| (none) | ElevenLabs | `γεια-etts-1.mp3` |
| `γεια.mp3` (legacy) | ElevenLabs | `γεια-etts-1.mp3` |
| `γεια-etts-1.mp3` | ElevenLabs | `γεια-etts-2.mp3` |
| `γεια-etts-2.mp3`, `γεια-gtts-1.mp3` | ElevenLabs | `γεια-etts-3.mp3` |
| `γεια-etts-2.mp3`, `γεια-gtts-1.mp3` | Google TTS | `γεια-gtts-2.mp3` |

**Important**: Legacy files (`{word}.mp3`) are ignored when calculating the next version. Versioning always starts at 1 per provider.

## "Regen TTS" Column

### Purpose

A new optional column in the Google Sheet that explicitly triggers audio regeneration.

### Behavior

| Cell Value | Action |
|------------|--------|
| Empty (blank) | Normal behavior: skip if audio exists, generate if missing |
| Any non-empty value | Force regeneration with incremented version number |

**Examples of trigger values**: "x", "1", "yes", "regen", "true" - any value works

### Workflow

1. User adds a value to "Regen TTS" column for cards that need audio regeneration
2. Run `./sync push`
3. Tool regenerates audio for flagged cards with incremented version
4. Tool automatically clears the "Regen TTS" cell after successful regeneration
5. Next push won't regenerate unless user adds value again

### Auto-Clear Behavior

After successful audio regeneration, the "Regen TTS" cell is automatically cleared by the sync tool. This provides:
- Clear indication that regeneration completed
- Ready state for next regeneration request
- Clean audit trail (new filename = proof of regeneration)

## Audio Generation Logic

### Current Logic (Simplified)

```
filename = "{greek}.mp3"

if audioFileExists(filename):
    return nil, filename  // Link existing audio
else:
    audioData = generateAudio(greek)
    return audioData, filename  // Upload new audio
```

### New Logic

```
shouldRegenerate = card.RegenTTS != ""  // Any value triggers regen

if shouldRegenerate:
    source = getProviderSource(currentProvider)  // "etts" or "gtts"
    version = getNextAudioVersion(greek, source)  // Scan and increment
    filename = "{greek}-{source}-{version}.mp3"
    audioData = generateAudio(greek)
    return audioData, filename  // Upload new versioned audio

else:
    // Try legacy format first
    legacyFilename = "{greek}.mp3"
    if audioFileExists(legacyFilename):
        return nil, legacyFilename  // Link existing legacy audio

    // Try versioned format
    source = getProviderSource(currentProvider)
    existingVersionedFile = findExistingVersionedAudio(greek, source)
    if existingVersionedFile != "":
        return nil, existingVersionedFile  // Link existing versioned audio

    // Generate new versioned audio
    version = getNextAudioVersion(greek, source)
    filename = "{greek}-{source}-{version}.mp3"
    audioData = generateAudio(greek)
    return audioData, filename  // Upload new audio
```

## Implementation Details

### 1. VocabCard Model Changes

**File**: `pkg/models/card.go`

Add new field:
```go
type VocabCard struct {
    // ... existing fields ...

    // RegenTTS indicates whether to force audio regeneration
    // Any non-empty value triggers regeneration
    RegenTTS string
}
```

### 2. Field Mapper Changes

**File**: `internal/mapper/field_mapper.go`

**In `RowToCard()`**:
```go
// Extract optional fields
card.RegenTTS, _ = getString(row, headers, "regen tts")
```

**In `CardToRow()`**:
```go
// Fill in the values
setCell(row, headers, "regen tts", card.RegenTTS)
```

### 3. Pusher Changes

**File**: `internal/sync/pusher.go`

#### New Helper Functions

```go
// getProviderSource returns the source code for the current TTS provider
func (p *Pusher) getProviderSource() string {
    if p.config.TextToSpeech == nil {
        return ""
    }

    provider := strings.ToLower(p.config.TextToSpeech.Provider)
    if provider == "" {
        provider = "elevenlabs"  // Default
    }

    switch provider {
    case "elevenlabs":
        return "etts"
    case "google":
        return "gtts"
    default:
        return ""
    }
}

// getNextAudioVersion finds the highest version number for a Greek word + source
// Returns 1 if no versioned files exist
func (p *Pusher) getNextAudioVersion(greekWord string, source string) int {
    mediaDir := p.getAnkiMediaDir()
    if mediaDir == "" {
        return 1
    }

    // Pattern: {greek}-{source}-*.mp3
    pattern := fmt.Sprintf("%s-%s-", greekWord, source)

    files, err := os.ReadDir(mediaDir)
    if err != nil {
        return 1
    }

    maxVersion := 0
    for _, file := range files {
        name := file.Name()
        if !strings.HasPrefix(name, pattern) || !strings.HasSuffix(name, ".mp3") {
            continue
        }

        // Extract version number
        // name format: "{greek}-{source}-{version}.mp3"
        versionPart := strings.TrimPrefix(name, pattern)
        versionPart = strings.TrimSuffix(versionPart, ".mp3")

        version, err := strconv.Atoi(versionPart)
        if err != nil {
            continue
        }

        if version > maxVersion {
            maxVersion = version
        }
    }

    return maxVersion + 1
}

// buildAudioFilename creates a versioned filename for audio
func (p *Pusher) buildAudioFilename(greekWord string, source string, version int) string {
    return fmt.Sprintf("%s-%s-%d.mp3", greekWord, source, version)
}

// findExistingAudio looks for existing audio in both legacy and versioned formats
// Returns empty string if no audio found
func (p *Pusher) findExistingAudio(greekWord string, source string) string {
    mediaDir := p.getAnkiMediaDir()
    if mediaDir == "" {
        return ""
    }

    // Check legacy format first
    legacyFilename := fmt.Sprintf("%s.mp3", greekWord)
    if p.audioFileExists(legacyFilename) {
        return legacyFilename
    }

    // Check for latest versioned file
    files, err := os.ReadDir(mediaDir)
    if err != nil {
        return ""
    }

    pattern := fmt.Sprintf("%s-%s-", greekWord, source)
    var latestFile string
    maxVersion := 0

    for _, file := range files {
        name := file.Name()
        if !strings.HasPrefix(name, pattern) || !strings.HasSuffix(name, ".mp3") {
            continue
        }

        versionPart := strings.TrimPrefix(name, pattern)
        versionPart = strings.TrimSuffix(versionPart, ".mp3")

        version, err := strconv.Atoi(versionPart)
        if err != nil {
            continue
        }

        if version > maxVersion {
            maxVersion = version
            latestFile = name
        }
    }

    return latestFile
}
```

#### Modified `generateAudioForCard()`

Replace the current implementation with:

```go
func (p *Pusher) generateAudioForCard(card *models.VocabCard) ([]byte, string) {
    // Validate Greek text
    greekText := strings.TrimSpace(card.Greek)
    if greekText == "" {
        p.logger.Warn("Skipping audio generation for card '%s' (row %d): Greek text is empty", card.English, card.RowNumber)
        return nil, ""
    }

    source := p.getProviderSource()
    if source == "" {
        p.logger.Warn("Unknown TTS provider, skipping audio for '%s'", card.Greek)
        return nil, ""
    }

    shouldRegenerate := strings.TrimSpace(card.RegenTTS) != ""

    if shouldRegenerate {
        // Force regeneration with incremented version
        version := p.getNextAudioVersion(greekText, source)
        filename := p.buildAudioFilename(greekText, source, version)

        audioData, err := p.ttsClient.GenerateAudio(greekText)
        if err != nil {
            p.logger.Error("Failed to generate audio for '%s': %v", greekText, err)
            return nil, ""
        }

        p.logger.Info("Regenerated audio for '%s' as %s (%d bytes)", greekText, filename, len(audioData))
        return audioData, filename
    }

    // Check for existing audio
    existingFile := p.findExistingAudio(greekText, source)
    if existingFile != "" {
        p.logger.Info("Audio already exists for '%s', linking to card: %s", greekText, existingFile)
        return nil, existingFile
    }

    // Generate new versioned audio
    version := p.getNextAudioVersion(greekText, source)
    filename := p.buildAudioFilename(greekText, source, version)

    audioData, err := p.ttsClient.GenerateAudio(greekText)
    if err != nil {
        p.logger.Error("Failed to generate audio for '%s': %v", greekText, err)
        return nil, ""
    }

    p.logger.Info("Generated audio for '%s' as %s (%d bytes)", greekText, filename, len(audioData))
    return audioData, filename
}
```

#### Update `createNewCards()` and `updateExistingCards()`

After successful audio generation when `card.RegenTTS != ""`:

```go
// In createNewCards(), after AddNote succeeds:
if ttsEnabled && strings.TrimSpace(card.RegenTTS) != "" {
    // Clear the Regen TTS flag
    updates = append(updates, sheets.BuildRegenTTSClearUpdate(card.RowNumber)...)
}

// In updateExistingCards(), after UpdateNoteFields succeeds:
if ttsEnabled && strings.TrimSpace(card.RegenTTS) != "" {
    // Clear the Regen TTS flag
    updates = append(updates, sheets.BuildRegenTTSClearUpdate(card.RowNumber)...)
}
```

### 4. Sheets Update Builder

**File**: `internal/sheets/update_builder.go`

Add new helper function:

```go
// BuildRegenTTSClearUpdate creates an update to clear the "Regen TTS" column
func BuildRegenTTSClearUpdate(rowNumber int) []CellUpdate {
    return []CellUpdate{
        {
            Row:    rowNumber,
            Column: "Regen TTS",
            Value:  "",  // Clear the cell
        },
    }
}
```

## Backward Compatibility

### Legacy Audio Files

**Behavior**: Existing audio files in legacy format (`{word}.mp3`) continue to work:
- They will be linked to cards that don't have audio
- They are ignored when calculating next version numbers
- They are not deleted or modified

**Migration**: No automatic migration. Legacy files remain until:
- User triggers regeneration via "Regen TTS" column
- User switches TTS provider
- Greek text is modified (triggers checksum change)

### Existing Cards

**No changes required**: Existing cards with audio continue working:
- If they have legacy audio, it remains linked
- If user triggers regeneration, new versioned file is created
- Old audio files remain in media directory (Anki manages cleanup)

## Testing Strategy

### Unit Tests

1. **Version number calculation**:
   - No files → version 1
   - Legacy file only → version 1
   - Versioned files → max + 1
   - Mixed providers → correct per-provider versioning

2. **Filename generation**:
   - Correct format with Greek characters
   - Correct source codes
   - Correct version numbers

3. **Regeneration logic**:
   - Empty RegenTTS → normal behavior
   - Non-empty RegenTTS → force regeneration
   - RegenTTS cleared after success

### Integration Tests

1. **First audio generation**: Creates `{word}-{source}-1.mp3`
2. **Regeneration request**: Creates `{word}-{source}-2.mp3`
3. **Provider switch**: Creates `{word}-{new-source}-1.mp3`
4. **Legacy migration**: Legacy file linked, regeneration creates versioned file
5. **RegenTTS column**: Cleared after successful regeneration

### Manual Tests

1. Add "Regen TTS" column to test sheet
2. Set value on card with existing audio
3. Run `./sync push`
4. Verify new versioned file created
5. Verify "Regen TTS" cell cleared
6. Verify Anki plays new audio

## Edge Cases

### Greek Text Contains Special Characters

**Issue**: Filenames with special characters may cause issues on some filesystems.

**Solution**: Greek Unicode characters are valid in filenames on modern systems (macOS, Linux, Windows). No sanitization needed for Greek text. Hyphens and numbers are always safe.

### Multiple Cards Same Greek Word

**Behavior**: Multiple cards with same Greek text share the same audio file. This is intended behavior - the Greek word is the key for audio generation.

**Example**: Two cards for "γεια" (different English translations) both use `γεια-etts-1.mp3`.

### Regeneration During Push Fails

**Behavior**:
- If audio generation fails, "Regen TTS" is NOT cleared
- User can retry on next push
- Partial updates (other cards) are still written to sheet

### Version Number Overflow

**Unlikely**: If version numbers exceed int32 (2 billion+), version calculation may fail. This would require 2 billion regenerations of the same word, which is unrealistic.

## Performance Considerations

### File System Scanning

**Impact**: `getNextAudioVersion()` scans the media directory per card generation.

**Mitigation**:
- Scans are only performed when generating new audio
- Directory scans are fast (typically < 1ms for thousands of files)
- Caching not needed for batch operations

### Sheet Updates

**Impact**: Clearing "Regen TTS" adds one update per regenerated card.

**Mitigation**: Batch updates are already used, minimal overhead.

## Future Enhancements

1. **Bulk regeneration**: Add CLI flag `--force-regen` to regenerate all audio
2. **Provider-specific regeneration**: Separate columns for "Regen ElevenLabs" and "Regen Google"
3. **Audio preview**: CLI command to test TTS without full push
4. **Version cleanup**: CLI command to delete old versions and keep only latest
5. **Audio diff tool**: Compare audio quality between versions

## Success Criteria

- [ ] "Regen TTS" column triggers audio regeneration
- [ ] Versioned filenames follow `{word}-{source}-{version}.mp3` format
- [ ] Version numbers auto-increment correctly per provider
- [ ] Legacy audio files continue working
- [ ] "Regen TTS" cleared after successful regeneration
- [ ] Anki recognizes and plays new audio versions
- [ ] Documentation updated with new column usage
