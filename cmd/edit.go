package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var editProjectFlag string

var editCmd = &cobra.Command{
	Use:   "edit [old task name] [new task name]",
	Short: "Rename an existing task",
	Long: `Rename an existing task. Use -p to specify the project it belongs to.
Without -p, the task is looked up in the default "General" project.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, newName := args[0], args[1]
		projectName := editProjectFlag
		if projectName == "" {
			projectName = "General"
		}

		if err := store.RenameTask(oldName, newName, projectName); err != nil {
			return err
		}
		fmt.Printf("Task %q in project %q renamed to %q.\n", oldName, projectName, newName)
		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&editProjectFlag, "project", "p", "", `project the task belongs to (default: "General")`)
	rootCmd.AddCommand(editCmd)
}
