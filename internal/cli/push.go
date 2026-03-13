package cli

import (
	"github.com/gataky/sync/internal/sync"
	"github.com/spf13/cobra"
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
	dryRun := getDryRun()

	ctx, err := Bootstrap(BootstrapOptions{LoadState: false, EnableTTS: true})
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Create pusher
	pusher := sync.NewPusher(ctx.SheetsClient, ctx.AnkiClient, ctx.Config, ctx.Logger, ctx.TTSClient)

	// Execute push
	if err := pusher.Push(dryRun); err != nil {
		return printError("push failed: %w", err)
	}

	return nil
}
