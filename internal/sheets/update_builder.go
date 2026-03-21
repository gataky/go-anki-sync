package sheets

import "github.com/gataky/sync/pkg/models"

// Column constants for standard vocab sheet layout
const (
	ColAnkiID       = "A"
	ColChecksum     = "B"
	ColEnglish      = "C"
	ColGreek        = "D"
	ColPartOfSpeech = "E"
	ColAttributes   = "F"
	ColExamples     = "G"
	ColTag          = "H"
	ColSubTag1      = "I"
	ColSubTag2      = "J"
)

// BuildCardUpdate creates complete cell updates for a vocab card.
// rowNumber should be 1-indexed excluding header (card.RowNumber).
// includeAnkiID determines whether to include Anki ID column.
func BuildCardUpdate(card *models.VocabCard, includeAnkiID bool) []CellUpdate {
	row := card.RowNumber - 1 // CellUpdate uses 0-indexed rows

	updates := make([]CellUpdate, 0, 10)
	if includeAnkiID {
		updates = append(updates, CellUpdate{Row: row, Column: ColAnkiID, Value: card.AnkiID})
	}

	return append(updates,
		CellUpdate{Row: row, Column: ColChecksum, Value: card.StoredChecksum},
		CellUpdate{Row: row, Column: ColEnglish, Value: card.English},
		CellUpdate{Row: row, Column: ColGreek, Value: card.Greek},
		CellUpdate{Row: row, Column: ColPartOfSpeech, Value: card.PartOfSpeech},
		CellUpdate{Row: row, Column: ColAttributes, Value: card.Attributes},
		CellUpdate{Row: row, Column: ColExamples, Value: card.Examples},
		CellUpdate{Row: row, Column: ColTag, Value: card.Tag},
		CellUpdate{Row: row, Column: ColSubTag1, Value: card.SubTag1},
		CellUpdate{Row: row, Column: ColSubTag2, Value: card.SubTag2},
	)
}

// BuildAnkiIDAndChecksumUpdate creates updates for new cards (ID + checksum only).
func BuildAnkiIDAndChecksumUpdate(rowNumber int, ankiID int64, checksum string) []CellUpdate {
	row := rowNumber - 1
	return []CellUpdate{
		{Row: row, Column: ColAnkiID, Value: ankiID},
		{Row: row, Column: ColChecksum, Value: checksum},
	}
}

// BuildChecksumOnlyUpdate creates update for checksum column only.
func BuildChecksumOnlyUpdate(rowNumber int, checksum string) []CellUpdate {
	return []CellUpdate{{Row: rowNumber - 1, Column: ColChecksum, Value: checksum}}
}

// ColumnIndexToLetter converts a 0-indexed column number to Excel-style column letter(s).
// Examples: 0→A, 1→B, 25→Z, 26→AA, 27→AB
func ColumnIndexToLetter(index int) string {
	result := ""
	for index >= 0 {
		result = string(rune('A'+(index%26))) + result
		index = index/26 - 1
	}
	return result
}

// BuildRegenTTSClearUpdate creates an update to clear the "Regen TTS" column.
// Used after successful audio regeneration to reset the flag.
// Returns nil if the "Regen TTS" column doesn't exist in headers.
func BuildRegenTTSClearUpdate(rowNumber int, headers map[string]int) []CellUpdate {
	// Look up the "Regen TTS" column index
	colIndex, exists := headers["regen tts"]
	if !exists {
		return nil // Column doesn't exist, skip update
	}

	// Convert column index to letter
	colLetter := ColumnIndexToLetter(colIndex)

	return []CellUpdate{
		{
			Row:    rowNumber,
			Column: colLetter,
			Value:  "",
		},
	}
}
