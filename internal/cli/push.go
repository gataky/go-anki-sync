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
Uses checksum-based change detection to avoid unnecessary updates.
Use --force to update every existing card, e.g. after a card layout change.`,
	RunE: runPush,
}

var forcePush bool

func init() {
	pushCmd.Flags().BoolVarP(&forcePush, "force", "f", false, "Update all existing cards, even if unchanged")
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	dryRun := getDryRun()

	ctx, err := Bootstrap(BootstrapOptions{EnableTTS: true})
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Create pusher
	pusher := sync.NewPusher(ctx.SheetsClient, ctx.AnkiClient, ctx.Config, ctx.Logger, ctx.TTSClient)
	pusher.SetForceUpdate(forcePush)

	// Execute push
	if err := pusher.Push(dryRun); err != nil {
		return printError("push failed: %w", err)
	}

	return nil
}
