package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listProjectsCmd = &cobra.Command{
	Use:   "list-projects",
	Short: "List all projects and their tasks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := store.ListProjects()
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		for i, p := range projects {
			fmt.Fprintf(w, "Project: %s\n", p.ProjectName)
			if len(p.Tasks) == 0 {
				fmt.Fprintln(w, "  (no tasks)")
			} else {
				for _, t := range p.Tasks {
					fmt.Fprintf(w, "  - %s\t%s\n", t.TaskName, formatDuration(t.TotalDuration))
				}
			}
			if i < len(projects)-1 {
				fmt.Fprintln(w)
			}
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listProjectsCmd)
}
