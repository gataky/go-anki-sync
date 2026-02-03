package sync

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/sync/internal/mapper"
	"github.com/yourusername/sync/pkg/models"
)

// Mock implementations for Puller tests

type mockPullerAnkiClient struct {
	modifiedNoteIDs []int64
	notesInfo       []*models.VocabCard
}

func (m *mockPullerAnkiClient) CreateDeck(deckName string) error {
	return nil
}

func (m *mockPullerAnkiClient) CreateNoteType(modelName string) error {
	return nil
}

func (m *mockPullerAnkiClient) AddNote(deckName, modelName string, card *models.VocabCard) (int64, error) {
	return 0, nil
}

func (m *mockPullerAnkiClient) UpdateNoteFields(noteID int64, card *models.VocabCard) error {
	return nil
}

func (m *mockPullerAnkiClient) FindModifiedNotes(deckName string, sinceTimestamp time.Time) ([]int64, error) {
	return m.modifiedNoteIDs, nil
}

func (m *mockPullerAnkiClient) GetNotesInfo(noteIDs []int64) ([]*models.VocabCard, error) {
	return m.notesInfo, nil
}

type mockStateManager struct {
	savedState *models.SyncState
	statePath  string
}

func (m *mockStateManager) LoadState(path string) (*models.SyncState, error) {
	return &models.SyncState{}, nil
}

func (m *mockStateManager) SaveState(state *models.SyncState, path string) error {
	m.savedState = state
	return nil
}

func (m *mockStateManager) GetDefaultStatePath() string {
	if m.statePath == "" {
		return "/tmp/state.json"
	}
	return m.statePath
}

func TestNewPuller(t *testing.T) {
	sheetsClient := &mockSheetsClient{}
	ankiClient := &mockPullerAnkiClient{}
	config := &models.Config{
		GoogleSheetID: "test-sheet-id",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	assert.NotNil(t, puller)
	assert.Equal(t, sheetsClient, puller.sheetsClient)
	assert.Equal(t, ankiClient, puller.ankiClient)
	assert.Equal(t, config, puller.config)
	assert.Equal(t, state, puller.state)
	assert.Equal(t, logger, puller.logger)
}

func TestPull_NoModifiedNotes(t *testing.T) {
	sheetsClient := &mockSheetsClient{}
	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{}, // No modified notes
	}
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// No updates should be written
	assert.Len(t, sheetsClient.batchUpdates, 0)

	// State should not be updated
	assert.Nil(t, stateManager.savedState)
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

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
		{float64(1234567890123), existingCard.StoredChecksum, "hello", "γεια", "Interjection", "", "", "", "", ""},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
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

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{1234567890123},
		notesInfo:       []*models.VocabCard{modifiedCard},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should have 9 updates (checksum + 8 content fields)
	assert.Len(t, sheetsClient.batchUpdates, 9)

	// Verify updates contain new values
	foundGreekUpdate := false
	for _, update := range sheetsClient.batchUpdates {
		if update.Column == "D" { // Greek column
			assert.Equal(t, "γεια σου", update.Value)
			foundGreekUpdate = true
		}
	}
	assert.True(t, foundGreekUpdate, "Should have updated Greek field")

	// State should be updated
	assert.NotNil(t, stateManager.savedState)
	assert.False(t, stateManager.savedState.LastPullTimestamp.IsZero())
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

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		// No data rows
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	// Setup Anki data with a modified note
	modifiedCard := &models.VocabCard{
		AnkiID:       9999999999999, // Not in Sheet
		English:      "test",
		Greek:        "τεστ",
		PartOfSpeech: "Noun",
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{9999999999999},
		notesInfo:       []*models.VocabCard{modifiedCard},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// No updates should be written (note skipped)
	assert.Len(t, sheetsClient.batchUpdates, 0)
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

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), "old-checksum", "hello", "γεια", "Interjection"},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}

	// Setup modified Anki card
	modifiedCard := &models.VocabCard{
		AnkiID:       1234567890123,
		English:      "hello",
		Greek:        "γεια σου", // Changed
		PartOfSpeech: "Interjection",
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{1234567890123},
		notesInfo:       []*models.VocabCard{modifiedCard},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	// Execute dry run
	err := puller.Pull(true)
	require.NoError(t, err)

	// No updates should be written
	assert.Len(t, sheetsClient.batchUpdates, 0)

	// State should not be updated
	assert.Nil(t, stateManager.savedState)
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

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
		{float64(1111111111111), "checksum1", "first", "πρώτο", "Adjective", "", "", "", "", ""},
		{float64(2222222222222), "checksum2", "second", "δεύτερο", "Adjective", "", "", "", "", ""},
		{float64(3333333333333), "checksum3", "third", "τρίτο", "Adjective", "", "", "", "", ""},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
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

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{1111111111111, 3333333333333},
		notesInfo:       []*models.VocabCard{modifiedCard1, modifiedCard2},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	state := &models.SyncState{
		LastPullTimestamp: time.Now().Add(-1 * time.Hour),
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should have 18 updates (2 cards × 9 fields each)
	assert.Len(t, sheetsClient.batchUpdates, 18)

	// State should be updated
	assert.NotNil(t, stateManager.savedState)
}

func TestPull_FirstPull_DefaultTimestamp(t *testing.T) {
	sheetsClient := &mockSheetsClient{
		rows: [][]interface{}{
			{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		},
		headers: map[string]int{
			"anki id":        0,
			"checksum":       1,
			"english":        2,
			"greek":          3,
			"part of speech": 4,
		},
	}

	ankiClient := &mockPullerAnkiClient{
		modifiedNoteIDs: []int64{}, // No notes
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}

	// State with zero timestamp (first pull)
	state := &models.SyncState{
		LastPullTimestamp: time.Time{},
	}
	stateManager := &mockStateManager{}
	logger := log.New(os.Stdout, "TEST: ", 0)

	puller := NewPuller(sheetsClient, ankiClient, config, state, stateManager, logger)

	err := puller.Pull(false)
	require.NoError(t, err)

	// Should handle zero timestamp gracefully
	// (uses default of 1 year ago in implementation)
}
