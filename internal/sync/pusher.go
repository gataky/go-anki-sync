package sync

import (
	"fmt"
	"log"

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
}

// NewPusher creates a new Pusher instance.
func NewPusher(
	sheetsClient SheetsClientInterface,
	ankiClient AnkiClientInterface,
	config *models.Config,
	logger *log.Logger,
) *Pusher {
	return &Pusher{
		sheetsClient: sheetsClient,
		ankiClient:   ankiClient,
		config:       config,
		logger:       logger,
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

	// Process new cards
	newCardUpdates, err := p.createNewCards(newCards, dryRun)
	if err != nil {
		return fmt.Errorf("failed to create new cards: %w", err)
	}

	// Process existing cards
	existingCardUpdates, err := p.updateExistingCards(existingCards, dryRun)
	if err != nil {
		return fmt.Errorf("failed to update existing cards: %w", err)
	}

	// Write updates to Sheet
	if !dryRun && (len(newCardUpdates) > 0 || len(existingCardUpdates) > 0) {
		allUpdates := append(newCardUpdates, existingCardUpdates...)
		if err := p.sheetsClient.BatchUpdateCells(
			p.config.GoogleSheetID,
			p.config.SheetName,
			allUpdates,
		); err != nil {
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

	return nil
}

// createNewCards processes new cards (AnkiID == 0) and creates them in Anki.
// Returns a list of CellUpdates for writing Anki IDs and checksums back to the Sheet.
func (p *Pusher) createNewCards(cards []*models.VocabCard, dryRun bool) ([]sheets.CellUpdate, error) {
	if len(cards) == 0 {
		return []sheets.CellUpdate{}, nil
	}

	updates := make([]sheets.CellUpdate, 0, len(cards)*2)

	for _, card := range cards {
		if dryRun {
			p.logger.Printf("DRY RUN: Would create card '%s' (%s)", card.English, card.Greek)
			continue
		}

		// Calculate checksum before creating note
		mapper.UpdateChecksum(card)

		// Create note in Anki
		noteID, err := p.ankiClient.AddNote(
			p.config.AnkiDeck,
			anki.VocabSyncModelName,
			card,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add note for '%s': %w", card.English, err)
		}

		card.AnkiID = noteID
		p.logger.Printf("Created card '%s' with Anki ID %d", card.English, noteID)

		// Prepare updates for Anki ID and Checksum columns
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber,
			Column: "A", // Anki ID column
			Value:  noteID,
		})
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber,
			Column: "B", // Checksum column
			Value:  card.StoredChecksum,
		})
	}

	return updates, nil
}

// updateExistingCards processes existing cards and updates them in Anki if changed.
// Returns a list of CellUpdates for writing updated checksums back to the Sheet.
func (p *Pusher) updateExistingCards(cards []*models.VocabCard, dryRun bool) ([]sheets.CellUpdate, error) {
	if len(cards) == 0 {
		return []sheets.CellUpdate{}, nil
	}

	updates := make([]sheets.CellUpdate, 0)

	for _, card := range cards {
		// Check if card has changed
		if !mapper.HasChanged(card) {
			continue // Card unchanged, skip
		}

		if dryRun {
			p.logger.Printf("DRY RUN: Would update card '%s' (Anki ID %d)", card.English, card.AnkiID)
			continue
		}

		// Update note in Anki
		if err := p.ankiClient.UpdateNoteFields(card.AnkiID, card); err != nil {
			return nil, fmt.Errorf("failed to update note %d ('%s'): %w", card.AnkiID, card.English, err)
		}

		p.logger.Printf("Updated card '%s' (Anki ID %d)", card.English, card.AnkiID)

		// Update checksum
		mapper.UpdateChecksum(card)

		// Prepare update for Checksum column
		updates = append(updates, sheets.CellUpdate{
			Row:    card.RowNumber,
			Column: "B", // Checksum column
			Value:  card.StoredChecksum,
		})
	}

	return updates, nil
}
