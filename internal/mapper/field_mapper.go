package mapper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gataky/sync/pkg/models"
)

// RowToCard converts a Google Sheets row to a VocabCard.
// The headers map provides column name to index mapping (case-insensitive).
// rowNumber is 1-indexed (excluding the header row).
//
// Required fields: English, Greek, Part of Speech
// Optional fields: Anki ID, Checksum, Attributes, Examples, Tag, Sub-Tag 1, Sub-Tag 2
func RowToCard(row []interface{}, headers map[string]int, rowNumber int) (*models.VocabCard, error) {
	card := &models.VocabCard{
		RowNumber: rowNumber,
	}

	// Extract required fields
	english, err := getString(row, headers, "english")
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowNumber, err)
	}
	if strings.TrimSpace(english) == "" {
		return nil, fmt.Errorf("row %d: English field is required", rowNumber)
	}
	card.English = english

	greek, err := getString(row, headers, "greek")
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowNumber, err)
	}
	if strings.TrimSpace(greek) == "" {
		return nil, fmt.Errorf("row %d: Greek field is required", rowNumber)
	}
	card.Greek = greek

	partOfSpeech, err := getString(row, headers, "part of speech")
	if err != nil {
		return nil, fmt.Errorf("row %d: %w", rowNumber, err)
	}
	if strings.TrimSpace(partOfSpeech) == "" {
		return nil, fmt.Errorf("row %d: Part of Speech field is required", rowNumber)
	}
	card.PartOfSpeech = partOfSpeech

	// Extract optional fields (no error if missing or empty)
	card.AnkiID, _ = getInt64(row, headers, "anki id")
	card.StoredChecksum, _ = getString(row, headers, "checksum")
	card.Attributes, _ = getString(row, headers, "attributes")
	card.Examples, _ = getString(row, headers, "examples")
	card.Tag, _ = getString(row, headers, "tag")
	card.SubTag1, _ = getString(row, headers, "sub-tag 1")
	card.SubTag2, _ = getString(row, headers, "sub-tag 2")

	return card, nil
}

// CardToRow converts a VocabCard back to a sheet row.
// The headers map provides column name to index mapping.
// Returns a slice with values in the correct column positions.
func CardToRow(card *models.VocabCard, headers map[string]int) []interface{} {
	// Create a row with the correct number of columns
	maxIndex := 0
	for _, idx := range headers {
		if idx > maxIndex {
			maxIndex = idx
		}
	}
	row := make([]interface{}, maxIndex+1)

	// Fill in the values
	setCell(row, headers, "anki id", card.AnkiID)
	setCell(row, headers, "checksum", card.StoredChecksum)
	setCell(row, headers, "english", card.English)
	setCell(row, headers, "greek", card.Greek)
	setCell(row, headers, "part of speech", card.PartOfSpeech)
	setCell(row, headers, "attributes", card.Attributes)
	setCell(row, headers, "examples", card.Examples)
	setCell(row, headers, "tag", card.Tag)
	setCell(row, headers, "sub-tag 1", card.SubTag1)
	setCell(row, headers, "sub-tag 2", card.SubTag2)

	return row
}

// getString extracts a string value from a row at the column specified by the header name.
// Returns empty string if column doesn't exist or cell is empty/nil.
func getString(row []interface{}, headers map[string]int, columnName string) (string, error) {
	idx, exists := headers[columnName]
	if !exists {
		return "", nil // Column doesn't exist, return empty string
	}

	if idx >= len(row) {
		return "", nil // Row doesn't have this column, return empty string
	}

	cell := row[idx]
	if cell == nil {
		return "", nil
	}

	// Convert to string
	return fmt.Sprintf("%v", cell), nil
}

// getInt64 extracts an int64 value from a row at the column specified by the header name.
// Returns 0 if column doesn't exist, cell is empty/nil, or value cannot be parsed.
func getInt64(row []interface{}, headers map[string]int, columnName string) (int64, error) {
	idx, exists := headers[columnName]
	if !exists {
		return 0, nil // Column doesn't exist
	}

	if idx >= len(row) {
		return 0, nil // Row doesn't have this column
	}

	cell := row[idx]
	if cell == nil {
		return 0, nil
	}

	// Try to convert to int64
	switch v := cell.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		// Google Sheets API returns numbers as float64
		return int64(v), nil
	case string:
		// Try to parse string as int64
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse '%s' as int64: %w", v, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected type %T for int64 field", cell)
	}
}

// setCell sets a value in a row at the column specified by the header name.
// Does nothing if the column doesn't exist.
func setCell(row []interface{}, headers map[string]int, columnName string, value interface{}) {
	idx, exists := headers[columnName]
	if !exists {
		return // Column doesn't exist, skip
	}

	if idx >= len(row) {
		return // Row doesn't have this column, skip
	}

	// Special handling for int64 to preserve as number (not scientific notation)
	if v, ok := value.(int64); ok {
		if v == 0 {
			row[idx] = "" // Empty cell for zero values
		} else {
			row[idx] = v
		}
		return
	}

	// Empty strings should be represented as empty cells
	if s, ok := value.(string); ok && s == "" {
		row[idx] = ""
		return
	}

	row[idx] = value
}

// ValidateCard performs business logic validation on a VocabCard.
// Returns an error if the card violates any business rules.
func ValidateCard(card *models.VocabCard) error {
	// Required fields
	if strings.TrimSpace(card.English) == "" {
		return fmt.Errorf("English field is required")
	}
	if strings.TrimSpace(card.Greek) == "" {
		return fmt.Errorf("Greek field is required")
	}
	if strings.TrimSpace(card.PartOfSpeech) == "" {
		return fmt.Errorf("Part of Speech field is required")
	}

	// Tag hierarchy validation
	if card.SubTag2 != "" && card.SubTag1 == "" {
		return fmt.Errorf("Sub-Tag 2 cannot be set without Sub-Tag 1")
	}
	if card.SubTag1 != "" && card.Tag == "" {
		return fmt.Errorf("Sub-Tag 1 cannot be set without Tag")
	}

	return nil
}
