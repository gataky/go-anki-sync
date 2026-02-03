package anki

import (
	"fmt"
	"strings"
	"time"

	"github.com/atselvan/ankiconnect"
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
	if err := c.client.Decks.Create(deckName); err != nil {
		return fmt.Errorf("failed to create deck %s: %v", deckName, err)
	}

	return nil
}

// CreateNoteType creates the VocabSync note type with fields and card templates.
// This operation checks if the model already exists before creating.
func (c *AnkiClient) CreateNoteType(modelName string) error {
	// Check if model already exists
	modelList, err := c.client.Models.GetAll()
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
	fields := []string{"Front", "Back", "Grammar", "Examples"}

	// Define card templates
	cardTemplates := []ankiconnect.CardTemplate{
		{
			Name:  "English to Greek",
			Front: "{{Front}}",
			Back:  "{{Back}}<br><br><small>{{Grammar}}</small><br><br>{{Examples}}",
		},
		{
			Name:  "Greek to English",
			Front: "{{Back}}",
			Back:  "{{Front}}<br><br><small>{{Grammar}}</small><br><br>{{Examples}}",
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

	if err := c.client.Models.Create(model); err != nil {
		return fmt.Errorf("failed to create note type %s: %v", modelName, err)
	}

	return nil
}

// AddNote creates a single note in Anki and returns the generated note ID.
func (c *AnkiClient) AddNote(deckName, modelName string, card *models.VocabCard) (int64, error) {
	// Build fields from card
	fields := ankiconnect.Fields{
		"Front":    card.English,
		"Back":     card.Greek,
		"Grammar":  buildGrammarField(card.PartOfSpeech, card.Attributes),
		"Examples": formatExamplesHTML(card.Examples),
	}

	// Build tags
	tags := buildTags(card.Tag, card.SubTag1, card.SubTag2)

	// Create note
	note := ankiconnect.Note{
		DeckName:  deckName,
		ModelName: modelName,
		Fields:    fields,
		Tags:      tags,
	}

	if err := c.client.Notes.Add(note); err != nil {
		return 0, fmt.Errorf("failed to add note: %v", err)
	}

	// To get the note ID, we need to search for it
	// Search for the note we just created by Front field
	query := fmt.Sprintf(`deck:"%s" "Front:%s"`, deckName, card.English)
	noteIDs, err := c.client.Notes.Search(query)
	if err != nil || noteIDs == nil || len(*noteIDs) == 0 {
		return 0, fmt.Errorf("note was created but could not retrieve its ID")
	}

	// Return the first matching note ID (should be the one we just created)
	return (*noteIDs)[0], nil
}

// UpdateNoteFields updates the fields of an existing note without touching review history.
func (c *AnkiClient) UpdateNoteFields(noteID int64, card *models.VocabCard) error {
	// Build updated fields
	fields := ankiconnect.Fields{
		"Front":    card.English,
		"Back":     card.Greek,
		"Grammar":  buildGrammarField(card.PartOfSpeech, card.Attributes),
		"Examples": formatExamplesHTML(card.Examples),
	}

	// Update note
	updateNote := ankiconnect.UpdateNote{
		Id:     noteID,
		Fields: fields,
	}

	if err := c.client.Notes.Update(updateNote); err != nil {
		return fmt.Errorf("failed to update note %d: %v", noteID, err)
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

	noteIDs, err := c.client.Notes.Search(query)
	if err != nil {
		return nil, fmt.Errorf("failed to find modified notes: %v", err)
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

	notesInfo, err := c.client.Notes.Get(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get notes info: %v", err)
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
func formatExamplesHTML(examples string) string {
	if examples == "" {
		return ""
	}

	// For now, return as-is. The mapper package will handle the actual formatting.
	// This is a simple pass-through that will be enhanced by the mapper.
	return examples
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
