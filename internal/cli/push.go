package cli

import (
	"github.com/spf13/cobra"
	"github.com/yourusername/sync/internal/anki"
	"github.com/yourusername/sync/internal/config"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/internal/sync"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push changes from Google Sheets to Anki",
	Long: `Push vocabulary cards from Google Sheets to Anki.
Creates new cards in Anki and updates existing cards that have changed.
Uses checksum-based change detection to avoid unnecessary updates.`,
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	logger := getLogger()
	dryRun := getDryRun()

	// Load configuration
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		return printError("failed to get config path: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return printError("failed to load configuration: %w", err)
	}

	// Get credentials path
	credentialsPath, err := config.GetDefaultCredentialsPath()
	if err != nil {
		return printError("failed to get credentials path: %w", err)
	}

	// Get token path
	tokenPath, err := config.GetDefaultTokenPath()
	if err != nil {
		return printError("failed to get token path: %w", err)
	}

	// Initialize Google Sheets client
	sheetsClient, err := sheets.NewSheetsClient(credentialsPath, tokenPath)
	if err != nil {
		return printError("failed to initialize Sheets client: %w", err)
	}

	// Initialize Anki client
	ankiClient, err := anki.NewAnkiClient(cfg.AnkiConnectURL)
	if err != nil {
		return printError("failed to initialize Anki client: %w", err)
	}

	// Create pusher
	pusher := sync.NewPusher(sheetsClient, ankiClient, cfg, logger)

	// Execute push
	if err := pusher.Push(dryRun); err != nil {
		return printError("push failed: %w", err)
	}

	return nil
}
