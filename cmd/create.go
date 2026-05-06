package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var createProjectFlag string

var createCmd = &cobra.Command{
	Use:   "create [task name]",
	Short: "Create a new task",
	Long: `Create a new task. Use -p to assign it to a project.
Without -p, the task is assigned to the default "General" project.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]
		projectName := createProjectFlag
		if projectName == "" {
			projectName = "General"
		}

		if err := store.CreateTask(taskName, projectName); err != nil {
			return err
		}
		fmt.Printf("Task %q created under project %q.\n", taskName, projectName)
		return nil
	},
}

func init() {
	createCmd.Flags().StringVarP(&createProjectFlag, "project", "p", "", `project to assign the task to (default: "General")`)
	rootCmd.AddCommand(createCmd)
}
