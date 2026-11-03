package mapper

import (
	"strings"

	"github.com/gataky/sync/pkg/models"
)

// Grammatical genders recognised in the Attributes column of a noun.
const (
	GenderMasculine = "masculine"
	GenderFeminine  = "feminine"
	GenderNeuter    = "neuter"
)

// singularArticles and pluralArticles map a gender to its nominative definite article.
var (
	singularArticles = map[string]string{GenderMasculine: "ο", GenderFeminine: "η", GenderNeuter: "το"}
	pluralArticles   = map[string]string{GenderMasculine: "οι", GenderFeminine: "οι", GenderNeuter: "τα"}
)

// NounGender returns the gender of a noun card (masculine, feminine or neuter), read from its attributes.
// Returns empty string if the card is not a noun or its attributes name zero or several genders.
func NounGender(card *models.VocabCard) string {
	pos := strings.ToLower(strings.TrimSpace(card.PartOfSpeech))
	if pos != "noun" && !strings.HasSuffix(pos, " noun") {
		return ""
	}

	attributes := strings.ToLower(card.Attributes)
	gender := ""
	for _, g := range []string{GenderMasculine, GenderFeminine, GenderNeuter} {
		if strings.Contains(attributes, g) {
			if gender != "" {
				return "" // Several genders (e.g. "masculine/feminine"), no single article applies
			}
			gender = g
		}
	}
	return gender
}

// NounArticle returns the definite article for a noun card (ο, η, το, or οι, τα for plural nouns).
// Returns empty string if the gender is unknown or the Greek text already starts with an article.
func NounArticle(card *models.VocabCard) string {
	gender := NounGender(card)
	if gender == "" || hasLeadingArticle(card.Greek) {
		return ""
	}
	if strings.Contains(strings.ToLower(card.Attributes), "plural") {
		return pluralArticles[gender]
	}
	return singularArticles[gender]
}

// SpokenGreek returns the Greek text to send to text-to-speech, prefixed with the article for nouns.
func SpokenGreek(card *models.VocabCard) string {
	greek := strings.TrimSpace(card.Greek)
	if article := NounArticle(card); article != "" {
		return article + " " + greek
	}
	return greek
}

// hasLeadingArticle reports whether the Greek text already begins with a definite article.
func hasLeadingArticle(greek string) bool {
	firstWord, _, found := strings.Cut(strings.TrimSpace(greek), " ")
	if !found {
		return false
	}
	switch strings.ToLower(firstWord) {
	case "ο", "η", "το", "οι", "τα", "ο/η":
		return true
	}
	return false
}
