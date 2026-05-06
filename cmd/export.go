package cmd

import (
	"encoding/csv"
	"fmt"
	"os"

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

		w := csv.NewWriter(f)
		defer w.Flush()

		// Header
		if err := w.Write([]string{"Project Name", "Task Name", "Duration"}); err != nil {
			return fmt.Errorf("could not write CSV header: %w", err)
		}

		for _, r := range rows {
			record := []string{r.ProjectName, r.TaskName, formatDuration(r.Duration)}
			if err := w.Write(record); err != nil {
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
