package anki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/atselvan/ankiconnect"
	"github.com/privatesquare/bkst-go-utils/utils/errors"
)

// Template names of the two card types generated for each note.
const (
	templateEnglishToGreek = "English to Greek"
	templateGreekToEnglish = "Greek to English"
)

// noteTypeFields lists the fields of the VocabSync note type in display order.
// Front holds the English text and Back holds the Greek text with its [sound:] tag.
var noteTypeFields = []string{
	"Front",
	"Back",
	"PartOfSpeech",
	"Article",
	"Gender",
	"Attributes",
	"Voice",
	"ActiveForm",
	"FutureSimple",
	"Aorist",
	"Examples",
	"Notes",
}

// Template fragments shared by both card types. Empty optional fields render nothing.
const (
	englishHTML = `<div class="prompt">{{Front}}</div>`

	greekHTML = `<div class="target {{Gender}}">{{#Article}}<span class="article">{{Article}}</span> {{/Article}}{{Back}}</div>`

	partOfSpeechHTML = `<div class="pos">{{PartOfSpeech}}</div>`

	detailsHTML = `<div class="grammar">` +
		`{{#Attributes}}<span>{{Attributes}}</span>{{/Attributes}}` +
		`{{#Voice}}<span>{{Voice}}</span>{{/Voice}}` +
		`{{#ActiveForm}}<span>active: {{ActiveForm}}</span>{{/ActiveForm}}` +
		`</div>` +
		`<div class="parts">` +
		`{{#FutureSimple}}<span><i>fut.</i> θα {{FutureSimple}}</span>{{/FutureSimple}}` +
		`{{#Aorist}}<span><i>aor.</i> {{Aorist}}</span>{{/Aorist}}` +
		`</div>` +
		`{{#Examples}}<div class="examples">{{Examples}}</div>{{/Examples}}` +
		`{{#Notes}}<div class="notes">{{Notes}}</div>{{/Notes}}`

	answerRule = `{{FrontSide}}<hr id="answer">`
)

// noteTypeTemplates defines the two card types. {{FrontSide}} does not replay the front's audio,
// so the Greek audio plays once per card: on the answer for English→Greek, on the question for Greek→English.
var noteTypeTemplates = []ankiconnect.CardTemplate{
	{
		Name:  templateEnglishToGreek,
		Front: englishHTML + partOfSpeechHTML,
		Back:  answerRule + greekHTML + detailsHTML,
	},
	{
		Name:  templateGreekToEnglish,
		Front: greekHTML + partOfSpeechHTML,
		Back:  answerRule + englishHTML + detailsHTML,
	},
}

// noteTypeCSS styles the cards in light and night mode. Nouns are colored by gender.
const noteTypeCSS = `.card {
	font-family: "Noto Sans", "Helvetica Neue", Arial, sans-serif;
	font-size: 20px;
	line-height: 1.4;
	text-align: center;
	color: #1f2328;
	background-color: #ffffff;
	padding: 12px;
}

.prompt {
	font-size: 28px;
}

.target {
	font-size: 36px;
	font-weight: 600;
}

.target .article {
	font-weight: 400;
}

.target.masculine { color: #0969da; }
.target.feminine { color: #cf222e; }
.target.neuter { color: #1a7f37; }

.pos {
	font-size: 14px;
	font-style: italic;
	color: #6e7781;
	margin-top: 4px;
}

hr#answer {
	border: none;
	border-top: 1px solid #d0d7de;
	width: 80%;
	margin: 16px auto;
}

.grammar,
.parts {
	font-size: 15px;
	color: #57606a;
	margin-top: 8px;
}

.grammar:empty,
.parts:empty {
	display: none;
}

.grammar span + span::before,
.parts span + span::before {
	content: " · ";
}

.parts i {
	font-style: normal;
	font-size: 13px;
	color: #8c959f;
}

.examples,
.notes {
	text-align: left;
	max-width: 34em;
	margin: 16px auto 0;
}

.ex {
	margin: 8px 0;
}

.ex-el,
.ex-en {
	display: block;
}

.ex-en {
	font-size: 16px;
	color: #6e7781;
}

.notes {
	font-size: 15px;
	font-style: italic;
	color: #57606a;
	border-left: 3px solid #d0d7de;
	padding-left: 10px;
}

.nightMode.card,
.night_mode.card {
	color: #e6edf3;
	background-color: #1e1e1e;
}

.nightMode .target.masculine, .night_mode .target.masculine { color: #58a6ff; }
.nightMode .target.feminine, .night_mode .target.feminine { color: #ff7b72; }
.nightMode .target.neuter, .night_mode .target.neuter { color: #3fb950; }

.nightMode .pos, .night_mode .pos,
.nightMode .ex-en, .night_mode .ex-en {
	color: #8b949e;
}

.nightMode .grammar, .night_mode .grammar,
.nightMode .parts, .night_mode .parts,
.nightMode .notes, .night_mode .notes {
	color: #9da7b3;
}

.nightMode hr#answer, .night_mode hr#answer,
.nightMode .notes, .night_mode .notes {
	border-color: #3d444d;
}
`

// EnsureNoteType creates the VocabSync note type, or brings an existing one up to date by adding
// missing fields and replacing its card templates and styling. Review history is not affected.
func (c *AnkiClient) EnsureNoteType(modelName string) error {
	var modelList *[]string
	err := retryWithBackoff("ListNoteTypes", 3, func() error {
		var restErr *errors.RestErr
		modelList, restErr = c.client.Models.GetAll()
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to list note types: %v", err)
	}

	if modelList != nil {
		for _, m := range *modelList {
			if m == modelName {
				return c.updateNoteType(modelName)
			}
		}
	}

	model := ankiconnect.Model{
		ModelName:     modelName,
		InOrderFields: noteTypeFields,
		Css:           noteTypeCSS,
		CardTemplates: noteTypeTemplates,
	}

	if err := retryWithBackoff("CreateNoteType", 3, func() error {
		if restErr := c.client.Models.Create(model); restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create note type %s after retries: %v", modelName, err)
	}

	return nil
}

// updateNoteType adds any missing fields to an existing note type and replaces its templates and styling.
func (c *AnkiClient) updateNoteType(modelName string) error {
	var existingFields *[]string
	err := retryWithBackoff("GetNoteTypeFields", 3, func() error {
		var restErr *errors.RestErr
		existingFields, restErr = c.client.Models.GetFields(modelName)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to get fields of note type %s: %v", modelName, err)
	}

	have := make(map[string]bool)
	if existingFields != nil {
		for _, f := range *existingFields {
			have[f] = true
		}
	}
	fieldCount := len(have)
	for _, field := range noteTypeFields {
		if have[field] {
			continue
		}
		params := map[string]any{"modelName": modelName, "fieldName": field, "index": fieldCount}
		if err := c.invoke("modelFieldAdd", params); err != nil {
			return fmt.Errorf("failed to add field %s to note type %s: %w", field, modelName, err)
		}
		fieldCount++
	}

	templates := make(map[string]map[string]string, len(noteTypeTemplates))
	for _, t := range noteTypeTemplates {
		templates[t.Name] = map[string]string{"Front": t.Front, "Back": t.Back}
	}
	templateParams := map[string]any{"model": map[string]any{"name": modelName, "templates": templates}}
	if err := c.invoke("updateModelTemplates", templateParams); err != nil {
		return fmt.Errorf("failed to update templates of note type %s: %w", modelName, err)
	}

	stylingParams := map[string]any{"model": map[string]any{"name": modelName, "css": noteTypeCSS}}
	if err := c.invoke("updateModelStyling", stylingParams); err != nil {
		return fmt.Errorf("failed to update styling of note type %s: %w", modelName, err)
	}

	return nil
}

// invoke calls an AnkiConnect action that the ankiconnect library does not expose, with retries.
func (c *AnkiClient) invoke(action string, params any) error {
	body, err := json.Marshal(map[string]any{"action": action, "version": 6, "params": params})
	if err != nil {
		return fmt.Errorf("failed to encode %s request: %w", action, err)
	}

	return retryWithBackoff(action, 3, func() error {
		resp, err := http.Post(c.url, "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var result struct {
			Error *string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode %s response: %w", action, err)
		}
		if result.Error != nil {
			return fmt.Errorf("%s", *result.Error)
		}
		return nil
	})
}
