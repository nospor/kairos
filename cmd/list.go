package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks and their durations",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := store.ListTasks()
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "Project\tTask\tDuration")
		fmt.Fprintln(w, strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 10))
		for _, t := range tasks {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.ProjectName, t.TaskName, formatDuration(t.TotalDuration))
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
