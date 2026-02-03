package mapper

import (
	"crypto/sha256"
	"fmt"

	"github.com/yourusername/sync/pkg/models"
)

// CalculateChecksum computes a SHA256 hash of the content fields of a VocabCard.
// Only content fields are included in the checksum:
// - English, Greek, PartOfSpeech, Attributes, Examples, Tag, SubTag1, SubTag2
// Excluded fields (not part of checksum):
// - RowNumber: positional metadata
// - AnkiID: external identifier
// - StoredChecksum: the checksum itself
// - ModifiedAt: timestamp metadata
//
// The checksum is computed as SHA256 of concatenated field values.
// Returns a hex-encoded string.
func CalculateChecksum(card *models.VocabCard) string {
	// Concatenate content fields in a consistent order
	content := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		card.English,
		card.Greek,
		card.PartOfSpeech,
		card.Attributes,
		card.Examples,
		card.Tag,
		card.SubTag1,
		card.SubTag2,
	)

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(content))

	// Return hex-encoded string
	return fmt.Sprintf("%x", hash)
}

// HasChanged compares the stored checksum with a freshly computed checksum
// to determine if the card content has changed.
func HasChanged(card *models.VocabCard) bool {
	currentChecksum := CalculateChecksum(card)
	return card.StoredChecksum != currentChecksum
}

// UpdateChecksum calculates and updates the StoredChecksum field of the card.
func UpdateChecksum(card *models.VocabCard) {
	card.StoredChecksum = CalculateChecksum(card)
}
