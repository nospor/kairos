package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [task name]",
	Short: "Delete a task and its tracked time",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.DeleteTask(name); err != nil {
			return err
		}
		fmt.Printf("Task %q deleted.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
