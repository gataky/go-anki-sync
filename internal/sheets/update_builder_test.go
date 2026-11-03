package sheets

import (
	"testing"

)

func TestBuildAnkiIDAndChecksumUpdate(t *testing.T) {
	tests := []struct {
		name      string
		rowNumber int
		ankiID    int64
		checksum  string
	}{
		{
			name:      "row 5",
			rowNumber: 5,
			ankiID:    12345,
			checksum:  "abc123",
		},
		{
			name:      "row 1",
			rowNumber: 1,
			ankiID:    99999,
			checksum:  "xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates := BuildAnkiIDAndChecksumUpdate(tt.rowNumber, tt.ankiID, tt.checksum)

			if len(updates) != 2 {
				t.Fatalf("BuildAnkiIDAndChecksumUpdate() returned %d updates, want 2", len(updates))
			}

			// Check row number (should be 0-indexed)
			expectedRow := tt.rowNumber - 1
			for _, update := range updates {
				if update.Row != expectedRow {
					t.Errorf("Update has row %d, want %d", update.Row, expectedRow)
				}
			}

			// Check AnkiID update
			ankiIDUpdate := updates[0]
			if ankiIDUpdate.Column != ColAnkiID {
				t.Errorf("First update column is %s, want %s", ankiIDUpdate.Column, ColAnkiID)
			}
			if ankiIDUpdate.Value != tt.ankiID {
				t.Errorf("AnkiID value is %v, want %v", ankiIDUpdate.Value, tt.ankiID)
			}

			// Check Checksum update
			checksumUpdate := updates[1]
			if checksumUpdate.Column != ColChecksum {
				t.Errorf("Second update column is %s, want %s", checksumUpdate.Column, ColChecksum)
			}
			if checksumUpdate.Value != tt.checksum {
				t.Errorf("Checksum value is %v, want %v", checksumUpdate.Value, tt.checksum)
			}
		})
	}
}

func TestBuildChecksumOnlyUpdate(t *testing.T) {
	tests := []struct {
		name      string
		rowNumber int
		checksum  string
	}{
		{
			name:      "row 3",
			rowNumber: 3,
			checksum:  "checksum123",
		},
		{
			name:      "row 10",
			rowNumber: 10,
			checksum:  "another_checksum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates := BuildChecksumOnlyUpdate(tt.rowNumber, tt.checksum)

			if len(updates) != 1 {
				t.Fatalf("BuildChecksumOnlyUpdate() returned %d updates, want 1", len(updates))
			}

			update := updates[0]

			// Check row number (should be 0-indexed)
			expectedRow := tt.rowNumber - 1
			if update.Row != expectedRow {
				t.Errorf("Update has row %d, want %d", update.Row, expectedRow)
			}

			// Check column
			if update.Column != ColChecksum {
				t.Errorf("Update column is %s, want %s", update.Column, ColChecksum)
			}

			// Check value
			if update.Value != tt.checksum {
				t.Errorf("Update value is %v, want %v", update.Value, tt.checksum)
			}
		})
	}
}

func TestBuildRegenTTSClearUpdate(t *testing.T) {
	// Create headers map with "regen tts" at column index 10 (column K)
	headers := map[string]int{
		"anki id":       0,
		"checksum":      1,
		"english":       2,
		"greek":         3,
		"part of speech": 4,
		"attributes":    5,
		"examples":      6,
		"tag":           7,
		"sub-tag 1":     8,
		"sub-tag 2":     9,
		"regen tts":     10,
	}

	updates := BuildRegenTTSClearUpdate(5, headers)

	if len(updates) != 1 {
		t.Fatalf("BuildRegenTTSClearUpdate() returned %d updates, want 1", len(updates))
	}

	update := updates[0]

	// Check row number (rowNumber 5 becomes 4 in 0-indexed)
	if update.Row != 4 {
		t.Errorf("Update has row %d, want %d", update.Row, 4)
	}

	// Check column (column index 10 = K)
	if update.Column != "K" {
		t.Errorf("Update column is %s, want K", update.Column)
	}

	// Check value
	if update.Value != "" {
		t.Errorf("Update value is %v, want empty string", update.Value)
	}
}

func TestBuildRegenTTSClearUpdate_MissingColumn(t *testing.T) {
	// Create headers map without "regen tts" column
	headers := map[string]int{
		"anki id":       0,
		"checksum":      1,
		"english":       2,
		"greek":         3,
	}

	updates := BuildRegenTTSClearUpdate(5, headers)

	// Should return nil if column doesn't exist
	if updates != nil {
		t.Errorf("BuildRegenTTSClearUpdate() returned %v, want nil when column missing", updates)
	}
}
