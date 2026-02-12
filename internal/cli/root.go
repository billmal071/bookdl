package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "bookdl",
	Short: "Download books from Anna's Archive",
	Long: `bookdl is a CLI tool for searching and downloading books from Anna's Archive.

It supports resumable downloads, interactive selection, and multiple access methods.

Examples:
  bookdl search "clean code"              Search for books
  bookdl search -f epub "design patterns" Search for EPUB books
  bookdl download abc123def456...         Download by MD5 hash
  bookdl list                             List all downloads
  bookdl resume 1                         Resume download #1
  bookdl pause 1                          Pause download #1`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize config
		if err := config.Init(cfgFile); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		// Initialize database
		if err := db.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		db.Close()
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.config/bookdl/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// Verbose returns whether verbose mode is enabled
func Verbose() bool {
	return verbose
}

// Printf prints if verbose mode is enabled
func Printf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

// Errorf prints an error message to stderr
func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// Successf prints a success message
func Successf(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}
