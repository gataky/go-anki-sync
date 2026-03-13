package sync

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/pkg/models"
)

// TestCreateNewCards_PartialFailure tests that partial results are written even when some cards fail
func TestCreateNewCards_PartialFailure(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	// Create mock client that fails on specific cards
	ankiClient := &mockAnkiClient{
		nextNoteID: 1000000000000,
		failCards:  map[string]bool{"fail-card": true}, // This card will fail
	}

	sheetsClient := &mockSheetsClient{}

	config := &models.Config{
		AnkiDeck: "Greek",
	}

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	// Test cards - one will succeed, one will fail, one will succeed
	cards := []*models.VocabCard{
		{RowNumber: 2, English: "success1", Greek: "επιτυχία1", PartOfSpeech: "Noun"},
		{RowNumber: 3, English: "fail-card", Greek: "αποτυχία", PartOfSpeech: "Noun"},
		{RowNumber: 4, English: "success2", Greek: "επιτυχία2", PartOfSpeech: "Noun"},
	}

	updates, err := pusher.createNewCards(cards, false)

	// Should return an error (because one card failed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail-card")

	// But should still return partial updates for successful cards
	assert.Len(t, updates, 4) // 2 successful cards × 2 updates each (ID + checksum)

	// Verify successful cards got IDs
	assert.Equal(t, 1, updates[0].Row) // First card (row 2 → 1)
	assert.Equal(t, "A", updates[0].Column)
	assert.Equal(t, int64(1000000000000), updates[0].Value)

	assert.Equal(t, 3, updates[2].Row) // Third card (row 4 → 3)
	assert.Equal(t, "A", updates[2].Column)
	assert.Equal(t, int64(1000000000001), updates[2].Value)

	// Verify only 2 notes were created
	assert.Len(t, ankiClient.createdNotes, 2)
}

// TestUpdateExistingCards_PartialFailure tests that partial results are written even when some updates fail
func TestUpdateExistingCards_PartialFailure(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	// Create mock client that fails on specific note IDs
	ankiClient := &mockAnkiClient{
		updatedNotes: make(map[int64]*models.VocabCard),
		failUpdates:  map[int64]bool{2222222222222: true}, // This update will fail
	}

	sheetsClient := &mockSheetsClient{}

	config := &models.Config{
		AnkiDeck: "Greek",
	}

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	// Test cards - all changed, but one update will fail
	card1 := &models.VocabCard{
		RowNumber:    2,
		AnkiID:       1111111111111,
		English:      "success1",
		Greek:        "επιτυχία1",
		PartOfSpeech: "Noun",
	}
	mapper.UpdateChecksum(card1)
	card1.English = "changed1" // Modify to trigger change detection

	card2 := &models.VocabCard{
		RowNumber:    3,
		AnkiID:       2222222222222,
		English:      "fail-update",
		Greek:        "αποτυχία",
		PartOfSpeech: "Noun",
	}
	mapper.UpdateChecksum(card2)
	card2.English = "changed-fail" // Modify to trigger change detection

	card3 := &models.VocabCard{
		RowNumber:    4,
		AnkiID:       3333333333333,
		English:      "success2",
		Greek:        "επιτυχία2",
		PartOfSpeech: "Noun",
	}
	mapper.UpdateChecksum(card3)
	card3.English = "changed2" // Modify to trigger change detection

	cards := []*models.VocabCard{card1, card2, card3}

	updates, err := pusher.updateExistingCards(cards, false)

	// Should return an error (because one update failed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2222222222222")

	// But should still return partial updates for successful cards
	assert.Len(t, updates, 2) // 2 successful updates

	// Verify successful cards got checksum updates
	assert.Equal(t, 1, updates[0].Row) // First card (row 2 → 1)
	assert.Equal(t, "B", updates[0].Column)

	assert.Equal(t, 3, updates[1].Row) // Third card (row 4 → 3)
	assert.Equal(t, "B", updates[1].Column)

	// Verify only 2 notes were updated
	assert.Len(t, ankiClient.updatedNotes, 2)
	assert.NotNil(t, ankiClient.updatedNotes[1111111111111])
	assert.NotNil(t, ankiClient.updatedNotes[3333333333333])
	assert.Nil(t, ankiClient.updatedNotes[2222222222222]) // Failed update
}

// TestPush_WritesPartialResultsOnError tests that the main Push function writes partial results
func TestPush_WritesPartialResultsOnError(t *testing.T) {
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	// Create mock client that fails on specific cards
	ankiClient := &mockAnkiClient{
		nextNoteID: 1000000000000,
		failCards:  map[string]bool{"fail-card": true},
		deckExists: true,
		modelExists: true,
	}

	sheetsClient := &mockSheetsClient{
		rows: [][]interface{}{
			{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
			{"", "", "success1", "επιτυχία1", "Noun"},
			{"", "", "fail-card", "αποτυχία", "Noun"},
			{"", "", "success2", "επιτυχία2", "Noun"},
		},
		headers: map[string]int{
			"anki id":        0,
			"checksum":       1,
			"english":        2,
			"greek":          3,
			"part of speech": 4,
		},
	}

	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Sheet1",
		AnkiDeck:      "Greek",
	}

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	err := pusher.Push(false)

	// Should return an error (because one card failed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail-card")

	// But should still have written updates to sheet for successful cards
	require.NotNil(t, sheetsClient.lastBatchUpdate)
	assert.Len(t, sheetsClient.lastBatchUpdate, 4) // 2 successful cards × 2 updates each

	// Verify the successful cards' Anki IDs were written
	hasAnkiID1 := false
	hasAnkiID2 := false
	for _, update := range sheetsClient.lastBatchUpdate {
		if update.Column == "A" && update.Row == 1 {
			hasAnkiID1 = true
		}
		if update.Column == "A" && update.Row == 3 {
			hasAnkiID2 = true
		}
	}
	assert.True(t, hasAnkiID1, "First successful card should have Anki ID written")
	assert.True(t, hasAnkiID2, "Second successful card should have Anki ID written")
}
