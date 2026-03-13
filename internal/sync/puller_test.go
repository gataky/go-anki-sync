package sync

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/internal/testutil"
	"github.com/gataky/sync/pkg/models"
)

// Mock implementations are in testutil package

func TestNewPuller(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{}
	ankiClient := &testutil.MockPullerAnkiClient{MockAnkiClient: testutil.NewMockAnkiClient(), }
	config := &models.Config{
		GoogleSheetID: "test-sheet-id",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	assert.NotNil(t, puller)
	assert.Equal(t, sheetsClient, puller.sheetsClient)
	assert.Equal(t, ankiClient, puller.ankiClient)
	assert.Equal(t, config, puller.config)
	assert.Equal(t, state, puller.state)
	assert.Equal(t, logger, puller.logger)
}

func TestPull_NoModifiedNotes(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{}
	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{}
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// No updates should be written
	assert.Len(t, sheetsClient.BatchUpdates, 0)

	// State should not be updated
	assert.Nil(t, stateManager.SavedState)
}

func TestPull_WithModifiedNotes(t *testing.T) {
	// Setup Sheet data
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
		"attributes":     5,
		"examples":       6,
		"tag":            7,
		"sub-tag 1":      8,
		"sub-tag 2":      9,
	}

	existingCard := &models.VocabCard{
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
	}
	mapper.UpdateChecksum(existingCard)

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
		{float64(1234567890123), existingCard.StoredChecksum, "hello", "γεια", "Interjection", "", "", "", "", ""},
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}

	// Setup Anki data - card has been modified in Anki
	modifiedCard := &models.VocabCard{
		AnkiID:       1234567890123,
		English:      "hello",
		Greek:        "γεια σου", // Changed in Anki
		PartOfSpeech: "Interjection",
		Attributes:   "Informal",
		Examples:     "Hello there!",
		Tag:          "Greetings",
		SubTag1:      "Basic",
		SubTag2:      "",
	}

	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{1234567890123}
	ankiClient.NotesInfo = []*models.VocabCard{modifiedCard}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should have 9 updates (checksum + 8 content fields)
	assert.Len(t, sheetsClient.BatchUpdates, 9)

	// Verify updates contain new values
	foundGreekUpdate := false
	for _, update := range sheetsClient.BatchUpdates {
		if update.Column == "D" { // Greek column
			assert.Equal(t, "γεια σου", update.Value)
			foundGreekUpdate = true
		}
	}
	assert.True(t, foundGreekUpdate, "Should have updated Greek field")

	// State should be updated
	assert.NotNil(t, stateManager.SavedState)
	assert.False(t, stateManager.SavedState.LastPullTimestamp.IsZero())
}

func TestPull_NoteNotFoundInSheet(t *testing.T) {
	// Setup empty Sheet
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		// No data rows
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}

	// Setup Anki data with a modified note
	modifiedCard := &models.VocabCard{
		AnkiID:       9999999999999, // Not in Sheet
		English:      "test",
		Greek:        "τεστ",
		PartOfSpeech: "Noun",
	}

	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{9999999999999}
	ankiClient.NotesInfo = []*models.VocabCard{modifiedCard}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// No updates should be written (note skipped)
	assert.Len(t, sheetsClient.BatchUpdates, 0)
}

func TestPull_DryRun(t *testing.T) {
	// Setup Sheet data
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), "old-checksum", "hello", "γεια", "Interjection"},
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}

	// Setup modified Anki card
	modifiedCard := &models.VocabCard{
		AnkiID:       1234567890123,
		English:      "hello",
		Greek:        "γεια σου", // Changed
		PartOfSpeech: "Interjection",
	}

	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{1234567890123}
	ankiClient.NotesInfo = []*models.VocabCard{modifiedCard}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	// Execute dry run
	err := puller.Pull(true)
	require.NoError(t, err)

	// No updates should be written
	assert.Len(t, sheetsClient.BatchUpdates, 0)

	// State should not be updated
	assert.Nil(t, stateManager.SavedState)
}

func TestPull_MultipleModifiedNotes(t *testing.T) {
	// Setup Sheet data with 3 cards
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
		"attributes":     5,
		"examples":       6,
		"tag":            7,
		"sub-tag 1":      8,
		"sub-tag 2":      9,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
		{float64(1111111111111), "checksum1", "first", "πρώτο", "Adjective", "", "", "", "", ""},
		{float64(2222222222222), "checksum2", "second", "δεύτερο", "Adjective", "", "", "", "", ""},
		{float64(3333333333333), "checksum3", "third", "τρίτο", "Adjective", "", "", "", "", ""},
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}

	// Setup Anki data - 2 cards modified
	modifiedCard1 := &models.VocabCard{
		AnkiID:       1111111111111,
		English:      "first",
		Greek:        "πρώτο (modified)", // Changed
		PartOfSpeech: "Adjective",
	}
	modifiedCard2 := &models.VocabCard{
		AnkiID:       3333333333333,
		English:      "third",
		Greek:        "τρίτο (modified)", // Changed
		PartOfSpeech: "Adjective",
	}

	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{1111111111111, 3333333333333}
	ankiClient.NotesInfo = []*models.VocabCard{modifiedCard1, modifiedCard2}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should have 18 updates (2 cards × 9 fields each)
	assert.Len(t, sheetsClient.BatchUpdates, 18)

	// State should be updated
	assert.NotNil(t, stateManager.SavedState)
}

func TestPull_FirstPull_DefaultTimestamp(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{
		Rows: [][]any{
			{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		},
		Headers: map[string]int{
			"anki id":        0,
			"checksum":       1,
			"english":        2,
			"greek":          3,
			"part of speech": 4,
		},
	}

	ankiClient := testutil.NewMockPullerAnkiClient()
	ankiClient.ModifiedNoteIDs = []int64{}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}

	// State with zero timestamp (first pull)
	state := &models.SyncState{
		LastPullTimestamp: time.Time{},
	}
	stateManager := &testutil.MockStateManager{}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should handle zero timestamp gracefully
	// (uses default of 1 year ago in implementation)
}
