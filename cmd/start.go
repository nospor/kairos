package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var startProjectFlag string

var startCmd = &cobra.Command{
	Use:   "start [task name]",
	Short: "Start tracking time for a task",
	Long: `Start tracking time for a task. Use -p to specify the project it belongs to.
Without -p, the task is looked up in the default "General" project.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := args[0]
		if err := store.StartTask(taskName, startProjectFlag); err != nil {
			return err
		}
		projectName := startProjectFlag
		if projectName == "" {
			projectName = "General"
		}
		fmt.Printf("Started tracking time for task %q (project: %q).\n", taskName, projectName)
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&startProjectFlag, "project", "p", "", `project the task belongs to (default: "General")`)
	rootCmd.AddCommand(startCmd)
}
