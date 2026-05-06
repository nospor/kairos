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
	reportGroupBy string
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
  --to "2026-01-31"        Show entries up to this date
  --group-by "day"         Group data by day, week, month, or year`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filter, err := buildFilter(reportProject, reportToday, reportWeek, reportMonth, reportFrom, reportTo, reportGroupBy)
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
	reportCmd.Flags().StringVar(&reportGroupBy, "group-by", "", "group given data by days, weeks, months, or years")
	rootCmd.AddCommand(reportCmd)
}

// buildFilter constructs a ReportFilter from the CLI flags.
func buildFilter(project string, today, week, month bool, from, to, groupBy string) (db.ReportFilter, error) {
	filter := db.ReportFilter{
		ProjectName: project,
	}

	switch groupBy {
	case "day", "days":
		filter.GroupBy = "day"
	case "week", "weeks":
		filter.GroupBy = "week"
	case "month", "months":
		filter.GroupBy = "month"
	case "year", "years":
		filter.GroupBy = "year"
	case "":
		// no grouping
	default:
		return filter, fmt.Errorf("invalid --group-by value: %s (expected day, week, month, or year)", groupBy)
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
	hasDate := false
	for _, r := range rows {
		if r.Date != "" {
			hasDate = true
			break
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if hasDate {
		fmt.Fprintln(w, "Project Name\tTask Name\tDate\tDuration")
		fmt.Fprintln(w, strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 10)+"\t"+strings.Repeat("-", 10))
	} else {
		fmt.Fprintln(w, "Project Name\tTask Name\tDuration")
		fmt.Fprintln(w, strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 10))
	}

	var currentProject string
	var projectTotal time.Duration
	var grandTotal time.Duration

	for i, r := range rows {
		if currentProject != "" && r.ProjectName != currentProject {
			// Print subtotal for the previous project
			if hasDate {
				fmt.Fprintf(w, "\t\t\t--------\n")
				fmt.Fprintf(w, "%s Total\t\t\t%s\n", currentProject, formatDuration(projectTotal))
				fmt.Fprintf(w, "\t\t\t\n") // spacer
			} else {
				fmt.Fprintf(w, "\t\t--------\n")
				fmt.Fprintf(w, "%s Total\t\t%s\n", currentProject, formatDuration(projectTotal))
				fmt.Fprintf(w, "\t\t\n") // spacer
			}
			projectTotal = 0
		}
		currentProject = r.ProjectName
		projectTotal += r.Duration
		grandTotal += r.Duration

		if hasDate {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ProjectName, r.TaskName, r.Date, formatDuration(r.Duration))
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.ProjectName, r.TaskName, formatDuration(r.Duration))
		}

		// Last row — print final subtotal
		if i == len(rows)-1 {
			if hasDate {
				fmt.Fprintf(w, "\t\t\t--------\n")
				fmt.Fprintf(w, "%s Total\t\t\t%s\n", currentProject, formatDuration(projectTotal))
			} else {
				fmt.Fprintf(w, "\t\t--------\n")
				fmt.Fprintf(w, "%s Total\t\t%s\n", currentProject, formatDuration(projectTotal))
			}
		}
	}

	// Grand total if more than one project
	if grandTotal != projectTotal || currentProject == "" {
		if hasDate {
			fmt.Fprintf(w, "\t\t\t\n") // spacer
			fmt.Fprintf(w, "Grand Total\t\t\t%s\n", formatDuration(grandTotal))
		} else {
			fmt.Fprintf(w, "\t\t\n") // spacer
			fmt.Fprintf(w, "Grand Total\t\t%s\n", formatDuration(grandTotal))
		}
	}

	w.Flush()
}
