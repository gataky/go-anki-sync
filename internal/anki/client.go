package anki

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/atselvan/ankiconnect"
	"github.com/privatesquare/bkst-go-utils/utils/errors"
	"github.com/yourusername/sync/pkg/models"
)

const (
	// VocabSyncModelName is the name of the custom note type for vocabulary sync
	VocabSyncModelName = "VocabSync"
)

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

// CreateNoteType creates the VocabSync note type with fields and card templates.
// This operation checks if the model already exists before creating.
func (c *AnkiClient) CreateNoteType(modelName string) error {
	// Check if model already exists (with retry logic)
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

	// If model exists, nothing to do
	if modelList != nil {
		for _, m := range *modelList {
			if m == modelName {
				return nil
			}
		}
	}

	// Define fields
	fields := []string{"Front", "Back", "PartOfSpeech", "Grammar", "Examples"}

	// Define card templates
	cardTemplates := []ankiconnect.CardTemplate{
		{
			Name:  "English to Greek",
			Front: "{{Front}} <small>({{PartOfSpeech}})</small>",
			Back:  "{{Front}} <small>({{PartOfSpeech}})</small><hr>{{Back}}<br><br><small>{{Grammar}}</small><br><br>{{Examples}}",
		},
		{
			Name:  "Greek to English",
			Front: "{{Back}} <small>({{PartOfSpeech}})</small>",
			Back:  "{{Back}} <small>({{PartOfSpeech}})</small><hr>{{Front}}<br><br><small>{{Grammar}}</small><br><br>{{Examples}}",
		},
	}

	// Define CSS
	css := `.card {
	font-family: arial;
	font-size: 20px;
	text-align: center;
	color: black;
	background-color: white;
}

small {
	font-size: 14px;
	color: #666;
}
`

	// Create the model
	model := ankiconnect.Model{
		ModelName:     modelName,
		InOrderFields: fields,
		Css:           css,
		CardTemplates: cardTemplates,
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
	fields := ankiconnect.Fields{
		"Front":        card.English,
		"Back":         backField,
		"PartOfSpeech": card.PartOfSpeech,
		"Grammar":      buildGrammarField(card.PartOfSpeech, card.Attributes),
		"Examples":     formatExamplesHTML(card.Examples),
	}

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
	// Search by both Front (English) and Back (Greek) fields
	// Use wildcard for Back field to match both "greek" and "greek [sound:...]"
	// This ensures uniqueness is based on the combination
	query := fmt.Sprintf(`deck:"%s" "Front:%s" "Back:%s*"`, deckName, english, greek)

	var noteIDs *[]int64
	err := retryWithBackoff("SearchNote", 3, func() error {
		var restErr *errors.RestErr
		noteIDs, restErr = c.client.Notes.Search(query)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("search failed after retries: %v", err)
	}

	if noteIDs == nil || len(*noteIDs) == 0 {
		return 0, fmt.Errorf("note not found")
	}

	// Return the first matching note ID
	return (*noteIDs)[0], nil
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
	fields := ankiconnect.Fields{
		"Front":        card.English,
		"Back":         backField,
		"PartOfSpeech": card.PartOfSpeech,
		"Grammar":      buildGrammarField(card.PartOfSpeech, card.Attributes),
		"Examples":     formatExamplesHTML(card.Examples),
	}

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

// FindModifiedNotes finds notes that have been modified since the given timestamp.
// Returns a list of note IDs.
func (c *AnkiClient) FindModifiedNotes(deckName string, sinceTimestamp time.Time) ([]int64, error) {
	// Calculate days since timestamp
	daysSince := int(time.Since(sinceTimestamp).Hours() / 24)
	if daysSince < 1 {
		daysSince = 1
	}

	// Build query: deck:"DeckName" edited:N
	query := fmt.Sprintf(`deck:"%s" edited:%d`, deckName, daysSince)

	var noteIDs *[]int64
	err := retryWithBackoff("FindModifiedNotes", 3, func() error {
		var restErr *errors.RestErr
		noteIDs, restErr = c.client.Notes.Search(query)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find modified notes after retries: %v", err)
	}

	if noteIDs == nil {
		return []int64{}, nil
	}

	return *noteIDs, nil
}

// GetNotesInfo retrieves full information for the given note IDs.
// Returns a slice of VocabCard structs populated with Anki data.
func (c *AnkiClient) GetNotesInfo(noteIDs []int64) ([]*models.VocabCard, error) {
	if len(noteIDs) == 0 {
		return []*models.VocabCard{}, nil
	}

	// Convert noteIDs to strings for the query
	queryParts := make([]string, len(noteIDs))
	for i, id := range noteIDs {
		queryParts[i] = fmt.Sprintf("nid:%d", id)
	}
	query := strings.Join(queryParts, " OR ")

	var notesInfo *[]ankiconnect.ResultNotesInfo
	err := retryWithBackoff("GetNotesInfo", 3, func() error {
		var restErr *errors.RestErr
		notesInfo, restErr = c.client.Notes.Get(query)
		if restErr != nil {
			return fmt.Errorf("%s", restErr.Error)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get notes info after retries: %v", err)
	}

	if notesInfo == nil {
		return []*models.VocabCard{}, nil
	}

	cards := make([]*models.VocabCard, 0, len(*notesInfo))
	for _, noteInfo := range *notesInfo {
		card := &models.VocabCard{
			AnkiID:     noteInfo.NoteId,
			English:    noteInfo.Fields["Front"].Value,
			Greek:      noteInfo.Fields["Back"].Value,
			ModifiedAt: time.Now(), // Note: ankiconnect library doesn't expose modification time
		}

		// Parse Grammar field back to PartOfSpeech and Attributes
		parseGrammarField(noteInfo.Fields["Grammar"].Value, card)

		// Store examples as-is (HTML format)
		card.Examples = noteInfo.Fields["Examples"].Value

		// Parse tags back to hierarchy
		parseTags(noteInfo.Tags, card)

		cards = append(cards, card)
	}

	return cards, nil
}

// buildGrammarField combines part of speech and attributes into a single field.
func buildGrammarField(partOfSpeech, attributes string) string {
	if attributes == "" {
		return partOfSpeech
	}
	return fmt.Sprintf("%s (%s)", partOfSpeech, attributes)
}

// formatExamplesHTML converts examples text to HTML format.
// Takes newline-separated examples and returns a numbered HTML list.
func formatExamplesHTML(examples string) string {
	if examples == "" {
		return ""
	}

	// Split by newlines and filter empty lines
	lines := strings.Split(examples, "\n")
	var validLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			validLines = append(validLines, trimmed)
		}
	}

	// If no valid lines, return empty
	if len(validLines) == 0 {
		return ""
	}

	// If only one line, return it without list formatting
	if len(validLines) == 1 {
		return validLines[0]
	}

	// Multiple lines: create numbered list
	var html strings.Builder
	html.WriteString("<ol style='text-align: left; margin: 10px auto; display: inline-block;'>")
	for _, line := range validLines {
		html.WriteString("<li>")
		html.WriteString(line)
		html.WriteString("</li>")
	}
	html.WriteString("</ol>")

	return html.String()
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

// parseGrammarField splits grammar field back into part of speech and attributes.
func parseGrammarField(grammar string, card *models.VocabCard) {
	// Simple parsing: look for format "PartOfSpeech (Attributes)"
	if grammar == "" {
		return
	}

	// Check if there are parentheses
	openParen := -1
	closeParen := -1
	for i, c := range grammar {
		if c == '(' {
			openParen = i
		} else if c == ')' {
			closeParen = i
		}
	}

	if openParen > 0 && closeParen > openParen {
		// Extract part of speech and attributes
		card.PartOfSpeech = grammar[:openParen]
		card.Attributes = grammar[openParen+1 : closeParen]
		// Trim spaces
		card.PartOfSpeech = card.PartOfSpeech[:len(card.PartOfSpeech)-1] // Remove trailing space before (
		return
	}

	// No parentheses, entire string is part of speech
	card.PartOfSpeech = grammar
	card.Attributes = ""
}

// parseTags splits hierarchical tags back into individual tag fields.
func parseTags(tags []string, card *models.VocabCard) {
	if len(tags) == 0 {
		return
	}

	// Take the first tag and split by ::
	firstTag := tags[0]
	parts := strings.Split(firstTag, "::")

	// Assign to card fields
	if len(parts) > 0 {
		card.Tag = parts[0]
	}
	if len(parts) > 1 {
		card.SubTag1 = parts[1]
	}
	if len(parts) > 2 {
		card.SubTag2 = parts[2]
	}
}
