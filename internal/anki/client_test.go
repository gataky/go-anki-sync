package anki

import (
	"regexp"
	"testing"

	"github.com/gataky/sync/pkg/models"
	"github.com/stretchr/testify/assert"
)

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
			name:     "whitespace only",
			examples: "   \n\t\n   ",
			expected: "",
		},
		{
			name:     "greek with translation",
			examples: "Θέλω λίγο νερό. — I want a little water.",
			expected: `<div class="ex"><span class="ex-el">Θέλω λίγο νερό.</span><span class="ex-en">I want a little water.</span></div>`,
		},
		{
			name:     "greek without translation",
			examples: "Αύριο εξετάζομαι στα μαθηματικά.",
			expected: `<div class="ex"><span class="ex-el">Αύριο εξετάζομαι στα μαθηματικά.</span></div>`,
		},
		{
			name:     "multiple examples with empty lines",
			examples: "Πού είναι; — Where is it?\n\nΕδώ είναι. — It is here.\n",
			expected: `<div class="ex"><span class="ex-el">Πού είναι;</span><span class="ex-en">Where is it?</span></div>` +
				`<div class="ex"><span class="ex-el">Εδώ είναι.</span><span class="ex-en">It is here.</span></div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExamplesHTML(tt.examples)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatNotesHTML(t *testing.T) {
	assert.Equal(t, "", formatNotesHTML(""))
	assert.Equal(t, "One note", formatNotesHTML("  One note  "))
	assert.Equal(t, "First<br>Second", formatNotesHTML("First\n\nSecond\n"))
}

func TestBuildNoteFields(t *testing.T) {
	card := &models.VocabCard{
		English:      "age",
		Greek:        "ηλικία",
		PartOfSpeech: "noun",
		Attributes:   "feminine",
		Examples:     "Ποια είναι η ηλικία σου; — What is your age?",
		Notes:        "Natives usually say Πόσων χρονών είσαι;",
	}

	fields := buildNoteFields(card, "ηλικία [sound:ηλικία-etts-1.mp3]")

	assert.Equal(t, "age", fields["Front"])
	assert.Equal(t, "ηλικία [sound:ηλικία-etts-1.mp3]", fields["Back"])
	assert.Equal(t, "η", fields["Article"])
	assert.Equal(t, "feminine", fields["Gender"])
	assert.Equal(t, "feminine", fields["Attributes"])
	assert.Equal(t, formatExamplesHTML(card.Examples), fields["Examples"])
	assert.Equal(t, "Natives usually say Πόσων χρονών είσαι;", fields["Notes"])

	// Every field written to a note must exist on the note type
	for name := range fields {
		assert.Contains(t, noteTypeFields, name)
	}
}

func TestNoteTypeTemplatesUseKnownFields(t *testing.T) {
	fieldRef := regexp.MustCompile(`\{\{[#/^]?([A-Za-z]+)\}\}`)
	known := map[string]bool{"FrontSide": true}
	for _, f := range noteTypeFields {
		known[f] = true
	}

	for _, tmpl := range noteTypeTemplates {
		for _, side := range []string{tmpl.Front, tmpl.Back} {
			for _, m := range fieldRef.FindAllStringSubmatch(side, -1) {
				assert.True(t, known[m[1]], "template %q references unknown field %q", tmpl.Name, m[1])
			}
		}
	}
}

func TestVocabSyncModelName(t *testing.T) {
	assert.Equal(t, "VocabSync", VocabSyncModelName)
}

// Note: Tests for AnkiClient methods that interact with the actual AnkiConnect API
// (NewAnkiClient, CheckConnection, CreateDeck, EnsureNoteType, AddNote, UpdateNoteFields,
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

// Note: Tests for audio-related methods (CheckAudioExists, StoreAudioFile, AddNote with audio)
// would require integration testing with a running Anki instance.
//
// The AddNote method signature now includes audioData []byte parameter.
// Integration tests should verify:
// - AddNote with nil audioData behaves as before (no audio attachment)
// - AddNote with non-nil audioData properly attaches audio to the note
// - AddNote properly encodes audio data to base64
// - AddNote sets correct filename and field ("Back") for audio attachment
