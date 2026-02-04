package sync

import (
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/pkg/models"
)

// SheetsClientInterface defines the methods needed from the Sheets client.
type SheetsClientInterface interface {
	ReadSheet(sheetID, sheetName string) ([][]interface{}, error)
	ParseHeaders(rows [][]interface{}) (map[string]int, error)
	ValidateRequiredColumns(headers map[string]int, required []string) error
	CreateChecksumColumnIfMissing(sheetID, sheetName string, headers map[string]int) error
	BatchUpdateCells(sheetID, sheetName string, updates []sheets.CellUpdate) error
}

// AnkiClientInterface defines the methods needed from the Anki client.
type AnkiClientInterface interface {
	CreateDeck(deckName string) error
	CreateNoteType(modelName string) error
	AddNote(deckName, modelName string, card *models.VocabCard, audioData []byte, audioFilename string) (int64, error)
	UpdateNoteFields(noteID int64, card *models.VocabCard) error
	CheckAudioExists(filename string) (bool, error)
}

// TTSClientInterface defines the methods needed from the TTS client.
type TTSClientInterface interface {
	GenerateAudio(greekText string) ([]byte, error)
	Close() error
}
