package mapper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/gataky/sync/pkg/models"
)

func TestCalculateChecksum(t *testing.T) {
	tests := []struct {
		name     string
		card     *models.VocabCard
		expected string // First 16 chars of expected checksum for readability
	}{
		{
			name: "basic card",
			card: &models.VocabCard{
				English:      "hello",
				Greek:        "γεια",
				PartOfSpeech: "Interjection",
				Attributes:   "",
				Examples:     "Hello, how are you?",
				Tag:          "Greetings",
				SubTag1:      "Basic",
				SubTag2:      "",
			},
			// We'll validate it's consistent, not the exact value
			expected: "",
		},
		{
			name: "card with all fields populated",
			card: &models.VocabCard{
				English:      "house",
				Greek:        "σπίτι",
				PartOfSpeech: "Noun",
				Attributes:   "Neuter",
				Examples:     "This is my house.\nΑυτό είναι το σπίτι μου.",
				Tag:          "Buildings",
				SubTag1:      "Residential",
				SubTag2:      "Common",
			},
			expected: "",
		},
		{
			name: "minimal card with required fields only",
			card: &models.VocabCard{
				English:      "test",
				Greek:        "τεστ",
				PartOfSpeech: "Noun",
				Attributes:   "",
				Examples:     "",
				Tag:          "",
				SubTag1:      "",
				SubTag2:      "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum := CalculateChecksum(tt.card)

			// Checksum should be 64 characters (SHA256 hex)
			assert.Len(t, checksum, 64, "Checksum should be 64 hex characters")

			// Checksum should be consistent
			checksum2 := CalculateChecksum(tt.card)
			assert.Equal(t, checksum, checksum2, "Checksum should be deterministic")
		})
	}
}

func TestCalculateChecksum_ExcludesMetadataFields(t *testing.T) {
	// Two cards with same content but different metadata
	card1 := &models.VocabCard{
		RowNumber:      1,
		AnkiID:         1234567890123,
		StoredChecksum: "old-checksum",
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
		ModifiedAt:     time.Now(),
	}

	card2 := &models.VocabCard{
		RowNumber:      999, // Different row
		AnkiID:         9876543210987, // Different Anki ID
		StoredChecksum: "different-old-checksum", // Different stored checksum
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
		ModifiedAt:     time.Now().Add(24 * time.Hour), // Different timestamp
	}

	checksum1 := CalculateChecksum(card1)
	checksum2 := CalculateChecksum(card2)

	// Checksums should be identical because content is the same
	assert.Equal(t, checksum1, checksum2,
		"Checksums should ignore metadata fields (RowNumber, AnkiID, StoredChecksum, ModifiedAt)")
}

func TestCalculateChecksum_SensitiveToContentChanges(t *testing.T) {
	baseCard := &models.VocabCard{
		English:      "hello",
		Greek:        "γεια",
		PartOfSpeech: "Interjection",
		Attributes:   "",
		Examples:     "Hello there",
		Tag:          "Greetings",
		SubTag1:      "",
		SubTag2:      "",
	}
	baseChecksum := CalculateChecksum(baseCard)

	tests := []struct {
		name   string
		modify func(*models.VocabCard)
	}{
		{
			name: "change English",
			modify: func(c *models.VocabCard) {
				c.English = "hi"
			},
		},
		{
			name: "change Greek",
			modify: func(c *models.VocabCard) {
				c.Greek = "γειά"
			},
		},
		{
			name: "change PartOfSpeech",
			modify: func(c *models.VocabCard) {
				c.PartOfSpeech = "Noun"
			},
		},
		{
			name: "change Attributes",
			modify: func(c *models.VocabCard) {
				c.Attributes = "Informal"
			},
		},
		{
			name: "change Examples",
			modify: func(c *models.VocabCard) {
				c.Examples = "Different example"
			},
		},
		{
			name: "change Tag",
			modify: func(c *models.VocabCard) {
				c.Tag = "Different"
			},
		},
		{
			name: "change SubTag1",
			modify: func(c *models.VocabCard) {
				c.SubTag1 = "Advanced"
			},
		},
		{
			name: "change SubTag2",
			modify: func(c *models.VocabCard) {
				c.SubTag2 = "Rare"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the base card
			modifiedCard := *baseCard
			tt.modify(&modifiedCard)

			modifiedChecksum := CalculateChecksum(&modifiedCard)
			assert.NotEqual(t, baseChecksum, modifiedChecksum,
				"Checksum should change when content field is modified")
		})
	}
}

func TestCalculateChecksum_EmptyFields(t *testing.T) {
	card1 := &models.VocabCard{
		English:      "test",
		Greek:        "τεστ",
		PartOfSpeech: "Noun",
		Attributes:   "",
		Examples:     "",
		Tag:          "",
		SubTag1:      "",
		SubTag2:      "",
	}

	card2 := &models.VocabCard{
		English:      "test",
		Greek:        "τεστ",
		PartOfSpeech: "Noun",
		// Attributes, Examples, Tag, SubTag1, SubTag2 are zero values
	}

	checksum1 := CalculateChecksum(card1)
	checksum2 := CalculateChecksum(card2)

	// Both should produce the same checksum (empty string vs zero value)
	assert.Equal(t, checksum1, checksum2,
		"Empty strings and zero values should produce same checksum")
}

func TestHasChanged(t *testing.T) {
	tests := []struct {
		name     string
		card     *models.VocabCard
		expected bool
	}{
		{
			name: "unchanged card",
			card: &models.VocabCard{
				English:        "hello",
				Greek:          "γεια",
				PartOfSpeech:   "Interjection",
				StoredChecksum: "", // Will be set correctly
			},
			expected: false,
		},
		{
			name: "changed card",
			card: &models.VocabCard{
				English:        "hello",
				Greek:          "γεια",
				PartOfSpeech:   "Interjection",
				StoredChecksum: "incorrect-checksum",
			},
			expected: true,
		},
		{
			name: "card with no stored checksum",
			card: &models.VocabCard{
				English:        "hello",
				Greek:          "γεια",
				PartOfSpeech:   "Interjection",
				StoredChecksum: "",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the "unchanged" test, set the correct checksum
			if !tt.expected && tt.card.StoredChecksum == "" {
				tt.card.StoredChecksum = CalculateChecksum(tt.card)
			}

			result := HasChanged(tt.card)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateChecksum(t *testing.T) {
	card := &models.VocabCard{
		English:        "hello",
		Greek:          "γεια",
		PartOfSpeech:   "Interjection",
		StoredChecksum: "old-checksum",
	}

	// Update the checksum
	UpdateChecksum(card)

	// Verify the StoredChecksum was updated
	assert.NotEqual(t, "old-checksum", card.StoredChecksum)
	assert.Len(t, card.StoredChecksum, 64)

	// Verify it matches a fresh calculation
	expected := CalculateChecksum(card)
	assert.Equal(t, expected, card.StoredChecksum)

	// Verify HasChanged now returns false
	assert.False(t, HasChanged(card))
}

func TestChecksumConsistency(t *testing.T) {
	// Same content should always produce same checksum across multiple runs
	card := &models.VocabCard{
		English:      "consistency test",
		Greek:        "δοκιμή συνέπειας",
		PartOfSpeech: "Noun",
		Attributes:   "Feminine",
		Examples:     "Example 1\nExample 2",
		Tag:          "Test",
		SubTag1:      "Unit",
		SubTag2:      "Checksum",
	}

	checksums := make([]string, 100)
	for i := 0; i < 100; i++ {
		checksums[i] = CalculateChecksum(card)
	}

	// All checksums should be identical
	firstChecksum := checksums[0]
	for i, checksum := range checksums {
		assert.Equal(t, firstChecksum, checksum,
			"Checksum %d should match first checksum", i)
	}
}
