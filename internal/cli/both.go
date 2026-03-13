package cli

import (
	"github.com/gataky/sync/internal/sync"
	"github.com/spf13/cobra"
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
	dryRun := getDryRun()

	ctx, err := Bootstrap(BootstrapOptions{LoadState: true, EnableTTS: false})
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Create both syncer
	bothSyncer := sync.NewBothSyncer(ctx.SheetsClient, ctx.AnkiClient, ctx.Config, ctx.State, ctx.StateManager, ctx.Logger)

	// Execute bidirectional sync
	if err := bothSyncer.Sync(dryRun); err != nil {
		return printError("bidirectional sync failed: %w", err)
	}

	return nil
}
