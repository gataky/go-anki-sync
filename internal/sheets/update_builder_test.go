package sheets

import (
	"testing"

	"github.com/gataky/sync/pkg/models"
)

func TestBuildCardUpdate(t *testing.T) {
	tests := []struct {
		name          string
		card          *models.VocabCard
		includeAnkiID bool
		wantLen       int
		wantAnkiID    bool
	}{
		{
			name: "full card with AnkiID",
			card: &models.VocabCard{
				RowNumber:      5,
				AnkiID:         12345,
				StoredChecksum: "abc123",
				English:        "hello",
				Greek:          "γεια",
				PartOfSpeech:   "noun",
				Attributes:     "common",
				Examples:       "example text",
				Tag:            "greetings",
				SubTag1:        "informal",
				SubTag2:        "basic",
			},
			includeAnkiID: true,
			wantLen:       10,
			wantAnkiID:    true,
		},
		{
			name: "full card without AnkiID",
			card: &models.VocabCard{
				RowNumber:      3,
				AnkiID:         0,
				StoredChecksum: "def456",
				English:        "goodbye",
				Greek:          "αντίο",
				PartOfSpeech:   "interjection",
				Attributes:     "",
				Examples:       "",
				Tag:            "",
				SubTag1:        "",
				SubTag2:        "",
			},
			includeAnkiID: false,
			wantLen:       9,
			wantAnkiID:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates := BuildCardUpdate(tt.card, tt.includeAnkiID)

			if len(updates) != tt.wantLen {
				t.Errorf("BuildCardUpdate() returned %d updates, want %d", len(updates), tt.wantLen)
			}

			// Check row number (should be 0-indexed)
			expectedRow := tt.card.RowNumber - 1
			for _, update := range updates {
				if update.Row != expectedRow {
					t.Errorf("Update has row %d, want %d", update.Row, expectedRow)
				}
			}

			// Check if AnkiID is included
			hasAnkiID := false
			for _, update := range updates {
				if update.Column == ColAnkiID {
					hasAnkiID = true
					if update.Value != tt.card.AnkiID {
						t.Errorf("AnkiID update has value %v, want %v", update.Value, tt.card.AnkiID)
					}
				}
			}
			if hasAnkiID != tt.wantAnkiID {
				t.Errorf("AnkiID included = %v, want %v", hasAnkiID, tt.wantAnkiID)
			}

			// Verify all expected columns are present
			expectedColumns := map[string]any{
				ColChecksum:     tt.card.StoredChecksum,
				ColEnglish:      tt.card.English,
				ColGreek:        tt.card.Greek,
				ColPartOfSpeech: tt.card.PartOfSpeech,
				ColAttributes:   tt.card.Attributes,
				ColExamples:     tt.card.Examples,
				ColTag:          tt.card.Tag,
				ColSubTag1:      tt.card.SubTag1,
				ColSubTag2:      tt.card.SubTag2,
			}

			for _, update := range updates {
				if update.Column == ColAnkiID {
					continue // Already checked
				}
				expectedValue, exists := expectedColumns[update.Column]
				if !exists {
					t.Errorf("Unexpected column %s in updates", update.Column)
					continue
				}
				if update.Value != expectedValue {
					t.Errorf("Column %s has value %v, want %v", update.Column, update.Value, expectedValue)
				}
			}
		})
	}
}

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
	updates := BuildRegenTTSClearUpdate(5)

	if len(updates) != 1 {
		t.Fatalf("BuildRegenTTSClearUpdate() returned %d updates, want 1", len(updates))
	}

	update := updates[0]

	// Check row number
	if update.Row != 5 {
		t.Errorf("Update has row %d, want %d", update.Row, 5)
	}

	// Check column
	if update.Column != "Regen TTS" {
		t.Errorf("Update column is %s, want %s", update.Column, "Regen TTS")
	}

	// Check value
	if update.Value != "" {
		t.Errorf("Update value is %v, want empty string", update.Value)
	}
}
