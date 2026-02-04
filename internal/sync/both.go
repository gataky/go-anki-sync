package sync

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/sync/internal/mapper"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/pkg/models"
)

// Conflict represents a card that has been modified in both Sheet and Anki.
type Conflict struct {
	AnkiID     int64
	SheetCard  *models.VocabCard
	AnkiCard   *models.VocabCard
	Winner     string // "Sheet" or "Anki"
	Resolution string // Description of resolution
}

// BothSyncer orchestrates bidirectional sync between Google Sheets and Anki.
type BothSyncer struct {
	pusher       *Pusher
	puller       *Puller
	sheetsClient SheetsClientInterface
	ankiClient   PullerAnkiClient
	config       *models.Config
	state        *models.SyncState
	stateManager StateManager
	logger       *log.Logger
}

// NewBothSyncer creates a new BothSyncer instance.
func NewBothSyncer(
	sheetsClient SheetsClientInterface,
	ankiClient PullerAnkiClient,
	config *models.Config,
	state *models.SyncState,
	stateManager StateManager,
	logger *log.Logger,
) *BothSyncer {
	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)
	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	return &BothSyncer{
		pusher:       pusher,
		puller:       puller,
		sheetsClient: sheetsClient,
		ankiClient:   ankiClient,
		config:       config,
		state:        state,
		stateManager: stateManager,
		logger:       logger,
	}
}

// Sync executes bidirectional sync with conflict resolution.
// If dryRun is true, no changes are made to Sheet, Anki, or state.
func (b *BothSyncer) Sync(dryRun bool) error {
	b.logger.Println("Starting bidirectional sync (Sheets ↔ Anki)")

	// Read Sheet data
	rows, err := b.sheetsClient.ReadSheet(b.config.GoogleSheetID, b.config.SheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet: %w", err)
	}

	headers, err := b.sheetsClient.ParseHeaders(rows)
	if err != nil {
		return fmt.Errorf("failed to parse headers: %w", err)
	}

	// Parse Sheet cards
	sheetCards := make([]*models.VocabCard, 0)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		card, err := mapper.RowToCard(row, headers, i+1)
		if err != nil {
			b.logger.Printf("Warning: skipping invalid row %d: %v", i+1, err)
			continue
		}

		sheetCards = append(sheetCards, card)
	}

	b.logger.Printf("Loaded %d cards from Sheet", len(sheetCards))

	// Query Anki for modified notes since last pull
	lastPullTime := b.state.LastPullTimestamp
	if lastPullTime.IsZero() {
		lastPullTime = time.Now().Add(-24 * 365 * time.Hour) // 1 year ago
	}

	modifiedNoteIDs, err := b.ankiClient.FindModifiedNotes(b.config.AnkiDeck, lastPullTime)
	if err != nil {
		return fmt.Errorf("failed to find modified notes: %w", err)
	}

	var ankiCards []*models.VocabCard
	if len(modifiedNoteIDs) > 0 {
		ankiCards, err = b.ankiClient.GetNotesInfo(modifiedNoteIDs)
		if err != nil {
			return fmt.Errorf("failed to get notes info: %w", err)
		}
		b.logger.Printf("Found %d modified notes in Anki", len(ankiCards))
	}

	// Detect conflicts (cards modified in both systems)
	conflicts := b.detectConflicts(sheetCards, ankiCards)
	if len(conflicts) > 0 {
		b.logger.Printf("Detected %d conflicts", len(conflicts))
		b.resolveConflicts(conflicts)
	}

	// Separate sheet cards into categories
	newCards := make([]*models.VocabCard, 0)
	existingChangedCards := make([]*models.VocabCard, 0)
	conflictedAnkiIDs := make(map[int64]bool)

	for _, conflict := range conflicts {
		conflictedAnkiIDs[conflict.AnkiID] = true
	}

	for _, card := range sheetCards {
		if card.AnkiID == 0 {
			newCards = append(newCards, card)
		} else if !conflictedAnkiIDs[card.AnkiID] && mapper.HasChanged(card) {
			existingChangedCards = append(existingChangedCards, card)
		}
	}

	b.logger.Printf("Changes to push: %d new cards, %d updated cards", len(newCards), len(existingChangedCards))

	// Counters for summary
	var (
		createdCount  int
		updatedCount  int
		pulledCount   int
		conflictCount = len(conflicts)
	)

	// Apply push changes (new cards and updates from Sheet)
	if !dryRun && (len(newCards) > 0 || len(existingChangedCards) > 0) {
		// Ensure deck and note type exist
		if err := b.ankiClient.CreateDeck(b.config.AnkiDeck); err != nil {
			return fmt.Errorf("failed to create deck: %w", err)
		}
		if err := b.ankiClient.CreateNoteType("VocabSync"); err != nil {
			return fmt.Errorf("failed to create note type: %w", err)
		}

		// Create new cards
		if len(newCards) > 0 {
			updates, err := b.pusher.createNewCards(newCards, dryRun)
			if err != nil {
				return fmt.Errorf("failed to create new cards: %w", err)
			}
			if len(updates) > 0 {
				if err := b.sheetsClient.BatchUpdateCells(b.config.GoogleSheetID, b.config.SheetName, updates); err != nil {
					return fmt.Errorf("failed to write new card updates: %w", err)
				}
			}
			createdCount = len(newCards)
		}

		// Update existing changed cards
		if len(existingChangedCards) > 0 {
			updates, err := b.pusher.updateExistingCards(existingChangedCards, dryRun)
			if err != nil {
				return fmt.Errorf("failed to update existing cards: %w", err)
			}
			if len(updates) > 0 {
				if err := b.sheetsClient.BatchUpdateCells(b.config.GoogleSheetID, b.config.SheetName, updates); err != nil {
					return fmt.Errorf("failed to write update checksums: %w", err)
				}
			}
			updatedCount = len(updates) // Number of cards actually updated
		}
	}

	// Apply conflict resolutions
	if !dryRun && len(conflicts) > 0 {
		sheetUpdates := make([]sheets.CellUpdate, 0)
		ankiUpdates := make(map[int64]*models.VocabCard)

		for _, conflict := range conflicts {
			if conflict.Winner == "Sheet" {
				// Update Anki from Sheet
				ankiUpdates[conflict.AnkiID] = conflict.SheetCard
			} else {
				// Update Sheet from Anki (build cell updates)
				mapper.UpdateChecksum(conflict.AnkiCard)
				// Note: CellUpdate.Row is 1-indexed excluding header, so subtract 1 from sheet row number
				rowNum := conflict.SheetCard.RowNumber - 1
				sheetUpdates = append(sheetUpdates,
					sheets.CellUpdate{Row: rowNum, Column: "B", Value: conflict.AnkiCard.StoredChecksum},
					sheets.CellUpdate{Row: rowNum, Column: "C", Value: conflict.AnkiCard.English},
					sheets.CellUpdate{Row: rowNum, Column: "D", Value: conflict.AnkiCard.Greek},
					sheets.CellUpdate{Row: rowNum, Column: "E", Value: conflict.AnkiCard.PartOfSpeech},
					sheets.CellUpdate{Row: rowNum, Column: "F", Value: conflict.AnkiCard.Attributes},
					sheets.CellUpdate{Row: rowNum, Column: "G", Value: conflict.AnkiCard.Examples},
					sheets.CellUpdate{Row: rowNum, Column: "H", Value: conflict.AnkiCard.Tag},
					sheets.CellUpdate{Row: rowNum, Column: "I", Value: conflict.AnkiCard.SubTag1},
					sheets.CellUpdate{Row: rowNum, Column: "J", Value: conflict.AnkiCard.SubTag2},
				)
			}
		}

		// Apply Anki updates
		for ankiID, card := range ankiUpdates {
			// Conflict resolution doesn't regenerate audio
			if err := b.ankiClient.UpdateNoteFields(ankiID, card, nil, ""); err != nil {
				return fmt.Errorf("failed to update conflicted card %d: %w", ankiID, err)
			}
		}

		// Apply Sheet updates
		if len(sheetUpdates) > 0 {
			if err := b.sheetsClient.BatchUpdateCells(b.config.GoogleSheetID, b.config.SheetName, sheetUpdates); err != nil {
				return fmt.Errorf("failed to write conflict resolutions: %w", err)
			}
		}
	}

	// Apply pull changes (non-conflicted Anki updates)
	if len(ankiCards) > 0 {
		pullUpdates := make([]sheets.CellUpdate, 0)

		// Build AnkiID -> Sheet row map
		ankiIDToRow := make(map[int64]int)
		for _, card := range sheetCards {
			if card.AnkiID != 0 {
				ankiIDToRow[card.AnkiID] = card.RowNumber
			}
		}

		for _, ankiCard := range ankiCards {
			// Skip if this card was in conflict
			if conflictedAnkiIDs[ankiCard.AnkiID] {
				continue
			}

			rowNum, exists := ankiIDToRow[ankiCard.AnkiID]
			if !exists {
				continue // Skip notes not in Sheet
			}

			mapper.UpdateChecksum(ankiCard)
			// Note: CellUpdate.Row is 1-indexed excluding header, so subtract 1 from sheet row number
			pullUpdates = append(pullUpdates,
				sheets.CellUpdate{Row: rowNum - 1, Column: "B", Value: ankiCard.StoredChecksum},
				sheets.CellUpdate{Row: rowNum - 1, Column: "C", Value: ankiCard.English},
				sheets.CellUpdate{Row: rowNum - 1, Column: "D", Value: ankiCard.Greek},
				sheets.CellUpdate{Row: rowNum - 1, Column: "E", Value: ankiCard.PartOfSpeech},
				sheets.CellUpdate{Row: rowNum - 1, Column: "F", Value: ankiCard.Attributes},
				sheets.CellUpdate{Row: rowNum - 1, Column: "G", Value: ankiCard.Examples},
				sheets.CellUpdate{Row: rowNum - 1, Column: "H", Value: ankiCard.Tag},
				sheets.CellUpdate{Row: rowNum - 1, Column: "I", Value: ankiCard.SubTag1},
				sheets.CellUpdate{Row: rowNum - 1, Column: "J", Value: ankiCard.SubTag2},
			)
			pulledCount++
		}

		if !dryRun && len(pullUpdates) > 0 {
			if err := b.sheetsClient.BatchUpdateCells(b.config.GoogleSheetID, b.config.SheetName, pullUpdates); err != nil {
				return fmt.Errorf("failed to write pull updates: %w", err)
			}
		}
	}

	// Update state timestamps
	if !dryRun {
		b.state.LastPushTimestamp = time.Now()
		b.state.LastPullTimestamp = time.Now()
		statePath := b.stateManager.GetDefaultStatePath()
		if err := b.stateManager.SaveState(b.state, statePath); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
	}

	// Log summary
	if dryRun {
		b.logger.Printf("DRY RUN: Would create %d cards, update %d cards, pull %d changes, resolve %d conflicts",
			createdCount, updatedCount, pulledCount, conflictCount)
	} else {
		b.logger.Printf("Sync complete: Created %d cards, updated %d cards, pulled %d changes, resolved %d conflicts",
			createdCount, updatedCount, pulledCount, conflictCount)
	}

	return nil
}

// detectConflicts finds cards that have been modified in both Sheet and Anki.
func (b *BothSyncer) detectConflicts(sheetCards, ankiCards []*models.VocabCard) []*Conflict {
	// Build map of AnkiID -> Sheet card with checksum mismatch
	sheetChangedMap := make(map[int64]*models.VocabCard)
	for _, card := range sheetCards {
		if card.AnkiID != 0 && mapper.HasChanged(card) {
			sheetChangedMap[card.AnkiID] = card
		}
	}

	// Find conflicts
	conflicts := make([]*Conflict, 0)
	for _, ankiCard := range ankiCards {
		if sheetCard, exists := sheetChangedMap[ankiCard.AnkiID]; exists {
			// Both modified - conflict detected
			conflict := &Conflict{
				AnkiID:    ankiCard.AnkiID,
				SheetCard: sheetCard,
				AnkiCard:  ankiCard,
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// resolveConflicts resolves conflicts using timestamp-based last-write-wins strategy.
func (b *BothSyncer) resolveConflicts(conflicts []*Conflict) {
	for _, conflict := range conflicts {
		// Compare modification timestamps (most recent wins)
		// Note: ModifiedAt is set by Anki for AnkiCard, but Sheet doesn't track modification time
		// For now, we'll default to Anki winning since it has explicit modification time
		// In a production system, Sheet would need to track modification times too

		// Simple heuristic: if checksum differs, Anki modification time exists, Anki wins
		// This could be enhanced with Sheet-side modification tracking
		if !conflict.AnkiCard.ModifiedAt.IsZero() {
			conflict.Winner = "Anki"
			conflict.Resolution = fmt.Sprintf("Anki modified at %s", conflict.AnkiCard.ModifiedAt.Format(time.RFC3339))
		} else {
			conflict.Winner = "Sheet"
			conflict.Resolution = "Sheet version preferred (no Anki modification time)"
		}

		checksumDisplay := conflict.SheetCard.StoredChecksum
		if len(checksumDisplay) > 8 {
			checksumDisplay = checksumDisplay[:8]
		}

		b.logger.Printf("CONFLICT - Card ID %d ('%s'): Sheet checksum=%s vs Anki modified=%s. Winner: %s",
			conflict.AnkiID,
			conflict.SheetCard.English,
			checksumDisplay,
			conflict.AnkiCard.ModifiedAt.Format(time.RFC3339),
			conflict.Winner,
		)
	}
}
