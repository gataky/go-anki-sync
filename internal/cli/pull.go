package cli

import (
	"github.com/spf13/cobra"
	"github.com/gataky/sync/internal/anki"
	"github.com/gataky/sync/internal/config"
	"github.com/gataky/sync/internal/sheets"
	"github.com/gataky/sync/internal/state"
	"github.com/gataky/sync/internal/sync"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from Anki to Google Sheets",
	Long: `Pull vocabulary cards from Anki to Google Sheets.
Updates Sheet rows that correspond to cards modified in Anki.
Uses modification timestamps to detect changes.`,
	RunE: runPull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	logger := newSyncLogger()
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

	// Create puller
	puller := sync.NewPuller(sheetsClient, ankiClient, cfg, syncState, stateManager, logger)

	// Execute pull
	if err := puller.Pull(dryRun); err != nil {
		return printError("pull failed: %w", err)
	}

	return nil
}
