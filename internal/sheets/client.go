package sheets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// CellUpdate represents a single cell update for batch operations.
type CellUpdate struct {
	Row    int    // Row number (1-indexed, excluding header)
	Column string // Column letter (A, B, C, etc.)
	Value  interface{}
}

// SheetsClient provides methods to interact with Google Sheets API.
type SheetsClient struct {
	service *sheets.Service
	ctx     context.Context
}

// NewSheetsClient creates a new Google Sheets client using service account authentication.
// This is simpler than OAuth2 - just need a service account JSON key file.
// No browser authorization required!
//
// The tokenPath parameter is ignored (kept for API compatibility).
func NewSheetsClient(credentialsPath, tokenPath string) (*SheetsClient, error) {
	ctx := context.Background()

	// Read service account credentials file
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account key file %s: %w", credentialsPath, err)
	}

	// Create credentials from service account JSON
	creds, err := google.CredentialsFromJSON(ctx, credentials, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account credentials: %w (make sure this is a service account key file)", err)
	}

	// Create Sheets service
	service, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets service: %w", err)
	}

	return &SheetsClient{
		service: service,
		ctx:     ctx,
	}, nil
}

// Note: OAuth2 functions removed - now using simpler service account authentication

// ReadSheet fetches all rows from a Google Sheet.
// Returns rows as a 2D slice of interface{} values.
func (c *SheetsClient) ReadSheet(sheetID, sheetName string) ([][]interface{}, error) {
	readRange := fmt.Sprintf("%s!A1:Z", sheetName) // Read columns A through Z

	resp, err := c.service.Spreadsheets.Values.Get(sheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(resp.Values) == 0 {
		return nil, fmt.Errorf("sheet is empty")
	}

	return resp.Values, nil
}

// ParseHeaders parses the header row and returns a map of column names to indices.
// Column matching is case-insensitive.
func (c *SheetsClient) ParseHeaders(rows [][]interface{}) (map[string]int, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows to parse")
	}

	headerRow := rows[0]
	headers := make(map[string]int)

	for i, cell := range headerRow {
		if cell == nil {
			continue
		}

		// Convert to string and normalize (lowercase, trim spaces)
		headerName := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", cell)))
		if headerName != "" {
			headers[headerName] = i
		}
	}

	return headers, nil
}

// ValidateRequiredColumns checks that all required columns exist in the sheet.
// Returns an error listing any missing columns.
func (c *SheetsClient) ValidateRequiredColumns(headers map[string]int, required []string) error {
	var missing []string

	for _, col := range required {
		normalized := strings.ToLower(col)
		if _, exists := headers[normalized]; !exists {
			missing = append(missing, col)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("required columns missing: %s", strings.Join(missing, ", "))
	}

	return nil
}

// BatchUpdateCells updates multiple cells in a single API call using BatchUpdate.
// This is more efficient than individual updates for bulk operations.
func (c *SheetsClient) BatchUpdateCells(sheetID, sheetName string, updates []CellUpdate) error {
	if len(updates) == 0 {
		return nil // Nothing to update
	}

	// Build ValueRange objects for each update
	var data []*sheets.ValueRange
	for _, update := range updates {
		// Convert row number to A1 notation (e.g., row 2, column A -> A3)
		// Note: row is 1-indexed excluding header, so we add 1 for header + 1 for actual row = +2
		cellRange := fmt.Sprintf("%s!%s%d", sheetName, update.Column, update.Row+1)

		data = append(data, &sheets.ValueRange{
			Range:  cellRange,
			Values: [][]interface{}{{update.Value}},
		})
	}

	// Execute batch update
	batchUpdateRequest := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "USER_ENTERED",
		Data:             data,
	}

	_, err := c.service.Spreadsheets.Values.BatchUpdate(sheetID, batchUpdateRequest).Do()
	if err != nil {
		return fmt.Errorf("failed to batch update cells: %w", err)
	}

	return nil
}

// CreateChecksumColumnIfMissing adds a "Checksum" column to the sheet if it doesn't exist.
// The column is inserted after the "Anki ID" column (column B).
func (c *SheetsClient) CreateChecksumColumnIfMissing(sheetID, sheetName string, headers map[string]int) error {
	// Check if checksum column already exists
	if _, exists := headers["checksum"]; exists {
		return nil // Column already exists
	}

	// Insert "Checksum" header in column B (after "Anki ID" in column A)
	cellRange := fmt.Sprintf("%s!B1", sheetName)
	valueRange := &sheets.ValueRange{
		Range:  cellRange,
		Values: [][]interface{}{{"Checksum"}},
	}

	_, err := c.service.Spreadsheets.Values.Update(sheetID, cellRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Do()

	if err != nil {
		return fmt.Errorf("failed to create Checksum column: %w", err)
	}

	return nil
}
