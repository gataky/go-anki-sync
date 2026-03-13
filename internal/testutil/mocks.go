package testutil

import (
	"fmt"
	"time"

	"github.com/gataky/sync/internal/sheets"
	"github.com/gataky/sync/pkg/models"
)

// MockSheetsClient is a mock implementation of SheetsClientInterface for testing.
type MockSheetsClient struct {
	Rows                [][]any
	Headers             map[string]int
	RequiredColumnsErr  error
	ChecksumColumnAdded bool
	BatchUpdates        []sheets.CellUpdate
	LastBatchUpdate     []sheets.CellUpdate // The most recent batch update
}

func (m *MockSheetsClient) ReadSheet(sheetID, sheetName string) ([][]any, error) {
	return m.Rows, nil
}

func (m *MockSheetsClient) ParseHeaders(rows [][]any) (map[string]int, error) {
	return m.Headers, nil
}

func (m *MockSheetsClient) ValidateRequiredColumns(headers map[string]int, required []string) error {
	return m.RequiredColumnsErr
}

func (m *MockSheetsClient) CreateChecksumColumnIfMissing(sheetID, sheetName string, headers map[string]int) error {
	if _, exists := headers["checksum"]; !exists {
		m.ChecksumColumnAdded = true
		m.Headers["checksum"] = 1 // Add checksum at column B
	}
	return nil
}

func (m *MockSheetsClient) BatchUpdateCells(sheetID, sheetName string, updates []sheets.CellUpdate) error {
	m.BatchUpdates = append(m.BatchUpdates, updates...)
	m.LastBatchUpdate = updates // Store the most recent batch
	return nil
}

// MockAnkiClient is a mock implementation of AnkiClientInterface for testing.
type MockAnkiClient struct {
	NextNoteID      int64
	CreatedNotes    []*models.VocabCard
	UpdatedNotes    map[int64]*models.VocabCard
	DeckCreated     string
	NoteTypeCreated string
	FailCards       map[string]bool // Cards that should fail to create (keyed by English)
	FailUpdates     map[int64]bool  // Note IDs that should fail to update
	DeckExists      bool            // If true, CreateDeck returns success without doing anything
	ModelExists     bool            // If true, CreateNoteType returns success without doing anything
	AudioExists     map[string]bool // Audio files that exist (keyed by filename)
}

// NewMockAnkiClient creates a new MockAnkiClient with default values.
func NewMockAnkiClient() *MockAnkiClient {
	return &MockAnkiClient{
		NextNoteID:   1000000000000,
		CreatedNotes: make([]*models.VocabCard, 0),
		UpdatedNotes: make(map[int64]*models.VocabCard),
	}
}

func (m *MockAnkiClient) CreateDeck(deckName string) error {
	m.DeckCreated = deckName
	return nil
}

func (m *MockAnkiClient) CreateNoteType(modelName string) error {
	m.NoteTypeCreated = modelName
	return nil
}

func (m *MockAnkiClient) AddNote(deckName, modelName string, card *models.VocabCard, audioData []byte, audioFilename string) (int64, error) {
	// Check if this card should fail
	if m.FailCards != nil && m.FailCards[card.English] {
		return 0, fmt.Errorf("mock error: failed to create card '%s'", card.English)
	}

	noteID := m.NextNoteID
	m.NextNoteID++
	m.CreatedNotes = append(m.CreatedNotes, card)
	return noteID, nil
}

func (m *MockAnkiClient) CheckAudioExists(filename string) (bool, error) {
	// Check if audio exists in the map
	if m.AudioExists != nil {
		if exists, ok := m.AudioExists[filename]; ok {
			return exists, nil
		}
	}
	// Default: audio doesn't exist
	return false, nil
}

func (m *MockAnkiClient) UpdateNoteFields(noteID int64, card *models.VocabCard, audioData []byte, audioFilename string) error {
	// Check if this update should fail
	if m.FailUpdates != nil && m.FailUpdates[noteID] {
		return fmt.Errorf("mock error: failed to update note %d", noteID)
	}

	m.UpdatedNotes[noteID] = card
	return nil
}

// MockPullerAnkiClient is a mock implementation of PullerAnkiClient for testing.
// It extends MockAnkiClient with pull-specific methods.
type MockPullerAnkiClient struct {
	*MockAnkiClient
	ModifiedNoteIDs []int64
	NotesInfo       []*models.VocabCard
}

// NewMockPullerAnkiClient creates a new MockPullerAnkiClient.
func NewMockPullerAnkiClient() *MockPullerAnkiClient {
	return &MockPullerAnkiClient{
		MockAnkiClient:  NewMockAnkiClient(),
		ModifiedNoteIDs: []int64{},
		NotesInfo:       []*models.VocabCard{},
	}
}

func (m *MockPullerAnkiClient) FindModifiedNotes(deckName string, sinceTimestamp time.Time) ([]int64, error) {
	return m.ModifiedNoteIDs, nil
}

func (m *MockPullerAnkiClient) GetNotesInfo(noteIDs []int64) ([]*models.VocabCard, error) {
	return m.NotesInfo, nil
}

// MockStateManager is a mock implementation of StateManager for testing.
type MockStateManager struct {
	SavedState *models.SyncState
	StatePath  string
}

func (m *MockStateManager) LoadState(path string) (*models.SyncState, error) {
	return &models.SyncState{}, nil
}

func (m *MockStateManager) SaveState(state *models.SyncState, path string) error {
	m.SavedState = state
	return nil
}

func (m *MockStateManager) GetDefaultStatePath() string {
	if m.StatePath == "" {
		return "/tmp/state.json"
	}
	return m.StatePath
}

// MockTTSClient is a mock implementation of TTSClient for testing.
type MockTTSClient struct {
	AudioGenerated map[string][]byte // keyed by Greek text
	GenerateError  error             // If set, GenerateAudio returns this error
}

// NewMockTTSClient creates a new MockTTSClient.
func NewMockTTSClient() *MockTTSClient {
	return &MockTTSClient{
		AudioGenerated: make(map[string][]byte),
	}
}

func (m *MockTTSClient) GenerateAudio(greekText string) ([]byte, error) {
	if m.GenerateError != nil {
		return nil, m.GenerateError
	}

	// Generate fake audio data
	audioData := []byte(fmt.Sprintf("audio-for-%s", greekText))
	m.AudioGenerated[greekText] = audioData
	return audioData, nil
}

func (m *MockTTSClient) Close() error {
	return nil
}
