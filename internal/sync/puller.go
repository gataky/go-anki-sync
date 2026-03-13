package sync

import (
	"fmt"
	"time"

	"github.com/yourusername/sync/internal/logging"
	"github.com/yourusername/sync/internal/mapper"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/pkg/models"
)

// PullerAnkiClient extends AnkiClientInterface with additional methods needed for pulling.
type PullerAnkiClient interface {
	AnkiClientInterface
	FindModifiedNotes(deckName string, sinceTimestamp time.Time) ([]int64, error)
	GetNotesInfo(noteIDs []int64) ([]*models.VocabCard, error)
}

// StateManager interface for loading and saving sync state.
type StateManager interface {
	LoadState(path string) (*models.SyncState, error)
	SaveState(state *models.SyncState, path string) error
	GetDefaultStatePath() string
}

// Puller orchestrates syncing data from Anki to Google Sheets.
type Puller struct {
	sheetsClient SheetsClientInterface
	ankiClient   PullerAnkiClient
	config       *models.Config
	state        *models.SyncState
	stateManager StateManager
	logger       *logging.SyncLogger
}

// NewPuller creates a new Puller instance.
func NewPuller(
	sheetsClient SheetsClientInterface,
	ankiClient PullerAnkiClient,
	config *models.Config,
	state *models.SyncState,
	stateManager StateManager,
	logger *logging.SyncLogger,
) *Puller {
	return &Puller{
		sheetsClient: sheetsClient,
		ankiClient:   ankiClient,
		config:       config,
		state:        state,
		stateManager: stateManager,
		logger:       logger,
	}
}

// Pull executes the pull sync from Anki to Google Sheets.
// If dryRun is true, no changes are made to Sheet or state.
func (p *Puller) Pull(dryRun bool) error {
	p.logger.Info("Starting pull sync (Anki → Sheets)")

	// Load last pull timestamp from state
	lastPullTime := p.state.LastPullTimestamp
	if lastPullTime.IsZero() {
		lastPullTime = time.Now().Add(-24 * 365 * time.Hour) // Default: 1 year ago
		p.logger.Info("No previous pull timestamp, using default (1 year ago)")
	} else {
		p.logger.Info("Last pull timestamp: %s", lastPullTime.Format(time.RFC3339))
	}

	// Query Anki for modified notes since last timestamp
	noteIDs, err := p.ankiClient.FindModifiedNotes(p.config.AnkiDeck, lastPullTime)
	if err != nil {
		return fmt.Errorf("failed to find modified notes: %w", err)
	}

	if len(noteIDs) == 0 {
		p.logger.Info("No changes in Anki since last pull")
		return nil
	}

	p.logger.Info("Found %d modified notes in Anki", len(noteIDs))

	// Fetch full note details
	ankiCards, err := p.ankiClient.GetNotesInfo(noteIDs)
	if err != nil {
		return fmt.Errorf("failed to get notes info: %w", err)
	}

	// Read current Sheet data to build map of AnkiID -> RowNumber
	rows, err := p.sheetsClient.ReadSheet(p.config.GoogleSheetID, p.config.SheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet: %w", err)
	}

	headers, err := p.sheetsClient.ParseHeaders(rows)
	if err != nil {
		return fmt.Errorf("failed to parse headers: %w", err)
	}

	// Build map of AnkiID -> RowNumber
	ankiIDToRow := make(map[int64]int)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		card, err := mapper.RowToCard(row, headers, i+1)
		if err != nil {
			// Skip invalid rows
			continue
		}

		if card.AnkiID != 0 {
			ankiIDToRow[card.AnkiID] = card.RowNumber
		}
	}

	p.logger.Info("Built map of %d Anki IDs to row numbers", len(ankiIDToRow))

	// Build CellUpdate list for modified notes
	updates := make([]sheets.CellUpdate, 0)

	for _, ankiCard := range ankiCards {
		// Find corresponding row in Sheet by AnkiID
		rowNumber, exists := ankiIDToRow[ankiCard.AnkiID]
		if !exists {
			// Note not found in Sheet (may have been deleted)
			p.logger.AddStat("skipped", 1)
			continue
		}

		// Update checksum
		mapper.UpdateChecksum(ankiCard)

		// Build updates for content fields and checksum
		// Assuming standard column layout: A=AnkiID, B=Checksum, C=English, D=Greek, E=PartOfSpeech, etc.
		// Note: CellUpdate.Row is 1-indexed excluding header, so subtract 1 from sheet row number
		updates = append(updates,
			sheets.CellUpdate{Row: rowNumber - 1, Column: "B", Value: ankiCard.StoredChecksum},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "C", Value: ankiCard.English},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "D", Value: ankiCard.Greek},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "E", Value: ankiCard.PartOfSpeech},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "F", Value: ankiCard.Attributes},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "G", Value: ankiCard.Examples},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "H", Value: ankiCard.Tag},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "I", Value: ankiCard.SubTag1},
			sheets.CellUpdate{Row: rowNumber - 1, Column: "J", Value: ankiCard.SubTag2},
		)
		p.logger.AddStat("updated", 1)
	}

	// Write updates to Sheet
	if !dryRun && len(updates) > 0 {
		if err := p.sheetsClient.BatchUpdateCells(
			p.config.GoogleSheetID,
			p.config.SheetName,
			updates,
		); err != nil {
			return fmt.Errorf("failed to write updates to sheet: %w", err)
		}
		p.logger.Info("Wrote %d cell updates to sheet", len(updates))

		// Update state with new LastPullTimestamp
		p.state.LastPullTimestamp = time.Now()
		statePath := p.stateManager.GetDefaultStatePath()
		if err := p.stateManager.SaveState(p.state, statePath); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
		p.logger.Info("Updated state with new pull timestamp")
	}

	// Print summary
	p.logger.PrintSummary("Pull")

	return nil
}
