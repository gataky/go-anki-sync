package sheets

// Column letters for the fixed leading columns of the vocab sheet layout
const (
	ColAnkiID   = "A"
	ColChecksum = "B"
)

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
			Row:    rowNumber - 1, // CellUpdate uses 0-indexed rows
			Column: colLetter,
			Value:  "",
		},
	}
}
