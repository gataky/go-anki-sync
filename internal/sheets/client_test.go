package sheets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHeaders(t *testing.T) {
	client := &SheetsClient{} // No service needed for this test

	tests := []struct {
		name     string
		rows     [][]interface{}
		expected map[string]int
		wantErr  bool
	}{
		{
			name: "valid headers",
			rows: [][]interface{}{
				{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
			},
			expected: map[string]int{
				"anki id":        0,
				"checksum":       1,
				"english":        2,
				"greek":          3,
				"part of speech": 4,
			},
			wantErr: false,
		},
		{
			name: "case insensitive",
			rows: [][]interface{}{
				{"ANKI ID", "English", "GREEK"},
			},
			expected: map[string]int{
				"anki id": 0,
				"english": 1,
				"greek":   2,
			},
			wantErr: false,
		},
		{
			name: "headers with spaces",
			rows: [][]interface{}{
				{"  Anki ID  ", "  English  ", "Greek"},
			},
			expected: map[string]int{
				"anki id": 0,
				"english": 1,
				"greek":   2,
			},
			wantErr: false,
		},
		{
			name: "empty cells ignored",
			rows: [][]interface{}{
				{"Anki ID", nil, "English", "", "Greek"},
			},
			expected: map[string]int{
				"anki id": 0,
				"english": 2,
				"greek":   4,
			},
			wantErr: false,
		},
		{
			name:     "no rows",
			rows:     [][]interface{}{},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, err := client.ParseHeaders(tt.rows)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, headers)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, headers)
			}
		})
	}
}

func TestValidateRequiredColumns(t *testing.T) {
	client := &SheetsClient{}

	tests := []struct {
		name     string
		headers  map[string]int
		required []string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "all required columns present",
			headers: map[string]int{
				"anki id":        0,
				"english":        1,
				"greek":          2,
				"part of speech": 3,
			},
			required: []string{"Anki ID", "English", "Greek", "Part of Speech"},
			wantErr:  false,
		},
		{
			name: "case insensitive matching",
			headers: map[string]int{
				"anki id": 0,
				"english": 1,
				"greek":   2,
			},
			required: []string{"ANKI ID", "ENGLISH", "GREEK"},
			wantErr:  false,
		},
		{
			name: "missing one required column",
			headers: map[string]int{
				"anki id": 0,
				"english": 1,
			},
			required: []string{"Anki ID", "English", "Greek"},
			wantErr:  true,
			errMsg:   "Greek",
		},
		{
			name: "missing multiple required columns",
			headers: map[string]int{
				"anki id": 0,
			},
			required: []string{"Anki ID", "English", "Greek", "Part of Speech"},
			wantErr:  true,
			errMsg:   "English",
		},
		{
			name: "empty required list",
			headers: map[string]int{
				"anki id": 0,
			},
			required: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateRequiredColumns(tt.headers, tt.required)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "required columns missing")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCellUpdateStructure(t *testing.T) {
	// Test that CellUpdate struct can be created and used
	update := CellUpdate{
		Row:    5,
		Column: "A",
		Value:  int64(1234567890123),
	}

	assert.Equal(t, 5, update.Row)
	assert.Equal(t, "A", update.Column)
	assert.Equal(t, int64(1234567890123), update.Value)

	// Test with string value
	update2 := CellUpdate{
		Row:    10,
		Column: "B",
		Value:  "abc123checksum",
	}

	assert.Equal(t, 10, update2.Row)
	assert.Equal(t, "B", update2.Column)
	assert.Equal(t, "abc123checksum", update2.Value)
}

func TestBatchUpdateCells_EmptyUpdates(t *testing.T) {
	client := &SheetsClient{} // No service needed for empty updates

	// Empty updates should not error
	err := client.BatchUpdateCells("fake-sheet-id", "Sheet1", []CellUpdate{})
	assert.NoError(t, err, "Empty updates should not return an error")
}

// Note: Full integration tests for ReadSheet, BatchUpdateCells, and CreateChecksumColumnIfMissing
// would require either mocking the Google Sheets API service or using a real test sheet.
// For production code, consider using gomock or similar to mock the sheets.Service.
// These tests focus on the logic that doesn't require API calls.

func TestParseHeaders_RealWorldExample(t *testing.T) {
	client := &SheetsClient{}

	// Simulate real sheet data
	rows := [][]interface{}{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech", "Attributes", "Examples", "Tag", "Sub-Tag 1", "Sub-Tag 2"},
		{"1234567890123", "abc123", "hello", "γεια", "Interjection", "", "Hello, how are you?", "Greetings", "Basic", ""},
	}

	headers, err := client.ParseHeaders(rows)
	require.NoError(t, err)

	// Verify all expected columns are found
	expectedColumns := []string{
		"anki id", "checksum", "english", "greek", "part of speech",
		"attributes", "examples", "tag", "sub-tag 1", "sub-tag 2",
	}

	for _, col := range expectedColumns {
		_, exists := headers[col]
		assert.True(t, exists, "Expected column %s to exist", col)
	}

	// Verify column indices
	assert.Equal(t, 0, headers["anki id"])
	assert.Equal(t, 1, headers["checksum"])
	assert.Equal(t, 2, headers["english"])
	assert.Equal(t, 3, headers["greek"])
	assert.Equal(t, 4, headers["part of speech"])
}

func TestValidateRequiredColumns_RealWorldExample(t *testing.T) {
	client := &SheetsClient{}

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

	// Validate the three required columns
	requiredColumns := []string{"English", "Greek", "Part of Speech"}
	err := client.ValidateRequiredColumns(headers, requiredColumns)
	assert.NoError(t, err)

	// Test with missing required column
	incompleteHeaders := map[string]int{
		"anki id": 0,
		"english": 1,
		// Missing "greek" and "part of speech"
	}

	err = client.ValidateRequiredColumns(incompleteHeaders, requiredColumns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Greek")
	assert.Contains(t, err.Error(), "Part of Speech")
}
