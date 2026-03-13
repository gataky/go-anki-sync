package cli

import (
	"context"

	"github.com/gataky/sync/internal/anki"
	"github.com/gataky/sync/internal/config"
	"github.com/gataky/sync/internal/logging"
	"github.com/gataky/sync/internal/sheets"
	"github.com/gataky/sync/internal/state"
	"github.com/gataky/sync/internal/tts"
	"github.com/gataky/sync/pkg/models"
)

// AppContext holds initialized dependencies for CLI commands.
type AppContext struct {
	Config       *models.Config
	SheetsClient *sheets.SheetsClient
	AnkiClient   *anki.AnkiClient
	TTSClient    *tts.TTSClient
	State        *models.SyncState
	StateManager *state.Manager
	Logger       *logging.SyncLogger
}

// BootstrapOptions configures which dependencies to initialize.
type BootstrapOptions struct {
	LoadState bool // Load sync state (for pull/both)
	EnableTTS bool // Initialize TTS client (for push)
}

// Bootstrap initializes all dependencies for a CLI command.
func Bootstrap(opts BootstrapOptions) (*AppContext, error) {
	logger := newSyncLogger()

	// Load configuration
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		return nil, printError("failed to get config path: %w", err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, printError("failed to load configuration: %w", err)
	}

	// Get credentials
	credentialsPath, err := config.GetDefaultCredentialsPath()
	if err != nil {
		return nil, printError("failed to get credentials path: %w", err)
	}

	// Initialize Sheets client
	sheetsClient, err := sheets.NewSheetsClient(credentialsPath, "")
	if err != nil {
		return nil, printError("failed to initialize Sheets client: %w", err)
	}

	// Initialize Anki client
	ankiClient, err := anki.NewAnkiClient(cfg.AnkiConnectURL)
	if err != nil {
		return nil, printError("failed to initialize Anki client: %w", err)
	}

	ctx := &AppContext{
		Config:       cfg,
		SheetsClient: sheetsClient,
		AnkiClient:   ankiClient,
		Logger:       logger,
	}

	// Conditionally load state
	if opts.LoadState {
		statePath, err := state.GetDefaultStatePath()
		if err != nil {
			return nil, printError("failed to get state path: %w", err)
		}
		syncState, err := state.LoadState(statePath)
		if err != nil {
			return nil, printError("failed to load state: %w", err)
		}
		ctx.State = syncState
		ctx.StateManager = &state.Manager{}
	}

	// Conditionally initialize TTS
	if opts.EnableTTS && cfg.TextToSpeech != nil && cfg.TextToSpeech.Enabled {
		ttsCtx := context.Background()
		ttsClient, err := tts.NewTTSClient(ttsCtx, credentialsPath, cfg.TextToSpeech)
		if err != nil {
			return nil, printError("failed to initialize TTS client: %w", err)
		}
		ctx.TTSClient = ttsClient
		logger.Info("TTS client initialized successfully")
	} else if opts.EnableTTS {
		logger.Info("TTS is disabled, skipping audio generation")
	}

	return ctx, nil
}

// Close cleans up resources.
func (ctx *AppContext) Close() {
	if ctx.TTSClient != nil {
		ctx.TTSClient.Close()
	}
}
