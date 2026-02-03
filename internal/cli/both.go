package cli

import (
	"github.com/spf13/cobra"
	"github.com/yourusername/sync/internal/anki"
	"github.com/yourusername/sync/internal/config"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/internal/state"
	"github.com/yourusername/sync/internal/sync"
)

var bothCmd = &cobra.Command{
	Use:   "both",
	Short: "Bidirectional sync between Google Sheets and Anki",
	Long: `Perform bidirectional synchronization between Google Sheets and Anki.
Creates new cards, updates changed cards in both directions, and resolves
conflicts using timestamp-based last-write-wins strategy.`,
	RunE: runBoth,
}

func init() {
	rootCmd.AddCommand(bothCmd)
}

func runBoth(cmd *cobra.Command, args []string) error {
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

	// Load state
	statePath, err := state.GetDefaultStatePath()
	if err != nil {
		return printError("failed to get state path: %w", err)
	}
	syncState, err := state.LoadState(statePath)
	if err != nil {
		return printError("failed to load state: %w", err)
	}

	// Get service account credentials path
	credentialsPath, err := config.GetDefaultCredentialsPath()
	if err != nil {
		return printError("failed to get credentials path: %w", err)
	}

	// Initialize Google Sheets client (tokenPath not needed for service accounts)
	sheetsClient, err := sheets.NewSheetsClient(credentialsPath, "")
	if err != nil {
		return printError("failed to initialize Sheets client: %w", err)
	}

	// Initialize Anki client
	ankiClient, err := anki.NewAnkiClient(cfg.AnkiConnectURL)
	if err != nil {
		return printError("failed to initialize Anki client: %w", err)
	}

	// Create state manager
	stateManager := &state.Manager{}

	// Create both syncer
	bothSyncer := sync.NewBothSyncer(sheetsClient, ankiClient, cfg, syncState, stateManager, logger)

	// Execute bidirectional sync
	if err := bothSyncer.Sync(dryRun); err != nil {
		return printError("bidirectional sync failed: %w", err)
	}

	return nil
}
