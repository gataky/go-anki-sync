package sync

import (
	"fmt"
	"os"
	"testing"

	"github.com/gataky/sync/internal/anki"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/internal/testutil"
	"github.com/gataky/sync/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations are in internal/testutil package

func TestNewPusher(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet-id",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	assert.NotNil(t, pusher)
	assert.Equal(t, sheetsClient, pusher.sheetsClient)
	assert.Equal(t, ankiClient, pusher.ankiClient)
	assert.Equal(t, config, pusher.config)
	assert.Equal(t, logger, pusher.logger)
}

func TestPush_NewCards(t *testing.T) {
	// Setup mock data
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"}, // Header
		{nil, "", "hello", "γεια", "Interjection"},                    // New card (no Anki ID)
		{nil, "", "house", "σπίτι", "Noun"},                           // New card (no Anki ID)
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	// Execute push
	err := pusher.Push(false)
	require.NoError(t, err)

	// Verify deck and note type created
	assert.Equal(t, "Greek", ankiClient.DeckCreated)
	assert.Equal(t, anki.VocabSyncModelName, ankiClient.NoteTypeCreated)

	// Verify cards created in Anki
	assert.Len(t, ankiClient.CreatedNotes, 2)
	assert.Equal(t, "hello", ankiClient.CreatedNotes[0].English)
	assert.Equal(t, "house", ankiClient.CreatedNotes[1].English)

	// Verify updates written to sheet (2 cards × 2 updates each = 4 updates)
	assert.Len(t, sheetsClient.BatchUpdates, 4)

	// Check that Anki IDs were written
	ankiIDUpdates := 0
	checksumUpdates := 0
	for _, update := range sheetsClient.BatchUpdates {
		if update.Column == "A" {
			ankiIDUpdates++
			assert.NotEqual(t, int64(0), update.Value)
		} else if update.Column == "B" {
			checksumUpdates++
			assert.NotEmpty(t, update.Value)
		}
	}
	assert.Equal(t, 2, ankiIDUpdates, "Should have 2 Anki ID updates")
	assert.Equal(t, 2, checksumUpdates, "Should have 2 checksum updates")
}

func TestPush_ExistingCards_NoChanges(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Create card with correct checksum
	card := &models.VocabCard{
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
	}
	mapper.UpdateChecksum(card)

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), card.StoredChecksum, "hello", "γεια", "Interjection"},
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	err := pusher.Push(false)
	require.NoError(t, err)

	// No notes should be created or updated
	assert.Len(t, ankiClient.CreatedNotes, 0)
	assert.Len(t, ankiClient.UpdatedNotes, 0)

	// No updates to sheet
	assert.Len(t, sheetsClient.BatchUpdates, 0)
}

func TestPush_ExistingCards_WithChanges(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{float64(1234567890123), "old-checksum", "hello", "γεια", "Interjection"}, // Changed content
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	err := pusher.Push(false)
	require.NoError(t, err)

	// Note should be updated
	assert.Len(t, ankiClient.UpdatedNotes, 1)
	updatedCard := ankiClient.UpdatedNotes[1234567890123]
	assert.NotNil(t, updatedCard)
	assert.Equal(t, "hello", updatedCard.English)

	// Checksum should be written to sheet
	assert.Len(t, sheetsClient.BatchUpdates, 1)
	assert.Equal(t, "B", sheetsClient.BatchUpdates[0].Column)
	assert.NotEqual(t, "old-checksum", sheetsClient.BatchUpdates[0].Value)
}

func TestPush_DryRun(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "hello", "γεια", "Interjection"}, // New card
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	// Execute dry run
	err := pusher.Push(true)
	require.NoError(t, err)

	// No notes should be created
	assert.Len(t, ankiClient.CreatedNotes, 0)

	// No updates to sheet
	assert.Len(t, sheetsClient.BatchUpdates, 0)

	// Deck and note type should not be created in dry run
	assert.Empty(t, ankiClient.DeckCreated)
	assert.Empty(t, ankiClient.NoteTypeCreated)
}

func TestPush_MixedNewAndExisting(t *testing.T) {
	headers := map[string]int{
		"anki id":        0,
		"checksum":       1,
		"english":        2,
		"greek":          3,
		"part of speech": 4,
	}

	// Create existing card with correct checksum
	existingCard := &models.VocabCard{
		English:      "existing",
		Greek:        "υπάρχον",
		PartOfSpeech: "Adjective",
	}
	mapper.UpdateChecksum(existingCard)

	rows := [][]any{
		{"Anki ID", "Checksum", "English", "Greek", "Part of Speech"},
		{nil, "", "new", "νέο", "Adjective"},                                                      // New card
		{float64(1111111111111), existingCard.StoredChecksum, "existing", "υπάρχον", "Adjective"}, // Existing, unchanged
		{float64(2222222222222), "wrong-checksum", "changed", "αλλαγμένο", "Adjective"},           // Existing, changed
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	err := pusher.Push(false)
	require.NoError(t, err)

	// 1 new card created
	assert.Len(t, ankiClient.CreatedNotes, 1)
	assert.Equal(t, "new", ankiClient.CreatedNotes[0].English)

	// 1 card updated (the one with wrong checksum)
	assert.Len(t, ankiClient.UpdatedNotes, 1)
	assert.NotNil(t, ankiClient.UpdatedNotes[2222222222222])

	// Updates: 2 for new card (Anki ID + checksum) + 1 for updated card (checksum) = 3
	assert.Len(t, sheetsClient.BatchUpdates, 3)
}

func TestPush_EmptySheet(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
	}

	rows := [][]any{
		{"English", "Greek", "Part of Speech"}, // Header only
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	err := pusher.Push(false)
	require.NoError(t, err)

	// No cards should be processed
	assert.Len(t, ankiClient.CreatedNotes, 0)
	assert.Len(t, sheetsClient.BatchUpdates, 0)
}

func TestPush_InvalidRow(t *testing.T) {
	headers := map[string]int{
		"english":        0,
		"greek":          1,
		"part of speech": 2,
	}

	rows := [][]any{
		{"English", "Greek", "Part of Speech"},
		{"hello", "", "Interjection"}, // Missing required Greek field
	}

	sheetsClient := &testutil.MockSheetsClient{
		Rows:    rows,
		Headers: headers,
	}
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		GoogleSheetID: "test-sheet",
		SheetName:     "Vocabulary",
		AnkiDeck:      "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, nil)

	// Should fail fast on validation error
	err := pusher.Push(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "row 2")
	assert.Contains(t, err.Error(), "Greek")
}

func TestCreateNewCards(t *testing.T) {
	ankiClient := testutil.NewMockAnkiClient()
	config := &models.Config{
		AnkiDeck: "Greek",
	}
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := &Pusher{
		ankiClient: ankiClient,
		config:     config,
		logger:     logger,
	}

	cards := []*models.VocabCard{
		{RowNumber: 2, English: "hello", Greek: "γεια", PartOfSpeech: "Interjection"},
		{RowNumber: 3, English: "house", Greek: "σπίτι", PartOfSpeech: "Noun"},
	}

	headers := map[string]int{
		"anki id": 0,
		"checksum": 1,
		"english": 2,
		"greek": 3,
	}

	updates, err := pusher.createNewCards(cards, headers, false)
	require.NoError(t, err)

	// Should have 4 updates (2 cards × 2 fields each)
	assert.Len(t, updates, 4)

	// Verify Anki IDs were assigned
	assert.NotEqual(t, int64(0), cards[0].AnkiID)
	assert.NotEqual(t, int64(0), cards[1].AnkiID)

	// Verify checksums were set
	assert.NotEmpty(t, cards[0].StoredChecksum)
	assert.NotEmpty(t, cards[1].StoredChecksum)
}

func TestUpdateExistingCards(t *testing.T) {
	ankiClient := testutil.NewMockAnkiClient()
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := &Pusher{
		ankiClient: ankiClient,
		logger:     logger,
	}

	// Card with wrong checksum (changed)
	changedCard := &models.VocabCard{
		RowNumber:      2,
		AnkiID:         1234567890123,
		StoredChecksum: "old-checksum",
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
	}

	// Card with correct checksum (unchanged)
	unchangedCard := &models.VocabCard{
		RowNumber:    3,
		AnkiID:       9876543210987,
		English:      "house",
		Greek:        "σπίτι",
		PartOfSpeech: "Noun",
	}
	mapper.UpdateChecksum(unchangedCard)

	cards := []*models.VocabCard{changedCard, unchangedCard}

	headers := map[string]int{
		"anki id": 0,
		"checksum": 1,
		"english": 2,
		"greek": 3,
	}

	updates, err := pusher.updateExistingCards(cards, headers, false)
	require.NoError(t, err)

	// Should have 1 update (only changed card)
	assert.Len(t, updates, 1)
	// Note: CellUpdate.Row is 1-indexed excluding header, so card RowNumber=2 becomes Row=1
	assert.Equal(t, 1, updates[0].Row) // Changed card's row

	// Verify only changed card was updated in Anki
	assert.Len(t, ankiClient.UpdatedNotes, 1)
	assert.NotNil(t, ankiClient.UpdatedNotes[1234567890123])
}

// Mock TTS client is in testutil package

func TestGenerateAudioForCard_Success(t *testing.T) {
	config := &models.Config{
		AnkiDeck: "Greek",
		TextToSpeech: &models.TTSConfig{
			Enabled: true,
		},
	}

	ankiClient := testutil.NewMockAnkiClient()
	ttsClient := testutil.NewMockTTSClient()
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := &Pusher{
		ankiClient: ankiClient,
		ttsClient:  ttsClient,
		config:     config,
		logger:     logger,
	}

	card := &models.VocabCard{
		English:   "hello",
		Greek:     "γεια",
		RowNumber: 2,
	}

	audioData, _ := pusher.generateAudioForCard(card)

	assert.NotNil(t, audioData)
	assert.Equal(t, []byte("audio-for-γεια"), audioData)
	assert.Equal(t, 1, len(ttsClient.AudioGenerated))
}

func TestGenerateAudioForCard_EmptyGreek(t *testing.T) {
	config := &models.Config{
		AnkiDeck: "Greek",
		TextToSpeech: &models.TTSConfig{
			Enabled: true,
		},
	}

	ankiClient := testutil.NewMockAnkiClient()
	ttsClient := testutil.NewMockTTSClient()
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := &Pusher{
		ankiClient: ankiClient,
		ttsClient:  ttsClient,
		config:     config,
		logger:     logger,
	}

	card := &models.VocabCard{
		English:   "hello",
		Greek:     "",
		RowNumber: 2,
	}

	audioData, _ := pusher.generateAudioForCard(card)

	assert.Nil(t, audioData)
	assert.Equal(t, 0, len(ttsClient.AudioGenerated))
}

// Note: Audio existence check was removed for simplicity.
// AnkiConnect handles duplicate media files by overwriting them.
// This avoids API compatibility issues with CheckAudioExists.

func TestGenerateAudioForCard_TTSError(t *testing.T) {
	config := &models.Config{
		AnkiDeck: "Greek",
		TextToSpeech: &models.TTSConfig{
			Enabled: true,
		},
	}

	ankiClient := testutil.NewMockAnkiClient()
	ttsClient := testutil.NewMockTTSClient()
	ttsClient.GenerateError = fmt.Errorf("TTS API error")
	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := &Pusher{
		ankiClient: ankiClient,
		ttsClient:  ttsClient,
		config:     config,
		logger:     logger,
	}

	card := &models.VocabCard{
		English:   "hello",
		Greek:     "γεια",
		RowNumber: 2,
	}

	audioData, _ := pusher.generateAudioForCard(card)

	// Should return nil on error (graceful degradation)
	assert.Nil(t, audioData)
}

func TestCreateNewCards_WithAudio(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{}
	ankiClient := testutil.NewMockAnkiClient()
	ttsClient := testutil.NewMockTTSClient()

	config := &models.Config{
		AnkiDeck: "Greek",
		TextToSpeech: &models.TTSConfig{
			Enabled:        true,
			RequestDelayMs: 0, // No delay in tests
		},
	}

	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, ttsClient)

	cards := []*models.VocabCard{
		{
			RowNumber:    2,
			English:      "hello",
			Greek:        "γεια",
			PartOfSpeech: "Interjection",
		},
	}

	headers := map[string]int{
		"anki id": 0,
		"checksum": 1,
		"english": 2,
		"greek": 3,
	}

	updates, err := pusher.createNewCards(cards, headers, false)

	require.NoError(t, err)
	assert.Len(t, updates, 2) // Anki ID + Checksum

	// Verify TTS was called
	assert.Equal(t, 1, len(ttsClient.AudioGenerated))
	assert.NotNil(t, ttsClient.AudioGenerated["γεια"])

	// Verify card was created
	assert.Len(t, ankiClient.CreatedNotes, 1)
}

func TestCreateNewCards_TTSDisabled(t *testing.T) {
	sheetsClient := &testutil.MockSheetsClient{}
	ankiClient := testutil.NewMockAnkiClient()
	ttsClient := testutil.NewMockTTSClient()

	config := &models.Config{
		AnkiDeck: "Greek",
		TextToSpeech: &models.TTSConfig{
			Enabled: false, // Disabled
		},
	}

	logger := logging.NewSyncLogger(logging.Silent, os.Stdout)

	pusher := NewPusher(sheetsClient, ankiClient, config, logger, ttsClient)

	cards := []*models.VocabCard{
		{
			RowNumber:    2,
			English:      "hello",
			Greek:        "γεια",
			PartOfSpeech: "Interjection",
		},
	}

	headers := map[string]int{
		"anki id": 0,
		"checksum": 1,
		"english": 2,
		"greek": 3,
	}

	updates, err := pusher.createNewCards(cards, headers, false)

	require.NoError(t, err)
	assert.Len(t, updates, 2) // Anki ID + Checksum

	// Verify TTS was NOT called
	assert.Equal(t, 0, len(ttsClient.AudioGenerated))

	// Verify card was still created
	assert.Len(t, ankiClient.CreatedNotes, 1)
}

func TestPusher_GetProviderSource(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{
			name:     "elevenlabs provider",
			provider: "elevenlabs",
			want:     "etts",
		},
		{
			name:     "google provider",
			provider: "google",
			want:     "gtts",
		},
		{
			name:     "empty provider defaults to elevenlabs",
			provider: "",
			want:     "etts",
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &models.Config{
				TextToSpeech: &models.TTSConfig{
					Provider: tt.provider,
					Enabled:  true,
				},
			}
			pusher := &Pusher{
				config: cfg,
			}

			got := pusher.getProviderSource()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPusher_BuildAudioFilename(t *testing.T) {
	pusher := &Pusher{}

	tests := []struct {
		name      string
		greekWord string
		source    string
		version   int
		want      string
	}{
		{
			name:      "elevenlabs version 1",
			greekWord: "γεια",
			source:    "etts",
			version:   1,
			want:      "γεια-etts-1.mp3",
		},
		{
			name:      "google version 2",
			greekWord: "γεια",
			source:    "gtts",
			version:   2,
			want:      "γεια-gtts-2.mp3",
		},
		{
			name:      "complex greek word",
			greekWord: "Καλημέρα",
			source:    "etts",
			version:   5,
			want:      "Καλημέρα-etts-5.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pusher.buildAudioFilename(tt.greekWord, tt.source, tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}
