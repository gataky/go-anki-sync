package sync

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/sync/internal/anki"
	"github.com/yourusername/sync/internal/mapper"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/pkg/models"
)

// Mock implementations for testing

type mockSheetsClient struct {
	rows                [][]interface{}
	headers             map[string]int
	requiredColumnsErr  error
	checksumColumnAdded bool
	batchUpdates        []sheets.CellUpdate
}

func (m *mockSheetsClient) ReadSheet(sheetID, sheetName string) ([][]interface{}, error) {
	return m.rows, nil
}

func (m *mockSheetsClient) ParseHeaders(rows [][]interface{}) (map[string]int, error) {
	return m.headers, nil
}

func (m *mockSheetsClient) ValidateRequiredColumns(headers map[string]int, required []string) error {
	return m.requiredColumnsErr
}

func (m *mockSheetsClient) CreateChecksumColumnIfMissing(sheetID, sheetName string, headers map[string]int) error {
	if _, exists := headers["checksum"]; !exists {
		m.checksumColumnAdded = true
		m.headers["checksum"] = 1 // Add checksum at column B
	}
	return nil
}

func (m *mockSheetsClient) BatchUpdateCells(sheetID, sheetName string, updates []sheets.CellUpdate) error {
	m.batchUpdates = append(m.batchUpdates, updates...)
	return nil
}

type mockAnkiClient struct {
	nextNoteID    int64
	createdNotes  []*models.VocabCard
	updatedNotes  map[int64]*models.VocabCard
	deckCreated   string
	noteTypeCreated string
}

func newMockAnkiClient() *mockAnkiClient {
	return &mockAnkiClient{
		nextNoteID:   1000000000000,
		createdNotes: make([]*models.VocabCard, 0),
		updatedNotes: make(map[int64]*models.VocabCard),
	}
}

func (m *mockAnkiClient) CreateDeck(deckName string) error {
	m.deckCreated = deckName
	return nil
}

func (m *mockAnkiClient) CreateNoteType(modelName string) error {
	m.noteTypeCreated = modelName
	return nil
}

func (m *mockAnkiClient) AddNote(deckName, modelName string, card *models.VocabCard) (int64, error) {
	noteID := m.nextNoteID
	m.nextNoteID++
	m.createdNotes = append(m.createdNotes, card)
	return noteID, nil
}

func (m *mockAnkiClient) UpdateNoteFields(noteID int64, card *models.VocabCard) error {
	m.updatedNotes[noteID] = card
	return nil
}

func TestNewPusher(t *testing.T) {
	sheetsClient := &mockSheetsClient{}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet-id",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	assert.NotNil(t, pusher)
	assert.Equal(t, sheetsClient, pusher.sheetsClient)
	assert.Equal(t, ankiClient, pusher.ankiClient)
	assert.Equal(t, config, pusher.config)
	assert.Equal(t, logger, pusher.logger)
}

func TestPush_NewCards(t *testing.T) {
	// Setup mock data
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"}, // Header
		{nil, "", "hello", "γεια", "Interjection"}, // New card (no Anki ID)
		{nil, "", "house", "σπίτι", "Noun"},       // New card (no Anki ID)
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	// Execute push
	err := pusher.Push(false)
	require.NoError(t, err)

	// Verify deck and note type created
	assert.Equal(t, "Greek", ankiClient.deckCreated)
	assert.Equal(t, anki.VocabSyncModelName, ankiClient.noteTypeCreated)

	// Verify cards created in Anki
	assert.Len(t, ankiClient.createdNotes, 2)
	assert.Equal(t, "hello", ankiClient.createdNotes[0].English)
	assert.Equal(t, "house", ankiClient.createdNotes[1].English)

	// Verify updates written to sheet (2 cards × 2 updates each = 4 updates)
	assert.Len(t, sheetsClient.batchUpdates, 4)

	// Check that Anki IDs were written
	ankiIDUpdates := 0
	checksumUpdates := 0
	for _, update := range sheetsClient.batchUpdates {
		if update.Column == "A" {
			ankiIDUpdates++
			assert.NotEqual(t, int64(0), update.Value)
		} else if update.Column == "B" {
			checksumUpdates++
			assert.NotEmpty(t, update.Value)
		}
	}
	assert.Equal(t, 2, ankiIDUpdates, "Should have 2 Anki ID updates")
	assert.Equal(t, 2, checksumUpdates, "Should have 2 checksum updates")
}

func TestPush_ExistingCards_NoChanges(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Create card with correct checksum
	card := &models.VocabCard{
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
	}
	mapper.UpdateChecksum(card)

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), card.StoredChecksum, "hello", "γεια", "Interjection"},
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	err := pusher.Push(false)
	require.NoError(t, err)

	// No notes should be created or updated
	assert.Len(t, ankiClient.createdNotes, 0)
	assert.Len(t, ankiClient.updatedNotes, 0)

	// No updates to sheet
	assert.Len(t, sheetsClient.batchUpdates, 0)
}

func TestPush_ExistingCards_WithChanges(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), "old-checksum", "hello", "γεια", "Interjection"}, // Changed content
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	err := pusher.Push(false)
	require.NoError(t, err)

	// Note should be updated
	assert.Len(t, ankiClient.updatedNotes, 1)
	updatedCard := ankiClient.updatedNotes[1234567890123]
	assert.NotNil(t, updatedCard)
	assert.Equal(t, "hello", updatedCard.English)

	// Checksum should be written to sheet
	assert.Len(t, sheetsClient.batchUpdates, 1)
	assert.Equal(t, "B", sheetsClient.batchUpdates[0].Column)
	assert.NotEqual(t, "old-checksum", sheetsClient.batchUpdates[0].Value)
}

func TestPush_DryRun(t *testing.T) {
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
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	// Execute dry run
	err := pusher.Push(true)
	require.NoError(t, err)

	// No notes should be created
	assert.Len(t, ankiClient.createdNotes, 0)

	// No updates to sheet
	assert.Len(t, sheetsClient.batchUpdates, 0)

	// Deck and note type should not be created in dry run
	assert.Empty(t, ankiClient.deckCreated)
	assert.Empty(t, ankiClient.noteTypeCreated)
}

func TestPush_MixedNewAndExisting(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Create existing card with correct checksum
	existingCard := &models.VocabCard{
		English:      "existing",
		Greek:        "υπάρχον",
		PartOfSpeech: "Adjective",
	}
	mapper.UpdateChecksum(existingCard)

	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "new", "νέο", "Adjective"},                                             // New card
		{float64(1111111111111), existingCard.StoredChecksum, "existing", "υπάρχον", "Adjective"}, // Existing, unchanged
		{float64(2222222222222), "wrong-checksum", "changed", "αλλαγμένο", "Adjective"},          // Existing, changed
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	err := pusher.Push(false)
	require.NoError(t, err)

	// 1 new card created
	assert.Len(t, ankiClient.createdNotes, 1)
	assert.Equal(t, "new", ankiClient.createdNotes[0].English)

	// 1 card updated (the one with wrong checksum)
	assert.Len(t, ankiClient.updatedNotes, 1)
	assert.NotNil(t, ankiClient.updatedNotes[2222222222222])

	// Updates: 2 for new card (Anki ID + checksum) + 1 for updated card (checksum) = 3
	assert.Len(t, sheetsClient.batchUpdates, 3)
}

func TestPush_EmptySheet(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
	}

	rows := [][]interface{}{
		{"English", "Greek", "Part of Speech"}, // Header only
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	err := pusher.Push(false)
	require.NoError(t, err)

	// No cards should be processed
	assert.Len(t, ankiClient.createdNotes, 0)
	assert.Len(t, sheetsClient.batchUpdates, 0)
}

func TestPush_InvalidRow(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
	}

	rows := [][]interface{}{
		{"English", "Greek", "Part of Speech"},
		{"hello", "", "Interjection"}, // Missing required Greek field
	}

	sheetsClient := &mockSheetsClient{
		rows:    rows,
		headers: headers,
	}
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger)

	// Should fail fast on validation error
	err := pusher.Push(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "row 2")
	assert.Contains(t, err.Error(), "Greek")
}

func TestCreateNewCards(t *testing.T) {
	ankiClient := newMockAnkiClient()
	config := &models.Config{
		AnkiDeck: "Greek",
	}
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := &Pusher{
		ankiClient: ankiClient,
		config:     config,
		logger:     logger,
	}

	cards := []*models.VocabCard{
		{RowNumber: 2, English: "hello", Greek: "γεια", PartOfSpeech: "Interjection"},
		{RowNumber: 3, English: "house", Greek: "σπίτι", PartOfSpeech: "Noun"},
	}

	updates, err := pusher.createNewCards(cards, false)
	require.NoError(t, err)

	// Should have 4 updates (2 cards × 2 fields each)
	assert.Len(t, updates, 4)

	// Verify Anki IDs were assigned
	assert.NotEqual(t, int64(0), cards[0].AnkiID)
	assert.NotEqual(t, int64(0), cards[1].AnkiID)

	// Verify checksums were set
	assert.NotEmpty(t, cards[0].StoredChecksum)
	assert.NotEmpty(t, cards[1].StoredChecksum)
}

func TestUpdateExistingCards(t *testing.T) {
	ankiClient := newMockAnkiClient()
	logger := log.New(os.Stdout, "TEST: ", 0)

	pusher := &Pusher{
		ankiClient: ankiClient,
		logger:     logger,
	}

	// Card with wrong checksum (changed)
	changedCard := &models.VocabCard{
		RowNumber:      2,
		AnkiID:         1234567890123,
		StoredChecksum: "old-checksum",
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
	}

	// Card with correct checksum (unchanged)
	unchangedCard := &models.VocabCard{
		RowNumber:    3,
		AnkiID:       9876543210987,
		English:      "house",
		Greek:        "σπίτι",
		PartOfSpeech: "Noun",
	}
	mapper.UpdateChecksum(unchangedCard)

	cards := []*models.VocabCard{changedCard, unchangedCard}

	updates, err := pusher.updateExistingCards(cards, false)
	require.NoError(t, err)

	// Should have 1 update (only changed card)
	assert.Len(t, updates, 1)
	assert.Equal(t, 2, updates[0].Row) // Changed card's row

	// Verify only changed card was updated in Anki
	assert.Len(t, ankiClient.updatedNotes, 1)
	assert.NotNil(t, ankiClient.updatedNotes[1234567890123])
}
