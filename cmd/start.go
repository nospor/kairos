package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [task name]",
	Short: "Start tracking time for a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]
		if err := store.StartTask(taskName); err != nil {
			return err
		}
		fmt.Printf("Started tracking time for task %q.\n", taskName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
