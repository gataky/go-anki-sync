package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/yourusername/sync/internal/anki"
	"github.com/yourusername/sync/internal/config"
	"github.com/yourusername/sync/internal/sheets"
	"github.com/yourusername/sync/internal/sync"
	"github.com/yourusername/sync/internal/tts"
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

	// Initialize TTS client if enabled
	var ttsClient *tts.TTSClient
	if cfg.TextToSpeech != nil && cfg.TextToSpeech.Enabled {
		ctx := context.Background()
		ttsClient, err = tts.NewTTSClient(ctx, credentialsPath, cfg.TextToSpeech)
		if err != nil {
			return printError("failed to initialize TTS client: %w", err)
		}
		defer ttsClient.Close()
		logger.Println("TTS client initialized successfully")
	} else {
		logger.Println("TTS is disabled, skipping audio generation")
	}

	// Create pusher
	pusher := sync.NewPusher(sheetsClient, ankiClient, cfg, logger, ttsClient)

	// Execute push
	if err := pusher.Push(dryRun); err != nil {
		return printError("push failed: %w", err)
	}

	return nil
}
