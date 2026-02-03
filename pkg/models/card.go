package models

import "time"

// VocabCard represents a vocabulary card with all its fields.
// This is the core domain model that bridges Google Sheets and Anki.
type VocabCard struct {
	// RowNumber is the row number in the Google Sheet (1-indexed, excluding header)
	RowNumber int

	// AnkiID is the 13-digit Anki note ID (0 if not yet created)
	AnkiID int64

	// StoredChecksum is the SHA256 checksum from the Sheet column (for comparison)
	StoredChecksum string

	// English is the primary English word(s) - Required field
	English string

	// Greek is the primary Greek word(s) - Required field
	Greek string

	// PartOfSpeech is the grammatical category (Noun, Verb, Adjective, etc.) - Required field
	PartOfSpeech string

	// Attributes contains gender for nouns (Masculine/Feminine/Neuter) or class for verbs - Optional
	Attributes string

	// Examples contains usage examples (raw text, will be converted to HTML) - Optional
	Examples string

	// Tag is the top-level category - Optional
	Tag string

	// SubTag1 is the second-level category - Optional
	SubTag1 string

	// SubTag2 is the third-level category - Optional
	SubTag2 string

	// ModifiedAt is the last modification timestamp
	ModifiedAt time.Time
}
