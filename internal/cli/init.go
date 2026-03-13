package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gataky/sync/internal/config"
	"github.com/gataky/sync/pkg/models"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sync configuration",
	Long: `Initialize the sync tool by creating a configuration file.
Prompts for Google Sheet ID, sheet name, and Anki deck name.
Saves configuration to ~/.sync/config.yaml`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	logger := newSyncLogger()
	logger.Info("Initializing sync configuration...")

	// Check if config already exists
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		return printError("failed to get config path: %w", err)
	}
	if _, err := os.Stat(configPath); err == nil {
		// Config exists, ask to overwrite
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Configuration already exists at %s\n", configPath)
		fmt.Print("Overwrite? (y/N): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			logger.Info("Initialization cancelled")
			return nil
		}
	}

	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return printError("failed to create config directory: %w", err)
	}

	// Check for service account key
	credentialsPath, err := config.GetDefaultCredentialsPath()
	if err != nil {
		return printError("failed to get credentials path: %w", err)
	}
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Println("\n⚠️  Warning: service account key file not found")
		fmt.Printf("Expected location: %s\n", credentialsPath)
		fmt.Println("\nTo create a service account (much simpler than OAuth2!):")
		fmt.Println("1. Go to https://console.cloud.google.com/")
		fmt.Println("2. Create a new project or select an existing one")
		fmt.Println("3. Enable the Google Sheets API")
		fmt.Println("4. Go to IAM & Admin → Service Accounts")
		fmt.Println("5. Create Service Account (any name works)")
		fmt.Println("6. Click on the service account → Keys → Add Key → Create New Key → JSON")
		fmt.Println("7. Download the JSON key file")
		fmt.Printf("8. Move it to %s\n", credentialsPath)
		fmt.Println("9. **IMPORTANT**: Share your Google Sheet with the service account email")
		fmt.Println("   (the email looks like: your-service@project.iam.gserviceaccount.com)")
		fmt.Println()
	}

	reader := bufio.NewReader(os.Stdin)

	// Prompt for Google Sheet ID
	fmt.Print("Google Sheet ID: ")
	sheetID, err := reader.ReadString('\n')
	if err != nil {
		return printError("failed to read Sheet ID: %w", err)
	}
	sheetID = strings.TrimSpace(sheetID)
	if sheetID == "" {
		return printError("Sheet ID cannot be empty")
	}

	// Validate Sheet ID format (should be alphanumeric and dashes)
	if !isValidSheetID(sheetID) {
		return printError("invalid Sheet ID format. Example: 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms")
	}

	// Prompt for Sheet name
	fmt.Print("Sheet name within spreadsheet (default: Sheet1): ")
	sheetName, err := reader.ReadString('\n')
	if err != nil {
		return printError("failed to read sheet name: %w", err)
	}
	sheetName = strings.TrimSpace(sheetName)
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	// Prompt for Anki deck name
	fmt.Print("Anki deck name: ")
	deckName, err := reader.ReadString('\n')
	if err != nil {
		return printError("failed to read deck name: %w", err)
	}
	deckName = strings.TrimSpace(deckName)
	if deckName == "" {
		return printError("Anki deck name cannot be empty")
	}

	// Create config
	cfg := &models.Config{
		GoogleSheetID: sheetID,
		SheetName:     sheetName,
		AnkiDeck:      deckName,
	}
	cfg.SetDefaults()

	// Validate config
	if err := cfg.Validate(); err != nil {
		return printError("invalid configuration: %w", err)
	}

	// Save config
	if err := config.SaveConfig(cfg, configPath); err != nil {
		return printError("failed to save configuration: %w", err)
	}

	logger.Info("Configuration saved to %s", configPath)
	logger.Info("\nNext steps:")
	logger.Info("1. Ensure Anki is running with AnkiConnect installed")
	logger.Info("2. Run 'sync push' to push cards from Sheet to Anki")
	logger.Info("3. Run 'sync pull' to pull changes from Anki to Sheet")
	logger.Info("4. Run 'sync both' for bidirectional sync")

	return nil
}

// isValidSheetID checks if the Sheet ID looks valid
func isValidSheetID(id string) bool {
	if len(id) < 10 {
		return false
	}
	// Sheet IDs are alphanumeric with some special chars
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
