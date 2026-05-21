package cmd

import (
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the background heartbeat daemon (internal use only)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return store.RunDaemon()
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
