package anki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/sync/pkg/models"
)

func TestBuildGrammarField(t *testing.T) {
	tests := []struct {
		name         string
		partOfSpeech string
		attributes   string
		expected     string
	}{
		{
			name:         "with attributes",
			partOfSpeech: "Noun",
			attributes:   "Masculine",
			expected:     "Noun (Masculine)",
		},
		{
			name:         "without attributes",
			partOfSpeech: "Verb",
			attributes:   "",
			expected:     "Verb",
		},
		{
			name:         "with complex attributes",
			partOfSpeech: "Verb",
			attributes:   "Class 1, Present",
			expected:     "Verb (Class 1, Present)",
		},
		{
			name:         "adjective without attributes",
			partOfSpeech: "Adjective",
			attributes:   "",
			expected:     "Adjective",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildGrammarField(tt.partOfSpeech, tt.attributes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTags(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		subTag1  string
		subTag2  string
		expected []string
	}{
		{
			name:     "single level tag",
			tag:      "City",
			subTag1:  "",
			subTag2:  "",
			expected: []string{"City"},
		},
		{
			name:     "two level tag",
			tag:      "City",
			subTag1:  "Buildings",
			subTag2:  "",
			expected: []string{"City::Buildings"},
		},
		{
			name:     "three level tag",
			tag:      "City",
			subTag1:  "Buildings",
			subTag2:  "Museums",
			expected: []string{"City::Buildings::Museums"},
		},
		{
			name:     "skip empty middle level",
			tag:      "City",
			subTag1:  "",
			subTag2:  "Transportation",
			expected: []string{"City"},
		},
		{
			name:     "no tags",
			tag:      "",
			subTag1:  "",
			subTag2:  "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTags(tt.tag, tt.subTag1, tt.subTag2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseGrammarField(t *testing.T) {
	tests := []struct {
		name             string
		grammar          string
		expectedPOS      string
		expectedAttr     string
	}{
		{
			name:         "with attributes",
			grammar:      "Noun (Masculine)",
			expectedPOS:  "Noun",
			expectedAttr: "Masculine",
		},
		{
			name:         "without attributes",
			grammar:      "Verb",
			expectedPOS:  "Verb",
			expectedAttr: "",
		},
		{
			name:         "with complex attributes",
			grammar:      "Verb (Class 1, Present)",
			expectedPOS:  "Verb",
			expectedAttr: "Class 1, Present",
		},
		{
			name:         "empty grammar",
			grammar:      "",
			expectedPOS:  "",
			expectedAttr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &models.VocabCard{}
			parseGrammarField(tt.grammar, card)
			assert.Equal(t, tt.expectedPOS, card.PartOfSpeech)
			assert.Equal(t, tt.expectedAttr, card.Attributes)
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name            string
		tags            []string
		expectedTag     string
		expectedSubTag1 string
		expectedSubTag2 string
	}{
		{
			name:            "single level",
			tags:            []string{"City"},
			expectedTag:     "City",
			expectedSubTag1: "",
			expectedSubTag2: "",
		},
		{
			name:            "two levels",
			tags:            []string{"City::Buildings"},
			expectedTag:     "City",
			expectedSubTag1: "Buildings",
			expectedSubTag2: "",
		},
		{
			name:            "three levels",
			tags:            []string{"City::Buildings::Museums"},
			expectedTag:     "City",
			expectedSubTag1: "Buildings",
			expectedSubTag2: "Museums",
		},
		{
			name:            "no tags",
			tags:            []string{},
			expectedTag:     "",
			expectedSubTag1: "",
			expectedSubTag2: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &models.VocabCard{}
			parseTags(tt.tags, card)
			assert.Equal(t, tt.expectedTag, card.Tag)
			assert.Equal(t, tt.expectedSubTag1, card.SubTag1)
			assert.Equal(t, tt.expectedSubTag2, card.SubTag2)
		})
	}
}

func TestFormatExamplesHTML(t *testing.T) {
	tests := []struct {
		name     string
		examples string
		expected string
	}{
		{
			name:     "empty examples",
			examples: "",
			expected: "",
		},
		{
			name:     "simple examples",
			examples: "Example 1",
			expected: "Example 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExamplesHTML(tt.examples)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTripGrammarField(t *testing.T) {
	tests := []struct {
		name         string
		partOfSpeech string
		attributes   string
	}{
		{"noun with gender", "Noun", "Masculine"},
		{"verb with class", "Verb", "Class 1"},
		{"adjective no attributes", "Adjective", ""},
		{"complex attributes", "Verb", "Class 2, Imperfect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build grammar field
			grammar := buildGrammarField(tt.partOfSpeech, tt.attributes)

			// Parse it back
			card := &models.VocabCard{}
			parseGrammarField(grammar, card)

			// Verify round-trip
			assert.Equal(t, tt.partOfSpeech, card.PartOfSpeech)
			assert.Equal(t, tt.attributes, card.Attributes)
		})
	}
}

func TestRoundTripTags(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		subTag1 string
		subTag2 string
	}{
		{"single level", "City", "", ""},
		{"two levels", "City", "Buildings", ""},
		{"three levels", "City", "Buildings", "Museums"},
		{"food category", "Food", "Vegetables", "Leafy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build tags
			tags := buildTags(tt.tag, tt.subTag1, tt.subTag2)

			// Parse them back
			card := &models.VocabCard{}
			parseTags(tags, card)

			// Verify round-trip
			assert.Equal(t, tt.tag, card.Tag)
			assert.Equal(t, tt.subTag1, card.SubTag1)
			assert.Equal(t, tt.subTag2, card.SubTag2)
		})
	}
}

func TestVocabSyncModelName(t *testing.T) {
	assert.Equal(t, "VocabSync", VocabSyncModelName)
}

// Note: Tests for AnkiClient methods that interact with the actual AnkiConnect API
// (NewAnkiClient, CheckConnection, CreateDeck, CreateNoteType, AddNote, UpdateNoteFields,
// DeleteNote, FindModifiedNotes, GetNotesInfo) would require either:
// 1. A running Anki instance with AnkiConnect installed (integration tests)
// 2. Mocking the ankiconnect.Client (unit tests with gomock)
//
// For production code, consider adding integration tests that can be run with:
// go test -tags=integration ./internal/anki/...
//
// These would require environment setup but provide high confidence.

func TestBuildTagsEdgeCases(t *testing.T) {
	// Test edge case: subTag2 without subTag1 should ignore subTag2
	tags := buildTags("City", "", "Transportation")
	assert.Equal(t, []string{"City"}, tags, "SubTag2 should be ignored if SubTag1 is empty")
}

func TestParseGrammarFieldEdgeCases(t *testing.T) {
	// Test malformed grammar strings
	tests := []struct {
		name         string
		grammar      string
		expectedPOS  string
		expectedAttr string
	}{
		{
			name:         "only opening paren",
			grammar:      "Noun (Masculine",
			expectedPOS:  "Noun (Masculine",
			expectedAttr: "",
		},
		{
			name:         "only closing paren",
			grammar:      "Noun Masculine)",
			expectedPOS:  "Noun Masculine)",
			expectedAttr: "",
		},
		{
			name:         "empty parentheses",
			grammar:      "Noun ()",
			expectedPOS:  "Noun",
			expectedAttr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &models.VocabCard{}
			parseGrammarField(tt.grammar, card)
			assert.Equal(t, tt.expectedPOS, card.PartOfSpeech)
			assert.Equal(t, tt.expectedAttr, card.Attributes)
		})
	}
}
