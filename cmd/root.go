package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/nospor/kairos/db"
	"github.com/spf13/cobra"
)

var store *db.Store
var configFlag string
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "kairos",
	Short:   "A CLI time tracker",
	Version: Version,
	Long: `Kairos is a command-line time tracking application.

It helps you track time spent on tasks organised by projects,
generate reports, and export data to CSV.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		store, err = db.New(configFlag)
		if err != nil {
			return fmt.Errorf("failed to initialise database: %w", err)
		}

		if cmd.Name() != "daemon" {
			_, _ = store.AutoStopStaleTasks(2 * time.Minute)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if store != nil {
			store.Close()
		}
	},
}

// Execute is the main entry point for the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "", "path to a custom database file (default: ~/.cache/kairos/kairos.db)")
}
