package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gataky/sync/internal/anki"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/internal/sheets"
	"github.com/gataky/sync/internal/util"
	"github.com/gataky/sync/pkg/models"
)

// Pusher orchestrates syncing data from Google Sheets to Anki.
type Pusher struct {
	sheetsClient SheetsClientInterface
	ankiClient   AnkiClientInterface
	config       *models.Config
	logger       *logging.SyncLogger
	ttsClient    TTSClientInterface
}

// NewPusher creates a new Pusher instance.
func NewPusher(
	sheetsClient SheetsClientInterface,
	ankiClient AnkiClientInterface,
	config *models.Config,
	logger *logging.SyncLogger,
	ttsClient TTSClientInterface,
) *Pusher {
	return &Pusher{
		sheetsClient: sheetsClient,
		ankiClient:   ankiClient,
		config:       config,
		logger:       logger,
		ttsClient:    ttsClient,
	}
}

// Push executes the push sync from Google Sheets to Anki.
// If dryRun is true, no changes are made to Anki or Sheet.
func (p *Pusher) Push(dryRun bool) error {
	p.logger.Info("Starting push sync (Sheets → Anki)")

	// Read all rows from Sheet
	rows, err := p.sheetsClient.ReadSheet(p.config.GoogleSheetID, p.config.SheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet: %w", err)
	}

	// Parse headers
	headers, err := p.sheetsClient.ParseHeaders(rows)
	if err != nil {
		return fmt.Errorf("failed to parse headers: %w", err)
	}

	// Validate required columns
	requiredColumns := []string{"English", "Greek", "Part of Speech"}
	if err := p.sheetsClient.ValidateRequiredColumns(headers, requiredColumns); err != nil {
		return fmt.Errorf("sheet validation failed: %w", err)
	}

	// Create Checksum column if missing
	if !dryRun {
		if err := p.sheetsClient.CreateChecksumColumnIfMissing(
			p.config.GoogleSheetID,
			p.config.SheetName,
			headers,
		); err != nil {
			return fmt.Errorf("failed to create checksum column: %w", err)
		}

		// Re-read headers after potentially adding checksum column
		rows, err = p.sheetsClient.ReadSheet(p.config.GoogleSheetID, p.config.SheetName)
		if err != nil {
			return fmt.Errorf("failed to re-read sheet: %w", err)
		}
		headers, err = p.sheetsClient.ParseHeaders(rows)
		if err != nil {
			return fmt.Errorf("failed to re-parse headers: %w", err)
		}
	}

	// Convert rows to VocabCards (skip header row)
	cards := make([]*models.VocabCard, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		card, err := mapper.RowToCard(row, headers, i+1)
		if err != nil {
			return fmt.Errorf("failed to parse row %d: %w", i+1, err)
		}

		// Validate card
		if err := mapper.ValidateCard(card); err != nil {
			return fmt.Errorf("validation failed for row %d: %w", i+1, err)
		}

		cards = append(cards, card)
	}

	p.logger.Info("Loaded %d cards from sheet", len(cards))

	// Separate into new and existing cards
	newCards := make([]*models.VocabCard, 0)
	existingCards := make([]*models.VocabCard, 0)

	for _, card := range cards {
		if card.AnkiID == 0 {
			newCards = append(newCards, card)
		} else {
			existingCards = append(existingCards, card)
		}
	}

	p.logger.Info("Found %d new cards, %d existing cards", len(newCards), len(existingCards))

	// Ensure deck exists
	if !dryRun {
		if err := p.ankiClient.CreateDeck(p.config.AnkiDeck); err != nil {
			return fmt.Errorf("failed to create deck: %w", err)
		}
		p.logger.Info("Ensured deck '%s' exists", p.config.AnkiDeck)
	}

	// Ensure VocabSync note type exists
	if !dryRun {
		if err := p.ankiClient.CreateNoteType(anki.VocabSyncModelName); err != nil {
			return fmt.Errorf("failed to create note type: %w", err)
		}
		p.logger.Info("Ensured note type '%s' exists", anki.VocabSyncModelName)
	}

	// Process new cards - collect partial results even on error
	newCardUpdates, newCardErr := p.createNewCards(newCards, dryRun)

	// Process existing cards - collect partial results even on error
	existingCardUpdates, existingCardErr := p.updateExistingCards(existingCards, dryRun)

	// Write partial updates to Sheet even if there were errors
	// This allows us to resume from where we left off on retry
	if !dryRun && (len(newCardUpdates) > 0 || len(existingCardUpdates) > 0) {
		allUpdates := append(newCardUpdates, existingCardUpdates...)
		if err := p.sheetsClient.BatchUpdateCells(
			p.config.GoogleSheetID,
			p.config.SheetName,
			allUpdates,
		); err != nil {
			// Sheet write failure is critical - return immediately
			return fmt.Errorf("failed to write updates to sheet: %w", err)
		}
		p.logger.Info("Wrote %d updates to sheet", len(allUpdates))
	}

	// Print summary
	p.logger.PrintSummary("Push")

	// Return combined errors if any occurred
	// Note: Partial results were already written to sheet above
	if newCardErr != nil && existingCardErr != nil {
		return fmt.Errorf("multiple errors occurred - new cards: %v; existing cards: %v", newCardErr, existingCardErr)
	} else if newCardErr != nil {
		return fmt.Errorf("failed to create new cards: %w", newCardErr)
	} else if existingCardErr != nil {
		return fmt.Errorf("failed to update existing cards: %w", existingCardErr)
	}

	return nil
}

// getAnkiMediaDir returns the path to the Anki media directory for the configured profile.
// Returns empty string if the directory cannot be determined.
func (p *Pusher) getAnkiMediaDir() string {
	profile := p.config.AnkiProfile
	if profile == "" {
		profile = "User 1"
	}

	var baseDir string
	switch runtime.GOOS {
	case "darwin": // macOS
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		baseDir = filepath.Join(homeDir, "Library", "Application Support", "Anki2")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return ""
		}
		baseDir = filepath.Join(appData, "Anki2")
	case "linux":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		baseDir = filepath.Join(homeDir, ".local", "share", "Anki2")
	default:
		return ""
	}

	return filepath.Join(baseDir, profile, "collection.media")
}

// audioFileExists checks if an audio file already exists in Anki's media directory.
func (p *Pusher) audioFileExists(filename string) bool {
	mediaDir := p.getAnkiMediaDir()
	if mediaDir == "" {
		// Can't determine media directory, assume file doesn't exist
		return false
	}

	filePath := filepath.Join(mediaDir, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// getProviderSource returns the source code for the current TTS provider.
// Returns "etts" for ElevenLabs, "gtts" for Google TTS, or empty string if unknown.
func (p *Pusher) getProviderSource() string {
	if p.config.TextToSpeech == nil {
		return ""
	}

	provider := strings.ToLower(p.config.TextToSpeech.Provider)
	if provider == "" {
		provider = "elevenlabs" // Default
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

// getNextAudioVersion finds the highest version number for a Greek word + source.
// Scans Anki media directory for files matching {greekWord}-{source}-*.mp3
// Returns max(versions) + 1, or 1 if no versioned files exist.
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

		// Extract version number from: {greek}-{source}-{version}.mp3
		versionPart := strings.TrimPrefix(name, pattern)
		versionPart = strings.TrimSuffix(versionPart, ".mp3")

		version, err := strconv.Atoi(versionPart)
		if err != nil {
			continue // Skip malformed filenames
		}

		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion + 1
}

// buildAudioFilename creates a versioned filename for audio.
// Format: {greekWord}-{source}-{version}.mp3
func (p *Pusher) buildAudioFilename(greekWord string, source string, version int) string {
	return fmt.Sprintf("%s-%s-%d.mp3", greekWord, source, version)
}

// findExistingAudio looks for existing audio in both legacy and versioned formats.
// Returns filename if found, empty string if no audio exists.
// Checks legacy format ({word}.mp3) first, then latest versioned file.
func (p *Pusher) findExistingAudio(greekWord string, source string) string {
	mediaDir := p.getAnkiMediaDir()
	if mediaDir == "" {
		return ""
	}

	// Check legacy format first: {word}.mp3
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

// generateAudioForCard generates audio for a vocabulary card using TTS.
// Returns (audioData, filename) where:
// - audioData is non-nil only if new audio was generated (needs uploading)
// - filename is always set if audio should be attached (even if it already exists)
// Supports versioned filenames and regeneration via card.RegenTTS flag.
// All errors are logged but do not block card creation.
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
			p.logger.Error("Failed to regenerate audio for '%s': %v", greekText, err)
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

// createNewCards processes new cards (AnkiID == 0) and creates them in Anki.
// Returns a list of CellUpdates for writing Anki IDs and checksums back to the Sheet.
// If errors occur, partial results are still returned along with a combined error.
func (p *Pusher) createNewCards(cards []*models.VocabCard, dryRun bool) ([]sheets.CellUpdate, error) {
	if len(cards) == 0 {
		return []sheets.CellUpdate{}, nil
	}

	updates := make([]sheets.CellUpdate, 0, len(cards)*2)
	var errors []error
	successCount := 0

	// Check if TTS is enabled
	ttsEnabled := p.ttsClient != nil && p.config.TextToSpeech != nil && p.config.TextToSpeech.Enabled

	for _, card := range cards {
		if dryRun {
			p.logger.Info("DRY RUN: Would create card '%s' (%s)", card.English, card.Greek)

			// Check if audio would be generated
			if ttsEnabled && strings.TrimSpace(card.Greek) != "" {
				filename := fmt.Sprintf("%s.mp3", card.Greek)
				if p.audioFileExists(filename) {
					p.logger.Info("DRY RUN: Audio already exists: %s", filename)
				} else {
					p.logger.Info("DRY RUN: Would generate audio: %s", filename)
				}
			}
			continue
		}

		time.Sleep(100 * time.Millisecond)

		// Calculate checksum before creating note
		mapper.UpdateChecksum(card)

		// Generate audio if TTS is enabled
		var audioData []byte
		var audioFilename string
		if ttsEnabled {
			audioData, audioFilename = p.generateAudioForCard(card)

			// Log audio attachment status
			if audioFilename != "" {
				if len(audioData) > 0 {
					p.logger.Info("Uploading and attaching audio '%s' to card '%s'", audioFilename, card.English)
				} else {
					p.logger.Info("Linking existing audio '%s' to card '%s'", audioFilename, card.English)
				}
			}

			// Add delay between TTS requests if configured (only if we generated new audio)
			if len(audioData) > 0 && p.config.TextToSpeech.RequestDelayMs > 0 {
				time.Sleep(time.Duration(p.config.TextToSpeech.RequestDelayMs) * time.Millisecond)
			}
		}

		// Create note in Anki with audio
		noteID, err := p.ankiClient.AddNote(
			p.config.AnkiDeck,
			anki.VocabSyncModelName,
			card,
			audioData,
			audioFilename,
		)
		if err != nil {
			// Log error but continue with remaining cards
			p.logger.Error("Failed to create card '%s' (row %d): %v", card.English, card.RowNumber, err)
			errors = append(errors, fmt.Errorf("row %d ('%s'): %w", card.RowNumber, card.English, err))
			continue
		}

		card.AnkiID = noteID
		p.logger.Info("Created card '%s' with Anki ID %d", card.English, noteID)
		p.logger.AddStat("created", 1)
		successCount++

		// Prepare updates for Anki ID and Checksum columns
		updates = append(updates, sheets.BuildAnkiIDAndChecksumUpdate(card.RowNumber, noteID, card.StoredChecksum)...)
	}

	// Return partial results with combined error if any failures occurred
	if len(errors) > 0 {
		p.logger.Info("Created %d/%d cards successfully, %d failed", successCount, len(cards), len(errors))
		return updates, fmt.Errorf("%s", util.FormatMultipleErrors(errors, 3))
	}

	return updates, nil
}

// updateExistingCards processes existing cards and updates them in Anki if changed.
// Returns a list of CellUpdates for writing updated checksums back to the Sheet.
// If errors occur, partial results are still returned along with a combined error.
func (p *Pusher) updateExistingCards(cards []*models.VocabCard, dryRun bool) ([]sheets.CellUpdate, error) {
	if len(cards) == 0 {
		return []sheets.CellUpdate{}, nil
	}

	updates := make([]sheets.CellUpdate, 0)
	var errors []error
	successCount := 0
	attemptedCount := 0

	// Check if TTS is enabled
	ttsEnabled := p.ttsClient != nil && p.config.TextToSpeech != nil && p.config.TextToSpeech.Enabled

	for _, card := range cards {
		// Check if card has changed
		if !mapper.HasChanged(card) {
			p.logger.AddStat("unchanged", 1)
			continue // Card unchanged, skip
		}

		attemptedCount++

		if dryRun {
			p.logger.Info("DRY RUN: Would update card '%s' (Anki ID %d)", card.English, card.AnkiID)
			continue
		}

		// Generate audio if TTS is enabled (same as for new cards)
		var audioData []byte
		var audioFilename string
		if ttsEnabled {
			audioData, audioFilename = p.generateAudioForCard(card)

			// Log audio status
			if audioFilename != "" {
				if len(audioData) > 0 {
					p.logger.Info("Uploading updated audio '%s' for card '%s'", audioFilename, card.English)
				} else {
					p.logger.Info("Linking audio '%s' to updated card '%s'", audioFilename, card.English)
				}
			}

			// Add delay between TTS requests if configured (only if we generated new audio)
			if len(audioData) > 0 && p.config.TextToSpeech.RequestDelayMs > 0 {
				time.Sleep(time.Duration(p.config.TextToSpeech.RequestDelayMs) * time.Millisecond)
			}
		}

		// Update note in Anki
		if err := p.ankiClient.UpdateNoteFields(card.AnkiID, card, audioData, audioFilename); err != nil {
			// Log error but continue with remaining cards
			p.logger.Error("Failed to update card '%s' (Anki ID %d, row %d): %v", card.English, card.AnkiID, card.RowNumber, err)
			errors = append(errors, fmt.Errorf("row %d ('%s', ID %d): %w", card.RowNumber, card.English, card.AnkiID, err))
			continue
		}

		p.logger.Info("Updated card '%s' (Anki ID %d)", card.English, card.AnkiID)
		p.logger.AddStat("updated", 1)
		successCount++

		// Update checksum
		mapper.UpdateChecksum(card)

		// Prepare update for Checksum column
		updates = append(updates, sheets.BuildChecksumOnlyUpdate(card.RowNumber, card.StoredChecksum)...)
	}

	// Return partial results with combined error if any failures occurred
	if len(errors) > 0 {
		p.logger.Info("Updated %d/%d cards successfully, %d failed", successCount, attemptedCount, len(errors))
		return updates, fmt.Errorf("%s", util.FormatMultipleErrors(errors, 3))
	}

	return updates, nil
}
