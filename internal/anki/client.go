package anki

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/atselvan/ankiconnect"
	"github.com/gataky/sync/internal/mapper"
	"github.com/gataky/sync/pkg/models"
	"github.com/privatesquare/bkst-go-utils/utils/errors"
)

const (
	// VocabSyncModelName is the name of the custom note type for vocabulary sync
	VocabSyncModelName = "VocabSync"
)

// soundTagPattern matches the [sound:file] tags Anki stores alongside a field's text.
var soundTagPattern = regexp.MustCompile(`\[sound:[^\]]*\]`)

// AnkiClient provides methods to interact with AnkiConnect API.
type AnkiClient struct {
	client *ankiconnect.Client
	url    string
}

// NewAnkiClient creates a new AnkiConnect client and validates connectivity.
// Returns an error if AnkiConnect cannot be reached.
func NewAnkiClient(url string) (*AnkiClient, error) {
	client := ankiconnect.NewClient()
	client.SetURL(url)

	ac := &AnkiClient{
		client: client,
		url:    url,
	}

	// Verify connectivity
	if err := ac.CheckConnection(); err != nil {
		return nil, err
	}

	return ac, nil
}

// CheckConnection verifies that AnkiConnect is reachable.
// Returns a helpful error message if Anki is not running.
func (c *AnkiClient) CheckConnection() error {
	// Use the Ping endpoint to check connectivity
	if err := c.client.Ping(); err != nil {
		return fmt.Errorf("cannot connect to AnkiConnect at %s. Is Anki running? Install AnkiConnect: https://ankiweb.net/shared/info/2055492159", c.url)
	}

	return nil
}

// CreateDeck creates a new deck in Anki.
// This operation is idempotent - it won't fail if the deck already exists.
func (c *AnkiClient) CreateDeck(deckName string) error {
	err := retryWithBackoff("CreateDeck", 3, func() error {
		if restErr := c.client.Decks.Create(deckName); restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create deck %s after retries: %v", deckName, err)
	}

	return nil
}

// AddNote creates a single note in Anki and returns the generated note ID.
// If a note with the same English+Greek combination already exists, updates it and returns that ID.
// This allows linking sheet rows to existing Anki cards.
// If audioData is provided, uploads and attaches audio. If audioFilename is provided (even with nil audioData),
// adds [sound:filename] tag to reference existing audio file.
func (c *AnkiClient) AddNote(deckName, modelName string, card *models.VocabCard, audioData []byte, audioFilename string) (int64, error) {
	// Check for existing note with same English + Greek
	// If found, update it and return the existing ID to allow linking
	existingID, err := c.findNoteByEnglishGreek(deckName, card.English, card.Greek)
	if err == nil && existingID > 0 {
		// Card already exists - update its fields and return the ID
		if err := c.UpdateNoteFields(existingID, card, audioData, audioFilename); err != nil {
			return 0, fmt.Errorf("found existing card but failed to update it: %w", err)
		}
		return existingID, nil
	}

	// Build Back field
	// Only add sound tag manually if we're linking to existing audio (no audioData to upload)
	// When uploading new audio, AnkiConnect will automatically add the sound tag via Audio.Fields
	backField := card.Greek
	if audioFilename != "" && len(audioData) == 0 {
		// Linking to existing audio file - add sound tag manually
		backField = fmt.Sprintf("%s [sound:%s]", card.Greek, audioFilename)
	}

	// Build fields from card
	fields := buildNoteFields(card, backField)

	// Build tags
	tags := buildTags(card.Tag, card.SubTag1, card.SubTag2)

	// Create note with options to allow duplicate Front fields
	// This allows multiple entries with same English but different Greek
	note := ankiconnect.Note{
		DeckName:  deckName,
		ModelName: modelName,
		Fields:    fields,
		Tags:      tags,
		Options: &ankiconnect.Options{
			AllowDuplicate: true,
		},
	}

	// Add audio if provided
	if len(audioData) > 0 {
		filename := fmt.Sprintf("%s.mp3", card.Greek)

		// Convert to base64 for ankiconnect
		encodedData := base64.StdEncoding.EncodeToString(audioData)

		note.Audio = []ankiconnect.Audio{
			{
				Data:     encodedData,
				Filename: filename,
				Fields:   []string{"Back"}, // AnkiConnect will append [sound:filename] to Back field
			},
		}

		// Note: AnkiConnect automatically appends [sound:filename.mp3] to the Back field
		// The final Back field will be: "γεια[sound:γεια.mp3]"
	}

	if err := retryWithBackoff("AddNote", 3, func() error {
		if restErr := c.client.Notes.Add(note); restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to add note after retries: %v", err)
	}

	// Retrieve the note ID by searching for English + Greek
	// This ensures we get the correct card even with English duplicates
	noteID, err := c.findNoteByEnglishGreek(deckName, card.English, card.Greek)
	if err != nil {
		return 0, fmt.Errorf("note was created but could not retrieve its ID: %w", err)
	}

	return noteID, nil
}

// findNoteByEnglishGreek searches for a note by English + Greek combination.
// Returns the note ID if found, or 0 if not found.
func (c *AnkiClient) findNoteByEnglishGreek(deckName, english, greek string) (int64, error) {
	// Search by both Front (English) and Back (Greek) fields.
	// The wildcard lets the search find "greek [sound:...]", but it also matches longer words that
	// start with the same letters ("κρύο*" finds "κρύος"), so each result is compared exactly below.
	query := fmt.Sprintf(`deck:"%s" "Front:%s" "Back:%s*"`, deckName, english, greek)

	var notes *[]ankiconnect.ResultNotesInfo
	err := retryWithBackoff("SearchNote", 3, func() error {
		var restErr *errors.RestErr
		notes, restErr = c.client.Notes.Get(query)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("search failed after retries: %v", err)
	}

	if notes != nil {
		for _, note := range *notes {
			if backMatchesGreek(note.Fields["Back"].Value, greek) {
				return note.NoteId, nil
			}
		}
	}

	return 0, fmt.Errorf("note not found")
}

// backMatchesGreek reports whether a note's Back field holds exactly the given Greek text, ignoring
// any [sound:...] tags. Case is ignored to match how Anki's own search compares text.
func backMatchesGreek(back, greek string) bool {
	text := soundTagPattern.ReplaceAllString(back, "")
	return strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(greek))
}

// UpdateNoteFields updates the fields of an existing note without touching review history.
// If audioData is provided, uploads new audio. If only audioFilename is provided, links to existing audio.
func (c *AnkiClient) UpdateNoteFields(noteID int64, card *models.VocabCard, audioData []byte, audioFilename string) error {
	// Build Back field with audio if provided
	backField := card.Greek
	if audioFilename != "" {
		if len(audioData) > 0 {
			// New audio being uploaded - store it first, AnkiConnect will add the sound tag
			if err := c.StoreAudioFile(audioFilename, audioData); err != nil {
				return fmt.Errorf("failed to store audio file: %w", err)
			}
			// Add sound tag manually since we're updating fields directly
			backField = fmt.Sprintf("%s [sound:%s]", card.Greek, audioFilename)
		} else {
			// Linking to existing audio
			backField = fmt.Sprintf("%s [sound:%s]", card.Greek, audioFilename)
		}
	}

	// Build updated fields
	fields := buildNoteFields(card, backField)

	// Update note
	updateNote := ankiconnect.UpdateNote{
		Id:     noteID,
		Fields: fields,
	}

	if err := retryWithBackoff("UpdateNote", 3, func() error {
		if restErr := c.client.Notes.Update(updateNote); restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update note %d after retries: %v", noteID, err)
	}

	return nil
}

// DeleteNote deletes a note from Anki by its ID.
// Note: The ankiconnect library doesn't have a DeleteNotes method, so this is a placeholder.
func (c *AnkiClient) DeleteNote(noteID int64) error {
	// The ankiconnect library doesn't expose deleteNotes, so we'll need to work around this
	// For now, return an error indicating this functionality needs implementation
	return fmt.Errorf("delete note functionality not yet implemented in ankiconnect library")
}

// CheckAudioExists checks if an audio file exists in Anki's media collection.
// Note: This method is not currently used as we check the filesystem directly for better performance.
func (c *AnkiClient) CheckAudioExists(filename string) (bool, error) {
	var fileNames *[]string
	err := retryWithBackoff("CheckAudioExists", 3, func() error {
		var restErr *errors.RestErr
		fileNames, restErr = c.client.Media.GetMediaFileNames(filename)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to check audio existence: %v", err)
	}

	if fileNames == nil || len(*fileNames) == 0 {
		return false, nil
	}

	// Check if exact filename match exists
	for _, name := range *fileNames {
		if name == filename {
			return true, nil
		}
	}

	return false, nil
}

// StoreAudioFile stores an audio file in Anki's media collection.
func (c *AnkiClient) StoreAudioFile(filename string, audioData []byte) error {
	// Convert to base64
	encodedData := base64.StdEncoding.EncodeToString(audioData)

	// Store via AnkiConnect with retry logic
	err := retryWithBackoff("StoreAudioFile", 3, func() error {
		_, restErr := c.client.Media.StoreMediaFile(filename, encodedData)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to store audio file '%s' after retries: %v", filename, err)
	}

	return nil
}

// exampleSeparator separates a Greek example sentence from its English translation.
const exampleSeparator = " — "

// buildNoteFields builds the Anki note fields for a card. backField is the Greek text with any sound tag.
func buildNoteFields(card *models.VocabCard, backField string) ankiconnect.Fields {
	return ankiconnect.Fields{
		"Front":        card.English,
		"Back":         backField,
		"PartOfSpeech": card.PartOfSpeech,
		"Article":      mapper.NounArticle(card),
		"Gender":       mapper.NounGender(card),
		"Attributes":   card.Attributes,
		"Voice":        card.Voice,
		"ActiveForm":   card.ActiveForm,
		"FutureSimple": card.FutureSimple,
		"Aorist":       card.Aorist,
		"Examples":     formatExamplesHTML(card.Examples),
		"Notes":        formatNotesHTML(card.Notes),
	}
}

// formatExamplesHTML converts examples text to HTML format.
// Each non-empty line becomes one example; "Greek — English" lines are split so the translation
// can be styled separately.
func formatExamplesHTML(examples string) string {
	var out strings.Builder
	for _, line := range nonEmptyLines(examples) {
		out.WriteString(`<div class="ex">`)
		if greek, english, found := strings.Cut(line, exampleSeparator); found {
			fmt.Fprintf(&out, `<span class="ex-el">%s</span><span class="ex-en">%s</span>`,
				strings.TrimSpace(greek), strings.TrimSpace(english))
		} else {
			fmt.Fprintf(&out, `<span class="ex-el">%s</span>`, line)
		}
		out.WriteString(`</div>`)
	}
	return out.String()
}

// formatNotesHTML converts notes text to HTML, keeping each non-empty line on its own line.
func formatNotesHTML(notes string) string {
	return strings.Join(nonEmptyLines(notes), "<br>")
}

// nonEmptyLines splits text into trimmed lines, dropping empty ones.
func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// buildTags constructs hierarchical tags from tag fields.
func buildTags(tag, subTag1, subTag2 string) []string {
	tags := []string{}

	if tag == "" {
		return tags
	}

	// Build hierarchical tag
	tagParts := []string{tag}

	if subTag1 != "" {
		tagParts = append(tagParts, subTag1)
	}

	if subTag2 != "" && subTag1 != "" {
		tagParts = append(tagParts, subTag2)
	}

	// Join with ::
	var fullTag string
	for i, part := range tagParts {
		if i == 0 {
			fullTag = part
		} else {
			fullTag = fullTag + "::" + part
		}
	}

	tags = append(tags, fullTag)
	return tags
}
