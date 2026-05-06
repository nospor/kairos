package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	exportProject string
	exportToday   bool
	exportWeek    bool
	exportMonth   bool
	exportFrom    string
	exportTo      string
)

var exportCmd = &cobra.Command{
	Use:   "export [file name]",
	Short: "Export the report to a CSV file",
	Long: `Export the report to a specified file in CSV format.

Supports the same filtering flags as the report command:
  --project, --today, --week, --month, --from, --to`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		filter, err := buildFilter(exportProject, exportToday, exportWeek, exportMonth, exportFrom, exportTo)
		if err != nil {
			return err
		}

		rows, err := store.GetReport(filter)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			fmt.Println("No time entries found to export.")
			return nil
		}

		f, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("could not create file %q: %w", filename, err)
		}
		defer f.Close()

		// Header
		if _, err := fmt.Fprintf(f, "\"%s\",\"%s\",\"%s\"\n", "Project Name", "Task Name", "Duration"); err != nil {
			return fmt.Errorf("could not write CSV header: %w", err)
		}

		var currentProject string
		var projectTotal time.Duration
		var grandTotal time.Duration

		for i, r := range rows {
			if currentProject != "" && r.ProjectName != currentProject {
				// Print subtotal for the previous project
				projTotalName := fmt.Sprintf("%s Total", currentProject)
				if _, err := fmt.Fprintf(f, "\"%s\",\"%s\",\"%s\"\n", projTotalName, "", formatDuration(projectTotal)); err != nil {
					return fmt.Errorf("could not write CSV row: %w", err)
				}
				projectTotal = 0
			}
			currentProject = r.ProjectName
			projectTotal += r.Duration
			grandTotal += r.Duration

			proj := strings.ReplaceAll(r.ProjectName, "\"", "\"\"")
			task := strings.ReplaceAll(r.TaskName, "\"", "\"\"")
			dur := formatDuration(r.Duration)
			if _, err := fmt.Fprintf(f, "\"%s\",\"%s\",\"%s\"\n", proj, task, dur); err != nil {
				return fmt.Errorf("could not write CSV row: %w", err)
			}

			// Last row — print final subtotal
			if i == len(rows)-1 {
				projTotalName := fmt.Sprintf("%s Total", currentProject)
				if _, err := fmt.Fprintf(f, "\"%s\",\"%s\",\"%s\"\n", projTotalName, "", formatDuration(projectTotal)); err != nil {
					return fmt.Errorf("could not write CSV row: %w", err)
				}
			}
		}

		// Grand total if more than one project
		if grandTotal != projectTotal || currentProject == "" {
			if _, err := fmt.Fprintf(f, "\"%s\",\"%s\",\"%s\"\n", "Grand Total", "", formatDuration(grandTotal)); err != nil {
				return fmt.Errorf("could not write CSV row: %w", err)
			}
		}

		fmt.Printf("Report exported to %s.\n", filename)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportProject, "project", "", "filter by project name")
	exportCmd.Flags().BoolVar(&exportToday, "today", false, "export only today's entries")
	exportCmd.Flags().BoolVar(&exportWeek, "week", false, "export only this week's entries")
	exportCmd.Flags().BoolVar(&exportMonth, "month", false, "export only this month's entries")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "export entries from this date (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "export entries up to this date (YYYY-MM-DD)")
	rootCmd.AddCommand(exportCmd)
}
