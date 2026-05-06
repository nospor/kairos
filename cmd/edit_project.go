package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var editProjectCmd = &cobra.Command{
	Use:   "edit-project [old name] [new name]",
	Short: "Rename an existing project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, newName := args[0], args[1]
		if err := store.RenameProject(oldName, newName); err != nil {
			return err
		}
		fmt.Printf("Project %q renamed to %q.\n", oldName, newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(editProjectCmd)
}
