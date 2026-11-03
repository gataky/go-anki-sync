package mapper

import (
	"testing"

	"github.com/gataky/sync/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestNounArticle(t *testing.T) {
	tests := []struct {
		name         string
		greek        string
		partOfSpeech string
		attributes   string
		wantGender   string
		wantArticle  string
		wantSpoken   string
	}{
		{"masculine", "γείτονας", "noun", "masculine", GenderMasculine, "ο", "ο γείτονας"},
		{"feminine", "ηλικία", "noun", "feminine", GenderFeminine, "η", "η ηλικία"},
		{"neuter", "απόγευμα", "Noun", "Neuter", GenderNeuter, "το", "το απόγευμα"},
		{"masculine plural", "γονείς", "noun", "masculine plural", GenderMasculine, "οι", "οι γονείς"},
		{"neuter plural", "χρήματα", "noun", "neuter, plural", GenderNeuter, "τα", "τα χρήματα"},
		{"proper noun", "Αθήνα", "proper noun", "feminine", GenderFeminine, "η", "η Αθήνα"},
		{"article already present", "το απόγευμα", "noun", "neuter", GenderNeuter, "", "το απόγευμα"},
		{"several genders", "φίλος", "noun", "masculine/feminine", "", "", "φίλος"},
		{"no gender", "ταξίδι", "noun", "", "", "", "ταξίδι"},
		{"not a noun", "λίγο", "adverb", "", "", "", "λίγο"},
		{"pronoun is not a noun", "αυτός", "pronoun", "masculine", "", "", "αυτός"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &models.VocabCard{Greek: tt.greek, PartOfSpeech: tt.partOfSpeech, Attributes: tt.attributes}
			assert.Equal(t, tt.wantGender, NounGender(card))
			assert.Equal(t, tt.wantArticle, NounArticle(card))
			assert.Equal(t, tt.wantSpoken, SpokenGreek(card))
		})
	}
}
