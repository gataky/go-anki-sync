package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	debug   bool
	dryRun  bool
	logger  *log.Logger
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync vocabulary cards between Google Sheets and Anki",
	Long: `A bidirectional sync tool for managing vocabulary flashcards.
Syncs vocabulary data between Google Sheets and Anki with checksum-based
change detection and timestamp-based conflict resolution.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set up logging based on flags
		logFlags := 0
		if debug {
			logFlags = log.Ldate | log.Ltime | log.Lshortfile
		} else if verbose {
			logFlags = log.Ldate | log.Ltime
		}

		logger = log.New(os.Stdout, "", logFlags)

		if debug {
			logger.Println("Debug mode enabled")
		} else if verbose {
			logger.Println("Verbose mode enabled")
		}
	},
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

// getLogger returns the global logger instance
func getLogger() *log.Logger {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	return logger
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
