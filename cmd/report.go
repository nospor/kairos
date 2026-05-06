package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nospor/kairos/db"
	"github.com/nospor/kairos/model"
	"github.com/spf13/cobra"
)

var (
	reportProject string
	reportToday   bool
	reportWeek    bool
	reportMonth   bool
	reportFrom    string
	reportTo      string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show a report of tracked tasks and their durations",
	Long: `Show a report of all tracked tasks and their durations.

Use flags to filter by project or time range:
  --project "Project A"    Filter by project
  --today                  Show only today's entries
  --week                   Show only this week's entries
  --month                  Show only this month's entries
  --from "2026-01-01"      Show entries from this date
  --to "2026-01-31"        Show entries up to this date`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filter, err := buildFilter(reportProject, reportToday, reportWeek, reportMonth, reportFrom, reportTo)
		if err != nil {
			return err
		}

		rows, err := store.GetReport(filter)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			fmt.Println("No time entries found.")
			return nil
		}

		printReport(rows)
		return nil
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportProject, "project", "", "filter by project name")
	reportCmd.Flags().BoolVar(&reportToday, "today", false, "show only today's entries")
	reportCmd.Flags().BoolVar(&reportWeek, "week", false, "show only this week's entries")
	reportCmd.Flags().BoolVar(&reportMonth, "month", false, "show only this month's entries")
	reportCmd.Flags().StringVar(&reportFrom, "from", "", "show entries from this date (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportTo, "to", "", "show entries up to this date (YYYY-MM-DD)")
	rootCmd.AddCommand(reportCmd)
}

// buildFilter constructs a ReportFilter from the CLI flags.
func buildFilter(project string, today, week, month bool, from, to string) (db.ReportFilter, error) {
	filter := db.ReportFilter{
		ProjectName: project,
	}

	now := time.Now()

	if today {
		bod := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		eod := bod.Add(24 * time.Hour)
		filter.From = &bod
		filter.To = &eod
	} else if week {
		// Go back to Monday of the current week
		weekday := now.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -int(weekday-time.Monday))
		bow := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
		eow := bow.AddDate(0, 0, 7)
		filter.From = &bow
		filter.To = &eow
	} else if month {
		bom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		eom := bom.AddDate(0, 1, 0)
		filter.From = &bom
		filter.To = &eom
	} else {
		if from != "" {
			t, err := time.Parse("2006-01-02", from)
			if err != nil {
				return filter, fmt.Errorf("invalid --from date %q (expected YYYY-MM-DD): %w", from, err)
			}
			filter.From = &t
		}
		if to != "" {
			t, err := time.Parse("2006-01-02", to)
			if err != nil {
				return filter, fmt.Errorf("invalid --to date %q (expected YYYY-MM-DD): %w", to, err)
			}
			// Include the entire "to" day
			eod := t.Add(24 * time.Hour)
			filter.To = &eod
		}
	}

	return filter, nil
}

// printReport renders a formatted report table to stdout.
func printReport(rows []model.ReportRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Project Name\tTask Name\tDuration")
	fmt.Fprintln(w, strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 10))

	var currentProject string
	var projectTotal time.Duration
	var grandTotal time.Duration

	for i, r := range rows {
		if currentProject != "" && r.ProjectName != currentProject {
			// Print subtotal for the previous project
			fmt.Fprintf(w, "\t\t--------\n")
			fmt.Fprintf(w, "%s Total\t\t%s\n\n", currentProject, formatDuration(projectTotal))
			projectTotal = 0
		}
		currentProject = r.ProjectName
		projectTotal += r.Duration
		grandTotal += r.Duration

		fmt.Fprintf(w, "%s\t%s\t%s\n", r.ProjectName, r.TaskName, formatDuration(r.Duration))

		// Last row — print final subtotal
		if i == len(rows)-1 {
			fmt.Fprintf(w, "\t\t--------\n")
			fmt.Fprintf(w, "%s Total\t\t%s\n", currentProject, formatDuration(projectTotal))
		}
	}

	// Grand total if more than one project
	if grandTotal != projectTotal || currentProject == "" {
		fmt.Fprintf(w, "\nGrand Total\t\t%s\n", formatDuration(grandTotal))
	}

	w.Flush()
}
