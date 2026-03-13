package cli

import (
	"github.com/gataky/sync/internal/sync"
	"github.com/spf13/cobra"
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
	dryRun := getDryRun()

	ctx, err := Bootstrap(BootstrapOptions{LoadState: true, EnableTTS: false})
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Create puller
	puller := sync.NewPuller(ctx.SheetsClient, ctx.AnkiClient, ctx.Config, ctx.State, ctx.StateManager, ctx.Logger)

	// Execute pull
	if err := puller.Pull(dryRun); err != nil {
		return printError("pull failed: %w", err)
	}

	return nil
}
