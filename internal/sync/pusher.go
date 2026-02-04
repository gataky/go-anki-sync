package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/yourusername/sync/internal/anki"
	"github.com/yourusername/sync/internal/mapper"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/pkg/models"
)

// Pusher orchestrates syncing data from Google Sheets to Anki.
type Pusher struct {
	sheetsClient SheetsClientInterface
	ankiClient   AnkiClientInterface
	config       *models.Config
	logger       *log.Logger
	ttsClient    TTSClientInterface
}

// NewPusher creates a new Pusher instance.
func NewPusher(
	sheetsClient SheetsClientInterface,
	ankiClient AnkiClientInterface,
	config *models.Config,
	logger *log.Logger,
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
	p.logger.Println("Starting push sync (Sheets → Anki)")

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

	p.logger.Printf("Loaded %d cards from sheet", len(cards))

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

	p.logger.Printf("Found %d new cards, %d existing cards", len(newCards), len(existingCards))

	// Ensure deck exists
	if !dryRun {
		if err := p.ankiClient.CreateDeck(p.config.AnkiDeck); err != nil {
			return fmt.Errorf("failed to create deck: %w", err)
		}
		p.logger.Printf("Ensured deck '%s' exists", p.config.AnkiDeck)
	}

	// Ensure VocabSync note type exists
	if !dryRun {
		if err := p.ankiClient.CreateNoteType(anki.VocabSyncModelName); err != nil {
			return fmt.Errorf("failed to create note type: %w", err)
		}
		p.logger.Printf("Ensured note type '%s' exists", anki.VocabSyncModelName)
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
		p.logger.Printf("Wrote %d updates to sheet", len(allUpdates))
	}

	// Log summary
	createdCount := len(newCardUpdates) / 2 // Each new card generates 2 updates (Anki ID + Checksum)
	updatedCount := len(existingCardUpdates)
	unchangedCount := len(existingCards) - updatedCount

	if dryRun {
		p.logger.Printf("DRY RUN: Would create %d new cards, would update %d cards, %d unchanged",
			createdCount, updatedCount, unchangedCount)
	} else {
		p.logger.Printf("Push complete: Created %d new cards, updated %d cards, %d unchanged",
			createdCount, updatedCount, unchangedCount)
	}

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

// generateAudioForCard generates audio for a vocabulary card using TTS.
// Returns (audioData, filename) where:
// - audioData is non-nil only if new audio was generated (needs uploading)
// - filename is always set if audio should be attached (even if it already exists)
// All errors are logged but do not block card creation.
func (p *Pusher) generateAudioForCard(card *models.VocabCard) ([]byte, string) {
	// Validate Greek text
	greekText := strings.TrimSpace(card.Greek)
	if greekText == "" {
		p.logger.Printf("WARNING: Skipping audio generation for card '%s' (row %d): Greek text is empty", card.English, card.RowNumber)
		return nil, ""
	}

	// Build filename
	filename := fmt.Sprintf("%s.mp3", card.Greek)

	// Check if audio file already exists
	if p.audioFileExists(filename) {
		p.logger.Printf("Audio already exists for '%s', linking to card", card.Greek)
		// Return nil data (no upload needed) but return filename to link it
		return nil, filename
	}

	// Generate audio using TTS
	audioData, err := p.ttsClient.GenerateAudio(greekText)
	if err != nil {
		p.logger.Printf("ERROR: Failed to generate audio for '%s': %v", card.Greek, err)
		return nil, ""
	}

	p.logger.Printf("Generated audio for '%s' (%d bytes)", card.Greek, len(audioData))
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
			p.logger.Printf("DRY RUN: Would create card '%s' (%s)", card.English, card.Greek)

			// Check if audio would be generated
			if ttsEnabled && strings.TrimSpace(card.Greek) != "" {
				filename := fmt.Sprintf("%s.mp3", card.Greek)
				if p.audioFileExists(filename) {
					p.logger.Printf("DRY RUN: Audio already exists: %s", filename)
				} else {
					p.logger.Printf("DRY RUN: Would generate audio: %s", filename)
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
					p.logger.Printf("Uploading and attaching audio '%s' to card '%s'", audioFilename, card.English)
				} else {
					p.logger.Printf("Linking existing audio '%s' to card '%s'", audioFilename, card.English)
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
			p.logger.Printf("ERROR: Failed to create card '%s' (row %d): %v", card.English, card.RowNumber, err)
			errors = append(errors, fmt.Errorf("row %d ('%s'): %w", card.RowNumber, card.English, err))
			continue
		}

		card.AnkiID = noteID
		p.logger.Printf("Created card '%s' with Anki ID %d", card.English, noteID)
		successCount++

		// Prepare updates for Anki ID and Checksum columns
		// Note: CellUpdate.Row is 1-indexed excluding header, so subtract 1 from sheet row number
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber - 1,
			Column: "A", // Anki ID column
			Value:  noteID,
		})
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber - 1,
			Column: "B", // Checksum column
			Value:  card.StoredChecksum,
		})
	}

	// Return partial results with combined error if any failures occurred
	if len(errors) > 0 {
		p.logger.Printf("Created %d/%d cards successfully, %d failed", successCount, len(cards), len(errors))
		var errMsg string
		if len(errors) == 1 {
			errMsg = errors[0].Error()
		} else {
			errMsg = fmt.Sprintf("%d cards failed: ", len(errors))
			for i, err := range errors {
				if i > 0 {
					errMsg += "; "
				}
				errMsg += err.Error()
				if i >= 2 && len(errors) > 3 {
					errMsg += fmt.Sprintf("; and %d more", len(errors)-3)
					break
				}
			}
		}
		return updates, fmt.Errorf("%s", errMsg)
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

	for _, card := range cards {
		// Check if card has changed
		if !mapper.HasChanged(card) {
			continue // Card unchanged, skip
		}

		attemptedCount++

		if dryRun {
			p.logger.Printf("DRY RUN: Would update card '%s' (Anki ID %d)", card.English, card.AnkiID)
			continue
		}

		// Update note in Anki
		if err := p.ankiClient.UpdateNoteFields(card.AnkiID, card); err != nil {
			// Log error but continue with remaining cards
			p.logger.Printf("ERROR: Failed to update card '%s' (Anki ID %d, row %d): %v", card.English, card.AnkiID, card.RowNumber, err)
			errors = append(errors, fmt.Errorf("row %d ('%s', ID %d): %w", card.RowNumber, card.English, card.AnkiID, err))
			continue
		}

		p.logger.Printf("Updated card '%s' (Anki ID %d)", card.English, card.AnkiID)
		successCount++

		// Update checksum
		mapper.UpdateChecksum(card)

		// Prepare update for Checksum column
		// Note: CellUpdate.Row is 1-indexed excluding header, so subtract 1 from sheet row number
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber - 1,
			Column: "B", // Checksum column
			Value:  card.StoredChecksum,
		})
	}

	// Return partial results with combined error if any failures occurred
	if len(errors) > 0 {
		p.logger.Printf("Updated %d/%d cards successfully, %d failed", successCount, attemptedCount, len(errors))
		var errMsg string
		if len(errors) == 1 {
			errMsg = errors[0].Error()
		} else {
			errMsg = fmt.Sprintf("%d cards failed: ", len(errors))
			for i, err := range errors {
				if i > 0 {
					errMsg += "; "
				}
				errMsg += err.Error()
				if i >= 2 && len(errors) > 3 {
					errMsg += fmt.Sprintf("; and %d more", len(errors)-3)
					break
				}
			}
		}
		return updates, fmt.Errorf("%s", errMsg)
	}

	return updates, nil
}
