package cmd

import (
	"github.com/spf13/cobra"
)

var daemonNotifyFlag int

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the background heartbeat daemon (internal use only)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return store.RunDaemon(daemonNotifyFlag)
	},
}

func init() {
	daemonCmd.Flags().IntVar(&daemonNotifyFlag, "notify", 0, "send notification every N minutes")
	rootCmd.AddCommand(daemonCmd)
}
