package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/sync/pkg/models"
)

func TestRowToCard_ValidData(t *testing.T) {
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

	row := []interface{}{
		float64(1234567890123), // Anki ID (Google Sheets returns as float64)
		"abc123checksum",
		"hello",
		"γεια",
		"Interjection",
		"Informal",
		"Hello, how are you?\nΓεια, πώς είσαι;",
		"Greetings",
		"Basic",
		"Common",
	}

	card, err := RowToCard(row, headers, 2)
	require.NoError(t, err)

	assert.Equal(t, 2, card.RowNumber)
	assert.Equal(t, int64(1234567890123), card.AnkiID)
	assert.Equal(t, "abc123checksum", card.StoredChecksum)
	assert.Equal(t, "hello", card.English)
	assert.Equal(t, "γεια", card.Greek)
	assert.Equal(t, "Interjection", card.PartOfSpeech)
	assert.Equal(t, "Informal", card.Attributes)
	assert.Equal(t, "Hello, how are you?\nΓεια, πώς είσαι;", card.Examples)
	assert.Equal(t, "Greetings", card.Tag)
	assert.Equal(t, "Basic", card.SubTag1)
	assert.Equal(t, "Common", card.SubTag2)
}

func TestRowToCard_MinimalData(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
	}

	row := []interface{}{
		"test",
		"τεστ",
		"Noun",
	}

	card, err := RowToCard(row, headers, 5)
	require.NoError(t, err)

	assert.Equal(t, 5, card.RowNumber)
	assert.Equal(t, int64(0), card.AnkiID) // Optional field defaults to 0
	assert.Equal(t, "", card.StoredChecksum)
	assert.Equal(t, "test", card.English)
	assert.Equal(t, "τεστ", card.Greek)
	assert.Equal(t, "Noun", card.PartOfSpeech)
	assert.Equal(t, "", card.Attributes)
	assert.Equal(t, "", card.Examples)
	assert.Equal(t, "", card.Tag)
	assert.Equal(t, "", card.SubTag1)
	assert.Equal(t, "", card.SubTag2)
}

func TestRowToCard_MissingRequiredField(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"part of speech": 2,
		// Missing "greek"
	}

	row := []interface{}{
		"test",
		"", // Empty where greek should be
		"Noun",
	}

	_, err := RowToCard(row, headers, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "row 3")
}

func TestRowToCard_EmptyRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		row       []interface{}
		headers   map[string]int
		expectErr string
	}{
		{
			name: "empty English",
			headers: map[string]int{
				"english":        0,
				"greek":          1,
				"part of speech": 2,
			},
			row:       []interface{}{"", "γεια", "Interjection"},
			expectErr: "English field is required",
		},
		{
			name: "empty Greek",
			headers: map[string]int{
				"english":        0,
				"greek":          1,
				"part of speech": 2,
			},
			row:       []interface{}{"hello", "", "Interjection"},
			expectErr: "Greek field is required",
		},
		{
			name: "empty Part of Speech",
			headers: map[string]int{
				"english":        0,
				"greek":          1,
				"part of speech": 2,
			},
			row:       []interface{}{"hello", "γεια", ""},
			expectErr: "Part of Speech field is required",
		},
		{
			name: "whitespace only English",
			headers: map[string]int{
				"english":        0,
				"greek":          1,
				"part of speech": 2,
			},
			row:       []interface{}{"  ", "γεια", "Interjection"},
			expectErr: "English field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RowToCard(tt.row, tt.headers, 10)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
		})
	}
}

func TestRowToCard_NilCells(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
		"attributes":     5,
	}

	row := []interface{}{
		nil, // Anki ID is nil
		nil, // Checksum is nil
		"hello",
		"γεια",
		"Interjection",
		nil, // Attributes is nil
	}

	card, err := RowToCard(row, headers, 1)
	require.NoError(t, err)

	assert.Equal(t, int64(0), card.AnkiID)
	assert.Equal(t, "", card.StoredChecksum)
	assert.Equal(t, "", card.Attributes)
	assert.Equal(t, "hello", card.English)
	assert.Equal(t, "γεια", card.Greek)
}

func TestRowToCard_ShortRow(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
		"attributes":     3,
		"examples":       4,
		"tag":            5,
	}

	// Row is shorter than headers expect
	row := []interface{}{
		"hello",
		"γεια",
		"Interjection",
		// Missing: attributes, examples, tag
	}

	card, err := RowToCard(row, headers, 1)
	require.NoError(t, err)

	assert.Equal(t, "hello", card.English)
	assert.Equal(t, "γεια", card.Greek)
	assert.Equal(t, "Interjection", card.PartOfSpeech)
	assert.Equal(t, "", card.Attributes) // Missing columns default to empty
	assert.Equal(t, "", card.Examples)
	assert.Equal(t, "", card.Tag)
}

func TestGetInt64_DifferentTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int64
		wantErr  bool
	}{
		{"int64 value", int64(1234567890123), 1234567890123, false},
		{"int value", int(123456), 123456, false},
		{"float64 value", float64(1234567890123), 1234567890123, false},
		{"string value", "1234567890123", 1234567890123, false},
		{"empty string", "", 0, false},
		{"whitespace string", "  ", 0, false},
		{"nil value", nil, 0, false},
		{"invalid string", "not-a-number", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]int{"test": 0}
			row := []interface{}{tt.value}

			result, err := getInt64(row, headers, "test")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetString_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string value", "hello", "hello"},
		{"int value", 123, "123"},
		{"float value", 45.67, "45.67"},
		{"bool value", true, "true"},
		{"nil value", nil, ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]int{"test": 0}
			row := []interface{}{tt.value}

			result, err := getString(row, headers, "test")

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCardToRow(t *testing.T) {
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

	card := &models.VocabCard{
		RowNumber:      5,
		AnkiID:         1234567890123,
		StoredChecksum: "abc123",
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
		Attributes:     "Informal",
		Examples:       "Example text",
		Tag:            "Greetings",
		SubTag1:        "Basic",
		SubTag2:        "Common",
	}

	row := CardToRow(card, headers)

	assert.Equal(t, int64(1234567890123), row[0])
	assert.Equal(t, "abc123", row[1])
	assert.Equal(t, "hello", row[2])
	assert.Equal(t, "γεια", row[3])
	assert.Equal(t, "Interjection", row[4])
	assert.Equal(t, "Informal", row[5])
	assert.Equal(t, "Example text", row[6])
	assert.Equal(t, "Greetings", row[7])
	assert.Equal(t, "Basic", row[8])
	assert.Equal(t, "Common", row[9])
}

func TestCardToRow_EmptyFields(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
		"anki id":        3,
		"checksum":       4,
	}

	card := &models.VocabCard{
		English:        "test",
		Greek:          "τεστ",
		PartOfSpeech:   "Noun",
		AnkiID:         0, // Zero value
		StoredChecksum: "", // Empty string
	}

	row := CardToRow(card, headers)

	assert.Equal(t, "test", row[0])
	assert.Equal(t, "τεστ", row[1])
	assert.Equal(t, "Noun", row[2])
	assert.Equal(t, "", row[3]) // Zero int64 becomes empty cell
	assert.Equal(t, "", row[4]) // Empty string becomes empty cell
}

func TestRoundTrip_RowToCardToRow(t *testing.T) {
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

	originalRow := []interface{}{
		float64(1234567890123),
		"checksum123",
		"hello",
		"γεια",
		"Interjection",
		"Informal",
		"Examples here",
		"Greetings",
		"Basic",
		"Common",
	}

	// Row -> Card
	card, err := RowToCard(originalRow, headers, 1)
	require.NoError(t, err)

	// Card -> Row
	newRow := CardToRow(card, headers)

	// Compare (handle type differences like float64 vs int64)
	assert.Equal(t, int64(1234567890123), newRow[0])
	assert.Equal(t, "checksum123", newRow[1])
	assert.Equal(t, "hello", newRow[2])
	assert.Equal(t, "γεια", newRow[3])
	assert.Equal(t, "Interjection", newRow[4])
	assert.Equal(t, "Informal", newRow[5])
	assert.Equal(t, "Examples here", newRow[6])
	assert.Equal(t, "Greetings", newRow[7])
	assert.Equal(t, "Basic", newRow[8])
	assert.Equal(t, "Common", newRow[9])
}

func TestValidateCard_Valid(t *testing.T) {
	tests := []struct {
		name string
		card *models.VocabCard
	}{
		{
			name: "complete card",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
				Tag:          "Greetings",
				SubTag1:      "Basic",
				SubTag2:      "Common",
			},
		},
		{
			name: "minimal card",
			card: &models.VocabCard{
				English:      "test",
				Greek:        "τεστ",
				PartOfSpeech: "Noun",
			},
		},
		{
			name: "card with tag hierarchy",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
				Tag:          "Greetings",
				SubTag1:      "Basic",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCard(tt.card)
			assert.NoError(t, err)
		})
	}
}

func TestValidateCard_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		card    *models.VocabCard
		wantErr string
	}{
		{
			name: "missing English",
			card: &models.VocabCard{
				English:      "",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
			},
			wantErr: "English field is required",
		},
		{
			name: "missing Greek",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "",
				PartOfSpeech: "Interjection",
			},
			wantErr: "Greek field is required",
		},
		{
			name: "missing Part of Speech",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "",
			},
			wantErr: "Part of Speech field is required",
		},
		{
			name: "SubTag2 without SubTag1",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
				Tag:          "Greetings",
				SubTag1:      "",
				SubTag2:      "Advanced",
			},
			wantErr: "Sub-Tag 2 cannot be set without Sub-Tag 1",
		},
		{
			name: "SubTag1 without Tag",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
				Tag:          "",
				SubTag1:      "Basic",
			},
			wantErr: "Sub-Tag 1 cannot be set without Tag",
		},
		{
			name: "whitespace only English",
			card: &models.VocabCard{
				English:      "   ",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
			},
			wantErr: "English field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCard(tt.card)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
