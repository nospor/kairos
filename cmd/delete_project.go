package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteProjectCmd = &cobra.Command{
	Use:   "delete-project [project name]",
	Short: "Delete a project and all its tasks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.DeleteProject(name); err != nil {
			return err
		}
		fmt.Printf("Project %q and all its tasks deleted.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteProjectCmd)
}
