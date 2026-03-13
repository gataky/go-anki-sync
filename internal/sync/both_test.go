package sync

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/pkg/models"
)

func TestNewBothSyncer(t *testing.T) {
	sheetsClient := &mockSheetsClient{}
	ankiClient := &mockPullerAnkiClient{}
	config := &models.Config{
		GoogleSheetID: "test-sheet-id",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	assert.NotNil(t, syncer)
	assert.NotNil(t, syncer.pusher)
	assert.NotNil(t, syncer.puller)
	assert.Equal(t, config, syncer.config)
}

func TestDetectConflicts_NoConflicts(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)
	syncer := &BothSyncer{logger: logger}

	// Sheet card unchanged
	sheetCard := &models.VocabCard{
		AnkiID:       1111111111111,
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
	}
	mapper.UpdateChecksum(sheetCard)

	// Anki card modified (but Sheet card hasn't changed)
	ankiCard := &models.VocabCard{
		AnkiID:       2222222222222, // Different ID
		English:      "different",
		Greek:        "διαφορετικό",
		PartOfSpeech: "Adjective",
		ModifiedAt:   time.Now(),
	}

	conflicts := syncer.detectConflicts([]*models.VocabCard{sheetCard}, []*models.VocabCard{ankiCard})
	assert.Len(t, conflicts, 0)
}

func TestDetectConflicts_WithConflict(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)
	syncer := &BothSyncer{logger: logger}

	// Sheet card modified (wrong checksum)
	sheetCard := &models.VocabCard{
		AnkiID:         1111111111111,
		StoredChecksum: "wrong-checksum",
		English:        "hello",
		Greek:          "γεια modified",
		PartOfSpeech:   "Interjection",
	}

	// Anki card also modified (same ID)
	ankiCard := &models.VocabCard{
		AnkiID:       1111111111111,
		English:      "hello",
		Greek:        "γεια different",
		PartOfSpeech: "Interjection",
		ModifiedAt:   time.Now(),
	}

	conflicts := syncer.detectConflicts([]*models.VocabCard{sheetCard}, []*models.VocabCard{ankiCard})
	assert.Len(t, conflicts, 1)
	assert.Equal(t, int64(1111111111111), conflicts[0].AnkiID)
	assert.Equal(t, sheetCard, conflicts[0].SheetCard)
	assert.Equal(t, ankiCard, conflicts[0].AnkiCard)
}

func TestResolveConflicts_AnkiWins(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)
	syncer := &BothSyncer{logger: logger}

	conflict := &Conflict{
		AnkiID: 1111111111111,
		SheetCard: &models.VocabCard{
			English: "hello",
			Greek:   "γεια sheet",
		},
		AnkiCard: &models.VocabCard{
			English:    "hello",
			Greek:      "γεια anki",
			ModifiedAt: time.Now(),
		},
	}

	conflicts := []*Conflict{conflict}
	syncer.resolveConflicts(conflicts)

	assert.Equal(t, "Anki", conflict.Winner)
	assert.Contains(t, conflict.Resolution, "Anki modified at")
}

func TestResolveConflicts_SheetWins(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)
	syncer := &BothSyncer{logger: logger}

	conflict := &Conflict{
		AnkiID: 1111111111111,
		SheetCard: &models.VocabCard{
			English: "hello",
			Greek:   "γεια sheet",
		},
		AnkiCard: &models.VocabCard{
			English:    "hello",
			Greek:      "γεια anki",
			ModifiedAt: time.Time{}, // Zero time
		},
	}

	conflicts := []*Conflict{conflict}
	syncer.resolveConflicts(conflicts)

	assert.Equal(t, "Sheet", conflict.Winner)
	assert.Contains(t, conflict.Resolution, "Sheet version preferred")
}

func TestSync_NewCardsOnly(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "hello", "γεια", "Interjection"}, // New card
		{nil, "", "house", "σπίτι", "Noun"},       // New card
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{}, // No Anki modifications
		notesInfo:       []*models.VocabCard{},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := syncer.Sync(false)
	require.NoError(t, err)

	// Should have created 2 cards
	// Each card generates 2 updates (Anki ID + checksum)
	assert.GreaterOrEqual(t, len(sheetsClient.batchUpdates), 4)
}

func TestSync_PullChangesOnly(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Existing card in Sheet with correct checksum
	existingCard := &models.VocabCard{
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
	}
	mapper.UpdateChecksum(existingCard)

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1111111111111), existingCard.StoredChecksum, "hello", "γεια", "Interjection"},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	// Modified card in Anki
	modifiedAnkiCard := &models.VocabCard{
		AnkiID:       1111111111111,
		English:      "hello",
		Greek:        "γεια σου", // Modified
		PartOfSpeech: "Interjection",
		ModifiedAt:   time.Now(),
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{1111111111111},
		notesInfo:       []*models.VocabCard{modifiedAnkiCard},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := syncer.Sync(false)
	require.NoError(t, err)

	// Should have pulled changes from Anki to Sheet
	assert.Greater(t, len(sheetsClient.batchUpdates), 0)

	// Verify Greek field was updated
	foundUpdate := false
	for _, update := range sheetsClient.batchUpdates {
		if update.Column == "D" && update.Value == "γεια σου" {
			foundUpdate = true
			break
		}
	}
	assert.True(t, foundUpdate, "Should have updated Greek field")
}

func TestSync_WithConflict(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Sheet card with wrong checksum (modified in Sheet)
	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1111111111111), "wrong-checksum", "hello", "γεια sheet", "Interjection"},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	// Anki card also modified
	modifiedAnkiCard := &models.VocabCard{
		AnkiID:       1111111111111,
		English:      "hello",
		Greek:        "γεια anki", // Different modification
		PartOfSpeech: "Interjection",
		ModifiedAt:   time.Now(),
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{1111111111111},
		notesInfo:       []*models.VocabCard{modifiedAnkiCard},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := syncer.Sync(false)
	require.NoError(t, err)

	// Conflict should be resolved (Anki wins because it has modification time)
	// Sheet should be updated with Anki's value
	foundAnkiValue := false
	for _, update := range sheetsClient.batchUpdates {
		if update.Column == "D" && update.Value == "γεια anki" {
			foundAnkiValue = true
			break
		}
	}
	assert.True(t, foundAnkiValue, "Should have resolved conflict with Anki's value")
}

func TestSync_DryRun(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "hello", "γεια", "Interjection"}, // New card
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{},
		notesInfo:       []*models.VocabCard{},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	// Execute dry run
	err := syncer.Sync(true)
	require.NoError(t, err)

	// No changes should be made
	assert.Len(t, sheetsClient.batchUpdates, 0)
	assert.Nil(t, stateManager.savedState)
}

func TestSync_MixedOperations(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Create cards with different states
	unchangedCard := &models.VocabCard{
		English:      "unchanged",
		Greek:        "αμετάβλητο",
		PartOfSpeech: "Adjective",
	}
	mapper.UpdateChecksum(unchangedCard)

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "new", "νέο", "Adjective"},                                                       // New card
		{float64(2222222222222), "wrong", "changed", "αλλαγμένο", "Adjective"},                   // Changed in Sheet
		{float64(3333333333333), unchangedCard.StoredChecksum, "unchanged", "αμετάβλητο", "Adjective"}, // Unchanged
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	// Anki has modified the unchanged card
	modifiedInAnki := &models.VocabCard{
		AnkiID:       3333333333333,
		English:      "unchanged",
		Greek:        "αμετάβλητο modified",
		PartOfSpeech: "Adjective",
		ModifiedAt:   time.Now(),
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{3333333333333},
		notesInfo:       []*models.VocabCard{modifiedInAnki},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	syncer := NewBothSyncer(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := syncer.Sync(false)
	require.NoError(t, err)

	// Should have updates for all operations
	assert.Greater(t, len(sheetsClient.batchUpdates), 0)
}
