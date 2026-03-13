package testutil

import (
	"os"

	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/pkg/models"
)

// NewTestLogger creates a silent logger for tests.
func NewTestLogger() *logging.SyncLogger {
	return logging.NewSyncLogger(logging.Silent, os.Stdout)
}

// NewTestConfig creates a standard test configuration.
func NewTestConfig() *models.Config {
	return &models.Config{
		GoogleSheetID:  "test-sheet-id",
		SheetName:      "Vocabulary",
		AnkiDeck:       "Greek",
		AnkiConnectURL: "http://localhost:8765",
		AnkiProfile:    "User 1",
	}
}

// StandardHeaders returns a standard header map for testing.
func StandardHeaders() map[string]int {
	return map[string]int{
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
}

// CardOption is a functional option for customizing test cards.
type CardOption func(*models.VocabCard)

// WithAnkiID sets the Anki ID for a test card.
func WithAnkiID(id int64) CardOption {
	return func(c *models.VocabCard) {
		c.AnkiID = id
	}
}

// WithChecksum sets the stored checksum for a test card.
func WithChecksum(checksum string) CardOption {
	return func(c *models.VocabCard) {
		c.StoredChecksum = checksum
	}
}

// WithAttributes sets the attributes for a test card.
func WithAttributes(attrs string) CardOption {
	return func(c *models.VocabCard) {
		c.Attributes = attrs
	}
}

// WithExamples sets the examples for a test card.
func WithExamples(examples string) CardOption {
	return func(c *models.VocabCard) {
		c.Examples = examples
	}
}

// WithTags sets the tag hierarchy for a test card.
func WithTags(tag, subTag1, subTag2 string) CardOption {
	return func(c *models.VocabCard) {
		c.Tag = tag
		c.SubTag1 = subTag1
		c.SubTag2 = subTag2
	}
}

// WithRowNumber sets the row number for a test card.
func WithRowNumber(rowNum int) CardOption {
	return func(c *models.VocabCard) {
		c.RowNumber = rowNum
	}
}

// NewTestCard creates a test VocabCard with default values and optional customizations.
// Default card: English="hello", Greek="γεια", PartOfSpeech="Interjection", RowNumber=2
func NewTestCard(english, greek, partOfSpeech string, opts ...CardOption) *models.VocabCard {
	card := &models.VocabCard{
		English:      english,
		Greek:        greek,
		PartOfSpeech: partOfSpeech,
		RowNumber:    2, // Default row (first data row after header)
	}

	for _, opt := range opts {
		opt(card)
	}

	return card
}

// BuildSheetRows converts cards to sheet rows format (as returned by Sheets API).
// Includes a header row at the beginning.
func BuildSheetRows(cards []*models.VocabCard) [][]any {
	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
	}

	for _, card := range cards {
		row := []any{
			card.AnkiID,
			card.StoredChecksum,
			card.English,
			card.Greek,
			card.PartOfSpeech,
			card.Attributes,
			card.Examples,
			card.Tag,
			card.SubTag1,
			card.SubTag2,
		}
		rows = append(rows, row)
	}

	return rows
}

// NewTestState creates a test sync state.
func NewTestState() *models.SyncState {
	return &models.SyncState{}
}
