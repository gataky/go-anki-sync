package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/sync/internal/logging"
)

var (
	verbose bool
	debug   bool
	dryRun  bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync vocabulary cards between Google Sheets and Anki",
	Long: `A bidirectional sync tool for managing vocabulary flashcards.
Syncs vocabulary data between Google Sheets and Anki with checksum-based
change detection and timestamp-based conflict resolution.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging (includes file and line info)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview changes without applying them")
}

// newSyncLogger creates a logger with the appropriate level based on flags
func newSyncLogger() *logging.SyncLogger {
	level := logging.Silent
	if debug {
		level = logging.Debug
	} else if verbose {
		level = logging.Verbose
	}
	return logging.NewSyncLogger(level, os.Stdout)
}

// getDryRun returns the global dry-run flag value
func getDryRun() bool {
	return dryRun
}

// printError prints an error message and returns the error
func printError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return err
}
