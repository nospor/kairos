package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop tracking the current task",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName, duration, err := store.StopActive()
		if err != nil {
			return err
		}
		fmt.Printf("Stopped tracking time for task %q. Duration: %s.\n", taskName, formatDuration(duration))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
